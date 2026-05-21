//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateCredential_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	cred := &Credential{
		Email:          "test@example.com",
		Phone:          "+6281234567890",
		PasswordHash: "hashed_password",
		Role:           "user",
	}

	mock.ExpectQuery(`^INSERT INTO credentials`).
		WithArgs(cred.Email, cred.Phone, cred.PasswordHash, cred.Role).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("uuid-123", time.Now(), time.Now()))

	created, err := repo.CreateCredential(context.Background(), cred)

	if err != nil {
		t.Errorf("error was not expected while inserting credential: %s", err)
	}

	if created.ID != "uuid-123" {
		t.Errorf("expected id 'uuid-123', got '%s'", created.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestFindByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM credentials WHERE id = \$1$`).
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "phone", "password_hash", "role", "status", "email_verified", "phone_verified", "created_at", "updated_at",
		}).AddRow("user-123", "test@example.com", "+62812", "hash", "user", "active", false, true, time.Now(), time.Now()))

	cred, err := repo.FindByID(context.Background(), "user-123")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cred == nil || cred.Email != "test@example.com" {
		t.Errorf("expected credential with email test@example.com")
	}
}

func TestFindByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM credentials WHERE id = \$1$`).
		WithArgs("unknown").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.FindByID(context.Background(), "unknown")

	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestFindByIdentifier_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM credentials WHERE email = \$1 OR phone = \$1$`).
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "phone", "password_hash", "role", "status", "email_verified", "phone_verified", "created_at", "updated_at",
		}).AddRow("user-123", "test@example.com", "+62812", "hash", "user", "active", false, true, time.Now(), time.Now()))

	cred, err := repo.FindByIdentifier(context.Background(), "test@example.com")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cred == nil || cred.ID != "user-123" {
		t.Errorf("expected credential with ID user-123")
	}
}

func TestFindByIdentifier_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM credentials WHERE email = \$1 OR phone = \$1$`).
		WithArgs("unknown").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.FindByIdentifier(context.Background(), "unknown")

	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestExistsByEmailOrPhone_Exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT EXISTS\(SELECT 1 FROM credentials WHERE email = \$1 OR phone = \$2\)$`).
		WithArgs("test@example.com", "+62812").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := repo.ExistsByEmailOrPhone(context.Background(), "test@example.com", "+62812")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists to be true")
	}
}

func TestExistsByEmailOrPhone_NotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectQuery(`^SELECT EXISTS\(SELECT 1 FROM credentials WHERE email = \$1 OR phone = \$2\)$`).
		WithArgs("test@example.com", "+62812").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	exists, err := repo.ExistsByEmailOrPhone(context.Background(), "test@example.com", "+62812")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists to be false")
	}
}

func TestUpdatePassword_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectExec(`^UPDATE credentials SET password_hash = \$1, updated_at = NOW\(\) WHERE id = \$2$`).
		WithArgs("new_hash", "user-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.UpdatePassword(context.Background(), "user-123", "new_hash")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetPhoneVerified_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewAuthRepository(db, nil)

	mock.ExpectExec(`^UPDATE credentials SET phone_verified = true, updated_at = NOW\(\) WHERE phone = \$1$`).
		WithArgs("+62812").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.SetPhoneVerified(context.Background(), "+62812")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
