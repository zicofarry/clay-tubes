package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrEmailNotFound    = errors.New("email log not found")
)

//go:generate mockgen -source=email_repository.go -destination=../../mocks/mock_email_repository.go -package=mocks
type EmailRepository interface {
	SaveEmailLog(ctx context.Context, email model.EmailStatusResponse) error
	GetEmailStatus(ctx context.Context, emailId string) (*model.EmailStatusResponse, error)
	UpsertTemplate(ctx context.Context, template model.EmailTemplate) error
	GetTemplates(ctx context.Context) ([]model.EmailTemplate, error)
	GetTemplate(ctx context.Context, id model.EmailTemplateId) (*model.EmailTemplate, error)
	UpdateEmailStatus(ctx context.Context, providerId, status string) error
	
	// New methods based on ERD Architecture (Redis)
	SaveRetryQueue(ctx context.Context, messageId string, data interface{}) error
	IncrementRateLimit(ctx context.Context, domain string) (int, error)
}

type redisEmailRepository struct {
	client *redis.Client
}

func NewEmailRepository(redisUrl string) EmailRepository {
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		panic(err)
	}
	return &redisEmailRepository{
		client: redis.NewClient(opt),
	}
}

func (r *redisEmailRepository) SaveEmailLog(ctx context.Context, email model.EmailStatusResponse) error {
	data, _ := json.Marshal(email)
	// Map provider_id to emailId for webhook updates
	if email.ProviderId != "" {
		r.client.Set(ctx, "email:provider:"+email.ProviderId, email.EmailId, 7*24*time.Hour)
	}
	return r.client.Set(ctx, "email:log:"+email.EmailId, data, 7*24*time.Hour).Err()
}

func (r *redisEmailRepository) GetEmailStatus(ctx context.Context, emailId string) (*model.EmailStatusResponse, error) {
	val, err := r.client.Get(ctx, "email:log:"+emailId).Result()
	if err == redis.Nil {
		return nil, ErrEmailNotFound
	} else if err != nil {
		return nil, err
	}
	var email model.EmailStatusResponse
	json.Unmarshal([]byte(val), &email)
	return &email, nil
}

func (r *redisEmailRepository) UpsertTemplate(ctx context.Context, template model.EmailTemplate) error {
	data, _ := json.Marshal(template)
	return r.client.HSet(ctx, "email:templates", string(template.TemplateId), data).Err()
}

func (r *redisEmailRepository) GetTemplates(ctx context.Context) ([]model.EmailTemplate, error) {
	vals, err := r.client.HGetAll(ctx, "email:templates").Result()
	if err != nil {
		return nil, err
	}
	var templates []model.EmailTemplate
	for _, val := range vals {
		var t model.EmailTemplate
		json.Unmarshal([]byte(val), &t)
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *redisEmailRepository) GetTemplate(ctx context.Context, id model.EmailTemplateId) (*model.EmailTemplate, error) {
	val, err := r.client.HGet(ctx, "email:templates", string(id)).Result()
	if err == redis.Nil {
		return nil, ErrTemplateNotFound
	} else if err != nil {
		return nil, err
	}
	var t model.EmailTemplate
	json.Unmarshal([]byte(val), &t)
	return &t, nil
}

func (r *redisEmailRepository) UpdateEmailStatus(ctx context.Context, providerId, status string) error {
	emailId, err := r.client.Get(ctx, "email:provider:"+providerId).Result()
	if err == redis.Nil || emailId == "" {
		return ErrEmailNotFound
	}
	
	email, err := r.GetEmailStatus(ctx, emailId)
	if err != nil {
		return err
	}
	
	email.Status = status
	if status == "delivered" {
		now := time.Now()
		email.DeliveredAt = &now
	}
	
	return r.SaveEmailLog(ctx, *email)
}

func (r *redisEmailRepository) SaveRetryQueue(ctx context.Context, messageId string, data interface{}) error {
	// ERD: email:retry:{message_id} HASH, TTL 24h
	bytes, _ := json.Marshal(data)
	var dict map[string]interface{}
	json.Unmarshal(bytes, &dict)
	
	err := r.client.HSet(ctx, "email:retry:"+messageId, dict).Err()
	if err != nil {
		return err
	}
	return r.client.Expire(ctx, "email:retry:"+messageId, 24*time.Hour).Err()
}

func (r *redisEmailRepository) IncrementRateLimit(ctx context.Context, domain string) (int, error) {
	// ERD: email:rate:{recipient_domain} STRING, TTL 1h
	key := "email:rate:" + domain
	count, err := r.client.Incr(ctx, key).Result()
	if count == 1 {
		r.client.Expire(ctx, key, time.Hour)
	}
	return int(count), err
}
