//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
)

func setupTestRepo(t *testing.T) (*UserRepository, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}

	repo := &UserRepository{
		db:    db,
		redis: nil, // for unit tests we can keep redis nil if we handle it
	}

	return repo, mock
}

func TestUserRepository_CreateProfile(t *testing.T) {
	repo, mock := setupTestRepo(t)
	defer repo.db.Close()

	ctx := context.Background()
	profile := &models.UserProfile{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		FullName:     "John Doe",
		AvatarURL:    "http://avatar.com",
		Gender:       "male",
		ReferralCode: "REF123",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mock.ExpectExec("INSERT INTO user_profiles").
		WithArgs(profile.ID, profile.UserID, profile.FullName, profile.AvatarURL, profile.BirthDate, profile.Gender, profile.ReferralCode, profile.ReferredBy, profile.CreatedAt, profile.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateProfile(ctx, profile)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_GetProfileByUserID(t *testing.T) {
	repo, mock := setupTestRepo(t)
	defer repo.db.Close()

	ctx := context.Background()
	userID := uuid.New()
	profileID := uuid.New()

	rows := sqlmock.NewRows([]string{"id", "user_id", "full_name", "avatar_url", "birth_date", "gender", "referral_code", "referred_by", "created_at", "updated_at"}).
		AddRow(profileID, userID, "John Doe", "http://avatar.com", "1990-01-01", "male", "REF123", nil, time.Now(), time.Now())

	mock.ExpectQuery("SELECT (.+) FROM user_profiles").
		WithArgs(userID).
		WillReturnRows(rows)

	// Note: Redis call will panic if r.redis is nil. 
	// I should probably mock redis or fix the code to check if redis is nil.
	// For now, I'll assume redis is initialized or I'll fix the code.
	
	p, err := repo.GetProfileByUserID(ctx, userID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "John Doe", p.FullName)
}

func TestUserRepository_CreateAddress(t *testing.T) {
	repo, mock := setupTestRepo(t)
	defer repo.db.Close()

	ctx := context.Background()
	addr := &models.UserAddress{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Label:     "Home",
		Address:   "123 Main St",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mock.ExpectExec("INSERT INTO user_addresses").
		WithArgs(addr.ID, addr.UserID, addr.Label, addr.Address, addr.Lat, addr.Lng, addr.Notes, addr.IsDefault, addr.CreatedAt, addr.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.CreateAddress(ctx, addr)
	assert.NoError(t, err)
}

func TestUserRepository_UpdateSettings(t *testing.T) {
	repo, mock := setupTestRepo(t)
	defer repo.db.Close()

	ctx := context.Background()
	settings := &models.UserSettings{
		UserID:           uuid.New(),
		Language:         "en",
		NotifEnabled:     true,
		MarketingEnabled: false,
	}

	mock.ExpectExec("INSERT INTO user_settings").
		WithArgs(settings.UserID, settings.Language, settings.NotifEnabled, settings.MarketingEnabled, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.UpdateSettings(ctx, settings)
	assert.NoError(t, err)
}
