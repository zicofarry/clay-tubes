//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/repository"
)

func setupTestDB(t *testing.T) (*sql.DB, *redis.Client) {
	// Connect to PostgreSQL (from docker-compose)
	dsn := "postgres://clay_user:clay_password@localhost:5434/user_db?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Wait for db to be ready
	for i := 0; i < 5; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Connect to Redis (from docker-compose)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6372",
		DB:   0,
	})

	// Inline schema
	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";

	CREATE TABLE IF NOT EXISTS user_profiles (
		id UUID PRIMARY KEY,
		user_id UUID UNIQUE NOT NULL,
		full_name VARCHAR(100) NOT NULL,
		avatar_url TEXT,
		birth_date DATE,
		gender VARCHAR(20),
		referral_code VARCHAR(20) UNIQUE NOT NULL,
		referred_by UUID,
		phone VARCHAR(20),
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_addresses (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL,
		label VARCHAR(50) NOT NULL,
		address TEXT NOT NULL,
		lat DECIMAL(10,7) NOT NULL,
		lng DECIMAL(10,7) NOT NULL,
		notes TEXT,
		is_default BOOLEAN NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS driver_profiles (
		id UUID PRIMARY KEY,
		user_id UUID UNIQUE NOT NULL,
		vehicle_type VARCHAR(20) NOT NULL,
		plate_number VARCHAR(20) UNIQUE NOT NULL,
		vehicle_brand VARCHAR(50),
		vehicle_model VARCHAR(50),
		vehicle_year SMALLINT,
		vehicle_color VARCHAR(30),
		sim_number VARCHAR(30) UNIQUE NOT NULL,
		ktp_number VARCHAR(30) UNIQUE NOT NULL,
		verification_status VARCHAR(20) NOT NULL,
		rating_avg DECIMAL(3,2),
		total_trips INTEGER,
		is_online BOOLEAN NOT NULL,
		last_online_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS driver_documents (
		id UUID PRIMARY KEY,
		driver_id UUID NOT NULL,
		type VARCHAR(20) NOT NULL,
		file_url TEXT NOT NULL,
		status VARCHAR(20) NOT NULL,
		rejection_reason TEXT,
		verified_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE TABLE IF NOT EXISTS user_settings (
		user_id UUID PRIMARY KEY,
		language VARCHAR(10),
		notif_enabled BOOLEAN,
		marketing_enabled BOOLEAN,
		updated_at TIMESTAMP NOT NULL
	);

	TRUNCATE TABLE user_profiles, user_addresses, driver_profiles, driver_documents, user_settings CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db, rdb
}

// ─── Profile Tests ───────────────────────────────────────────────────────────

func TestProfileCRUD(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	userID := uuid.New()
	profileID := uuid.New()
	now := time.Now()

	t.Run("Create Profile", func(t *testing.T) {
		profile := &models.UserProfile{
			ID: profileID, UserID: userID,
			FullName: "Zico Farry", Gender: "male",
			ReferralCode: "ZICO01", CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateProfile(ctx, profile)
		require.NoError(t, err)
	})

	t.Run("Get Profile by UserID", func(t *testing.T) {
		p, err := repo.GetProfileByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "Zico Farry", p.FullName)
		assert.Equal(t, "male", p.Gender)
		assert.Equal(t, "ZICO01", p.ReferralCode)
	})

	t.Run("Get Profile by UserID - cached from Redis", func(t *testing.T) {
		p, err := repo.GetProfileByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "Zico Farry", p.FullName)
	})

	t.Run("Update Profile", func(t *testing.T) {
		bd := "1999-05-20"
		updated := &models.UserProfile{
			UserID: userID, FullName: "Zico Updated",
			AvatarURL: "https://img.test/avatar.jpg",
			BirthDate: &bd, Gender: "male", UpdatedAt: time.Now(),
		}
		err := repo.UpdateProfile(ctx, updated)
		require.NoError(t, err)

		p, err := repo.GetProfileByUserID(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "Zico Updated", p.FullName)
		assert.Equal(t, "https://img.test/avatar.jpg", p.AvatarURL)
		require.NotNil(t, p.BirthDate)
	})

	t.Run("Get Profile Not Found", func(t *testing.T) {
		p, err := repo.GetProfileByUserID(ctx, uuid.New())
		require.NoError(t, err)
		assert.Nil(t, p)
	})

	t.Run("Apply Referral", func(t *testing.T) {
		referrerID := uuid.New()
		err := repo.ApplyReferral(ctx, userID, referrerID)
		require.NoError(t, err)

		p, err := repo.GetProfileByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, p.ReferredBy)
		assert.Equal(t, referrerID, *p.ReferredBy)
	})

	t.Run("Create Duplicate ReferralCode Fails", func(t *testing.T) {
		dup := &models.UserProfile{
			ID: uuid.New(), UserID: uuid.New(),
			FullName: "Duplicate", ReferralCode: "ZICO01",
			CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateProfile(ctx, dup)
		assert.Error(t, err, "should fail on unique referral_code constraint")
	})
}

// ─── Address Tests ───────────────────────────────────────────────────────────

func TestAddressCRUD(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	userID := uuid.New()
	now := time.Now()

	// Seed a profile for FK-free test (no FK on user_addresses, but we need userID)
	addr1ID := uuid.New()
	addr2ID := uuid.New()

	t.Run("Create Address", func(t *testing.T) {
		addr := &models.UserAddress{
			ID: addr1ID, UserID: userID,
			Label: "Home", Address: "Jl. Sudirman No. 1, Jakarta",
			Lat: -6.2088, Lng: 106.8456, Notes: "Pagar hitam",
			IsDefault: true, CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateAddress(ctx, addr)
		require.NoError(t, err)
	})

	t.Run("Create Second Address", func(t *testing.T) {
		addr := &models.UserAddress{
			ID: addr2ID, UserID: userID,
			Label: "Office", Address: "Jl. Gatot Subroto No. 10, Jakarta",
			Lat: -6.2350, Lng: 106.8270, Notes: "Lantai 5",
			IsDefault: false, CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateAddress(ctx, addr)
		require.NoError(t, err)
	})

	t.Run("List Addresses", func(t *testing.T) {
		addrs, err := repo.ListAddresses(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, addrs, 2)
	})

	t.Run("Get Address by ID", func(t *testing.T) {
		addr, err := repo.GetAddress(ctx, addr1ID)
		require.NoError(t, err)
		require.NotNil(t, addr)
		assert.Equal(t, "Home", addr.Label)
		assert.Equal(t, -6.2088, addr.Lat)
		assert.True(t, addr.IsDefault)
	})

	t.Run("Update Address", func(t *testing.T) {
		updated := &models.UserAddress{
			ID: addr1ID, UserID: userID,
			Label: "Rumah Utama", Address: "Jl. Sudirman No. 1A, Jakarta",
			Lat: -6.2090, Lng: 106.8460, Notes: "Pagar biru",
			IsDefault: true, UpdatedAt: time.Now(),
		}
		err := repo.UpdateAddress(ctx, updated)
		require.NoError(t, err)

		addr, err := repo.GetAddress(ctx, addr1ID)
		require.NoError(t, err)
		assert.Equal(t, "Rumah Utama", addr.Label)
		assert.Equal(t, "Pagar biru", addr.Notes)
	})

	t.Run("Set Default Address", func(t *testing.T) {
		err := repo.SetDefaultAddress(ctx, addr2ID, userID)
		require.NoError(t, err)

		// addr2 should now be default
		a2, err := repo.GetAddress(ctx, addr2ID)
		require.NoError(t, err)
		assert.True(t, a2.IsDefault)

		// addr1 should no longer be default
		a1, err := repo.GetAddress(ctx, addr1ID)
		require.NoError(t, err)
		assert.False(t, a1.IsDefault)
	})

	t.Run("Delete Address", func(t *testing.T) {
		err := repo.DeleteAddress(ctx, addr1ID, userID)
		require.NoError(t, err)

		addrs, err := repo.ListAddresses(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, addrs, 1)
		assert.Equal(t, addr2ID, addrs[0].ID)
	})

	t.Run("List Addresses Empty for Unknown User", func(t *testing.T) {
		addrs, err := repo.ListAddresses(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, addrs)
	})
}

// ─── Driver Profile Tests ────────────────────────────────────────────────────

func TestDriverProfileCRUD(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now()

	t.Run("Create Driver Profile", func(t *testing.T) {
		dp := &models.DriverProfile{
			ID: driverID, UserID: userID,
			VehicleType: "motor", PlateNumber: "B1234XY",
			VehicleBrand: "Honda", VehicleModel: "Vario 160",
			VehicleYear: 2023, VehicleColor: "Hitam",
			SimNumber: "SIM001", KtpNumber: "KTP001",
			VerificationStatus: "pending",
			RatingAvg: 0, TotalTrips: 0, IsOnline: false,
			CreatedAt: now,
		}
		err := repo.CreateDriverProfile(ctx, dp)
		require.NoError(t, err)
	})

	t.Run("Get Driver Profile by UserID", func(t *testing.T) {
		dp, err := repo.GetDriverProfileByUserID(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, dp)
		assert.Equal(t, "motor", dp.VehicleType)
		assert.Equal(t, "B1234XY", dp.PlateNumber)
		assert.Equal(t, "Honda", dp.VehicleBrand)
		assert.Equal(t, "pending", dp.VerificationStatus)
		assert.False(t, dp.IsOnline)
	})

	t.Run("Get Driver Profile by ID", func(t *testing.T) {
		dp, err := repo.GetDriverProfileByID(ctx, driverID)
		require.NoError(t, err)
		require.NotNil(t, dp)
		assert.Equal(t, userID, dp.UserID)
	})

	t.Run("Update Driver Profile", func(t *testing.T) {
		updated := &models.DriverProfile{
			ID: driverID, UserID: userID,
			VehicleType: "car", PlateNumber: "B5678AB",
			VehicleBrand: "Toyota", VehicleModel: "Avanza",
			VehicleYear: 2024, VehicleColor: "Putih",
		}
		err := repo.UpdateDriverProfile(ctx, updated)
		require.NoError(t, err)

		dp, err := repo.GetDriverProfileByID(ctx, driverID)
		require.NoError(t, err)
		assert.Equal(t, "car", dp.VehicleType)
		assert.Equal(t, "B5678AB", dp.PlateNumber)
		assert.Equal(t, "Toyota", dp.VehicleBrand)
		assert.Equal(t, "Putih", dp.VehicleColor)
	})

	t.Run("Toggle Driver Online", func(t *testing.T) {
		err := repo.ToggleDriverOnline(ctx, driverID, true)
		require.NoError(t, err)

		dp, err := repo.GetDriverProfileByID(ctx, driverID)
		require.NoError(t, err)
		assert.True(t, dp.IsOnline)
		assert.NotNil(t, dp.LastOnlineAt)
	})

	t.Run("Toggle Driver Offline", func(t *testing.T) {
		err := repo.ToggleDriverOnline(ctx, driverID, false)
		require.NoError(t, err)

		dp, err := repo.GetDriverProfileByID(ctx, driverID)
		require.NoError(t, err)
		assert.False(t, dp.IsOnline)
	})

	t.Run("Get Driver Not Found", func(t *testing.T) {
		dp, err := repo.GetDriverProfileByUserID(ctx, uuid.New())
		require.NoError(t, err)
		assert.Nil(t, dp)
	})

	t.Run("Duplicate PlateNumber Fails", func(t *testing.T) {
		dup := &models.DriverProfile{
			ID: uuid.New(), UserID: uuid.New(),
			VehicleType: "motor", PlateNumber: "B5678AB",
			SimNumber: "SIM999", KtpNumber: "KTP999",
			VerificationStatus: "pending", CreatedAt: now,
		}
		err := repo.CreateDriverProfile(ctx, dup)
		assert.Error(t, err, "should fail on unique plate_number")
	})
}

// ─── Driver Document Tests ───────────────────────────────────────────────────

func TestDriverDocumentCRUD(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	driverID := uuid.New()
	now := time.Now()
	ktpDocID := uuid.New()
	simDocID := uuid.New()

	t.Run("Create KTP Document", func(t *testing.T) {
		doc := &models.DriverDocument{
			ID: ktpDocID, DriverID: driverID,
			Type: "ktp", FileURL: "https://storage.test/ktp.jpg",
			Status: "pending", CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateDriverDocument(ctx, doc)
		require.NoError(t, err)
	})

	t.Run("Create SIM Document", func(t *testing.T) {
		doc := &models.DriverDocument{
			ID: simDocID, DriverID: driverID,
			Type: "sim", FileURL: "https://storage.test/sim.jpg",
			Status: "pending", CreatedAt: now, UpdatedAt: now,
		}
		err := repo.CreateDriverDocument(ctx, doc)
		require.NoError(t, err)
	})

	t.Run("List Driver Documents", func(t *testing.T) {
		docs, err := repo.ListDriverDocuments(ctx, driverID)
		require.NoError(t, err)
		assert.Len(t, docs, 2)
	})

	t.Run("Get Document by ID", func(t *testing.T) {
		doc, err := repo.GetDriverDocument(ctx, ktpDocID)
		require.NoError(t, err)
		require.NotNil(t, doc)
		assert.Equal(t, "ktp", doc.Type)
		assert.Equal(t, "pending", doc.Status)
		assert.Nil(t, doc.VerifiedAt)
	})

	t.Run("Approve Document", func(t *testing.T) {
		err := repo.UpdateDriverDocumentStatus(ctx, ktpDocID, "approved", "")
		require.NoError(t, err)

		doc, err := repo.GetDriverDocument(ctx, ktpDocID)
		require.NoError(t, err)
		assert.Equal(t, "approved", doc.Status)
		assert.NotNil(t, doc.VerifiedAt, "verified_at should be set on approval")
		assert.Empty(t, doc.RejectionReason)
	})

	t.Run("Reject Document", func(t *testing.T) {
		err := repo.UpdateDriverDocumentStatus(ctx, simDocID, "rejected", "Foto buram")
		require.NoError(t, err)

		doc, err := repo.GetDriverDocument(ctx, simDocID)
		require.NoError(t, err)
		assert.Equal(t, "rejected", doc.Status)
		assert.Equal(t, "Foto buram", doc.RejectionReason)
		assert.Nil(t, doc.VerifiedAt, "verified_at should be nil on rejection")
	})

	t.Run("Delete Pending Document OK", func(t *testing.T) {
		newDocID := uuid.New()
		doc := &models.DriverDocument{
			ID: newDocID, DriverID: driverID,
			Type: "stnk", FileURL: "https://storage.test/stnk.jpg",
			Status: "pending", CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, repo.CreateDriverDocument(ctx, doc))

		err := repo.DeleteDriverDocument(ctx, newDocID, driverID)
		require.NoError(t, err)
	})

	t.Run("Delete Non-Pending Document Fails", func(t *testing.T) {
		// ktpDocID is "approved", should not be deletable
		err := repo.DeleteDriverDocument(ctx, ktpDocID, driverID)
		assert.Error(t, err, "should fail: cannot delete approved doc")
	})

	t.Run("Get Document Not Found", func(t *testing.T) {
		doc, err := repo.GetDriverDocument(ctx, uuid.New())
		require.NoError(t, err)
		assert.Nil(t, doc)
	})

	t.Run("List Documents Empty for Unknown Driver", func(t *testing.T) {
		docs, err := repo.ListDriverDocuments(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, docs)
	})
}

// ─── Settings Tests ──────────────────────────────────────────────────────────

func TestSettingsCRUD(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	userID := uuid.New()

	t.Run("Get Default Settings (no row)", func(t *testing.T) {
		s, err := repo.GetSettings(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, s)
		assert.Equal(t, "en", s.Language)
		assert.True(t, s.NotifEnabled)
		assert.False(t, s.MarketingEnabled)
	})

	t.Run("Create Settings via Upsert", func(t *testing.T) {
		err := repo.UpdateSettings(ctx, &models.UserSettings{
			UserID: userID, Language: "id",
			NotifEnabled: true, MarketingEnabled: true,
			UpdatedAt: time.Now(),
		})
		require.NoError(t, err)

		s, err := repo.GetSettings(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "id", s.Language)
		assert.True(t, s.NotifEnabled)
		assert.True(t, s.MarketingEnabled)
	})

	t.Run("Update Settings via Upsert", func(t *testing.T) {
		err := repo.UpdateSettings(ctx, &models.UserSettings{
			UserID: userID, Language: "en",
			NotifEnabled: false, MarketingEnabled: false,
			UpdatedAt: time.Now(),
		})
		require.NoError(t, err)

		s, err := repo.GetSettings(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "en", s.Language)
		assert.False(t, s.NotifEnabled)
		assert.False(t, s.MarketingEnabled)
	})
}

// ─── Phone Lookup Tests ──────────────────────────────────────────────────────

func TestLookupUserByPhone(t *testing.T) {
	db, rdb := setupTestDB(t)
	defer db.Close()
	repo := repository.NewUserRepository(db, rdb)
	ctx := context.Background()

	userID := uuid.New()
	now := time.Now()

	// Create profile with phone
	profile := &models.UserProfile{
		ID: uuid.New(), UserID: userID,
		FullName: "Phone User", ReferralCode: "PHONE01",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, repo.CreateProfile(ctx, profile))

	// Set phone manually since CreateProfile doesn't set it in the model
	_, err := db.ExecContext(ctx, `UPDATE user_profiles SET phone = $1 WHERE user_id = $2`, "+6281234567890", userID)
	require.NoError(t, err)

	t.Run("Lookup Existing Phone", func(t *testing.T) {
		p, err := repo.LookupUserByPhone(ctx, "+6281234567890")
		require.NoError(t, err)
		require.NotNil(t, p)
		assert.Equal(t, "Phone User", p.FullName)
		assert.Equal(t, userID, p.UserID)
	})

	t.Run("Lookup Non-Existing Phone", func(t *testing.T) {
		p, err := repo.LookupUserByPhone(ctx, "+6289999999999")
		require.NoError(t, err)
		assert.Nil(t, p)
	})
}
