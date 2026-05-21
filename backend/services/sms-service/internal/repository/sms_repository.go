package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrOTPNotFound = errors.New("otp not found")
	ErrSMSNotFound = errors.New("sms not found")
)

//go:generate mockgen -source=sms_repository.go -destination=../../mocks/repomock/mock_sms_repository.go -package=repomock
type SMSRepositoryInterface interface {
	// OTP methods
	StoreOTP(ctx context.Context, phone, purpose, code string, ttl time.Duration) error
	GetOTP(ctx context.Context, phone, purpose string) (string, error)
	DeleteOTP(ctx context.Context, phone, purpose string) error
	IncrementRateLimit(ctx context.Context, phone string, ttl time.Duration) (int64, error)
	GetRateLimit(ctx context.Context, phone string) (int64, error)

	// Retry Queue methods
	StoreRetryMessage(ctx context.Context, messageID, phone, message string, attempts int, ttl time.Duration) error
	GetRetryMessage(ctx context.Context, messageID string) (phone string, message string, attempts int, err error)
	IncrementRetryAttempt(ctx context.Context, messageID string) (int64, error)
	DeleteRetryMessage(ctx context.Context, messageID string) error
}

type SMSRepository struct {
	redis *redis.Client
}

func NewSMSRepository(rdb *redis.Client) *SMSRepository {
	return &SMSRepository{
		redis: rdb,
	}
}

// StoreOTP stores the OTP in Redis.
func (r *SMSRepository) StoreOTP(ctx context.Context, phone, purpose, code string, ttl time.Duration) error {
	key := "sms:otp:" + phone + ":" + purpose
	return r.redis.Set(ctx, key, code, ttl).Err()
}

// GetOTP retrieves the OTP from Redis.
func (r *SMSRepository) GetOTP(ctx context.Context, phone, purpose string) (string, error) {
	key := "sms:otp:" + phone + ":" + purpose
	code, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrOTPNotFound
	}
	return code, err
}

// DeleteOTP removes the OTP from Redis.
func (r *SMSRepository) DeleteOTP(ctx context.Context, phone, purpose string) error {
	key := "sms:otp:" + phone + ":" + purpose
	return r.redis.Del(ctx, key).Err()
}

// IncrementRateLimit increments the usage counter for a phone number.
func (r *SMSRepository) IncrementRateLimit(ctx context.Context, phone string, ttl time.Duration) (int64, error) {
	key := "sms:rate:" + phone
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// If it's the first time, set the TTL
	if count == 1 {
		r.redis.Expire(ctx, key, ttl)
	}
	return count, nil
}

// GetRateLimit returns the current rate limit count.
func (r *SMSRepository) GetRateLimit(ctx context.Context, phone string) (int64, error) {
	key := "sms:rate:" + phone
	count, err := r.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// StoreRetryMessage stores message data for retries in a Redis HASH.
// Key pattern: sms:retry:{message_id} with fields: phone, message, attempts
// TTL = 24h as per ERD specification.
func (r *SMSRepository) StoreRetryMessage(ctx context.Context, messageID, phone, message string, attempts int, ttl time.Duration) error {
	key := "sms:retry:" + messageID
	err := r.redis.HSet(ctx, key, map[string]interface{}{
		"phone":    phone,
		"message":  message,
		"attempts": fmt.Sprintf("%d", attempts),
	}).Err()
	if err != nil {
		return err
	}
	return r.redis.Expire(ctx, key, ttl).Err()
}

// GetRetryMessage retrieves the retry message details from Redis HASH.
func (r *SMSRepository) GetRetryMessage(ctx context.Context, messageID string) (phone string, message string, attempts int, err error) {
	key := "sms:retry:" + messageID
	data, err := r.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return "", "", 0, err
	}
	if len(data) == 0 {
		return "", "", 0, ErrSMSNotFound
	}

	phone = data["phone"]
	message = data["message"]

	attempts, _ = strconv.Atoi(data["attempts"])

	return phone, message, attempts, nil
}

// IncrementRetryAttempt increments the 'attempts' field in the HASH.
func (r *SMSRepository) IncrementRetryAttempt(ctx context.Context, messageID string) (int64, error) {
	key := "sms:retry:" + messageID
	return r.redis.HIncrBy(ctx, key, "attempts", 1).Result()
}

// DeleteRetryMessage deletes the retry message HASH.
func (r *SMSRepository) DeleteRetryMessage(ctx context.Context, messageID string) error {
	key := "sms:retry:" + messageID
	return r.redis.Del(ctx, key).Err()
}
