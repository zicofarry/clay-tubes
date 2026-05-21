//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
	"github.com/zicofarry/clay-app/backend/services/user-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*UserService, *repomock.MockUserRepositoryInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockUserRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewUserService(mockRepo, logger)
	return svc, mockRepo, ctrl
}

func TestUserService_GetProfile(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		expectedProfile := &models.UserProfile{
			ID:        uuid.New(),
			UserID:    userID,
			FullName:  "John Doe",
			CreatedAt: time.Now(),
		}
		mockRepo.EXPECT().GetProfileByUserID(gomock.Any(), userID).Return(expectedProfile, nil)

		resp, err := svc.GetProfile(context.Background(), userID)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", resp.FullName)
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo.EXPECT().GetProfileByUserID(gomock.Any(), userID).Return(nil, nil)

		resp, err := svc.GetProfile(context.Background(), userID)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUserService_CreateProfile(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		req := models.CreateProfileRequest{
			FullName: "Jane Doe",
		}
		mockRepo.EXPECT().GetProfileByUserID(gomock.Any(), userID).Return(nil, nil)
		mockRepo.EXPECT().CreateProfile(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.CreateProfile(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "Jane Doe", resp.FullName)
	})

	t.Run("already exists", func(t *testing.T) {
		req := models.CreateProfileRequest{
			FullName: "Jane Doe",
		}
		mockRepo.EXPECT().GetProfileByUserID(gomock.Any(), userID).Return(&models.UserProfile{}, nil)

		resp, err := svc.CreateProfile(context.Background(), userID, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUserService_UpdateProfile(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		req := models.UpdateProfileRequest{
			FullName: "Jane Updated",
		}
		mockRepo.EXPECT().GetProfileByUserID(gomock.Any(), userID).Return(&models.UserProfile{FullName: "Jane Doe"}, nil)
		mockRepo.EXPECT().UpdateProfile(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.UpdateProfile(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "Jane Updated", resp.FullName)
	})
}

func TestUserService_LookupUserByPhone(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	phone := "+628123456789"

	t.Run("found", func(t *testing.T) {
		mockRepo.EXPECT().LookupUserByPhone(gomock.Any(), phone).Return(&models.UserProfile{UserID: uuid.New(), FullName: "John"}, nil)

		resp, err := svc.LookupUserByPhone(context.Background(), phone)
		assert.NoError(t, err)
		assert.True(t, resp.Found)
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo.EXPECT().LookupUserByPhone(gomock.Any(), phone).Return(nil, nil)

		resp, err := svc.LookupUserByPhone(context.Background(), phone)
		assert.NoError(t, err)
		assert.False(t, resp.Found)
	})
}

func TestUserService_Address(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()
	addrID := uuid.New()

	t.Run("ListAddresses success", func(t *testing.T) {
		mockRepo.EXPECT().ListAddresses(gomock.Any(), userID).Return([]models.UserAddress{{ID: addrID, Label: "Home"}}, nil)

		resp, err := svc.ListAddresses(context.Background(), userID)
		assert.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "Home", resp[0].Label)
	})

	t.Run("CreateAddress success", func(t *testing.T) {
		req := models.AddressRequest{Label: "Work"}
		mockRepo.EXPECT().CreateAddress(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.CreateAddress(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "Work", resp.Label)
	})

	t.Run("UpdateAddress success", func(t *testing.T) {
		req := models.AddressRequest{Label: "Updated Work"}
		mockRepo.EXPECT().GetAddress(gomock.Any(), addrID).Return(&models.UserAddress{ID: addrID, UserID: userID, Label: "Work"}, nil)
		mockRepo.EXPECT().UpdateAddress(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.UpdateAddress(context.Background(), userID, addrID, req)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Work", resp.Label)
	})

	t.Run("DeleteAddress success", func(t *testing.T) {
		mockRepo.EXPECT().DeleteAddress(gomock.Any(), addrID, userID).Return(nil)
		err := svc.DeleteAddress(context.Background(), userID, addrID)
		assert.NoError(t, err)
	})

	t.Run("SetDefaultAddress success", func(t *testing.T) {
		mockRepo.EXPECT().SetDefaultAddress(gomock.Any(), addrID, userID).Return(nil)

		err := svc.SetDefaultAddress(context.Background(), userID, addrID)
		assert.NoError(t, err)
	})
}

func TestUserService_Driver(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()
	driverID := uuid.New()

	t.Run("GetDriverProfile success", func(t *testing.T) {
		mockRepo.EXPECT().GetDriverProfileByUserID(gomock.Any(), userID).Return(&models.DriverProfile{ID: driverID, UserID: userID}, nil)

		resp, err := svc.GetDriverProfile(context.Background(), userID)
		assert.NoError(t, err)
		assert.Equal(t, driverID, resp.ID)
	})

	t.Run("CreateDriverProfile success", func(t *testing.T) {
		req := models.CreateDriverProfileRequest{VehicleType: "bike"}
		mockRepo.EXPECT().GetDriverProfileByUserID(gomock.Any(), userID).Return(nil, nil)
		mockRepo.EXPECT().CreateDriverProfile(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.CreateDriverProfile(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "bike", resp.VehicleType)
	})

	t.Run("UpdateDriverProfile success", func(t *testing.T) {
		req := models.UpdateDriverProfileRequest{VehicleColor: "blue"}
		mockRepo.EXPECT().GetDriverProfileByUserID(gomock.Any(), userID).Return(&models.DriverProfile{ID: driverID, UserID: userID}, nil)
		mockRepo.EXPECT().UpdateDriverProfile(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.UpdateDriverProfile(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "blue", resp.VehicleColor)
	})

	t.Run("ToggleDriverOnline success", func(t *testing.T) {
		mockRepo.EXPECT().GetDriverProfileByID(gomock.Any(), driverID).Return(&models.DriverProfile{ID: driverID, VerificationStatus: "verified"}, nil)
		mockRepo.EXPECT().ToggleDriverOnline(gomock.Any(), driverID, true).Return(nil)

		resp, err := svc.ToggleDriverOnline(context.Background(), driverID, true)
		assert.NoError(t, err)
		assert.True(t, resp.IsOnline)
	})

	t.Run("ToggleDriverOnline forbidden", func(t *testing.T) {
		mockRepo.EXPECT().GetDriverProfileByID(gomock.Any(), driverID).Return(&models.DriverProfile{ID: driverID, VerificationStatus: "pending"}, nil)

		resp, err := svc.ToggleDriverOnline(context.Background(), driverID, true)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUserService_Document(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	driverID := uuid.New()
	docID := uuid.New()

	t.Run("ListDriverDocuments success", func(t *testing.T) {
		mockRepo.EXPECT().ListDriverDocuments(gomock.Any(), driverID).Return([]models.DriverDocument{{ID: docID}}, nil)

		resp, err := svc.ListDriverDocuments(context.Background(), driverID)
		assert.NoError(t, err)
		assert.Len(t, resp, 1)
	})

	t.Run("UploadDocument success", func(t *testing.T) {
		mockRepo.EXPECT().CreateDriverDocument(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.UploadDocument(context.Background(), driverID, "ktp", "http://url")
		assert.NoError(t, err)
		assert.Equal(t, "ktp", resp.Type)
	})

	t.Run("VerifyDocument success", func(t *testing.T) {
		mockRepo.EXPECT().UpdateDriverDocumentStatus(gomock.Any(), docID, "approved", "").Return(nil)
		mockRepo.EXPECT().GetDriverDocument(gomock.Any(), docID).Return(&models.DriverDocument{ID: docID, DriverID: driverID, Status: "approved"}, nil)

		resp, err := svc.VerifyDocument(context.Background(), docID, "approved", "")
		assert.NoError(t, err)
		assert.Equal(t, "approved", resp.Status)
	})

	t.Run("DeleteDocument success", func(t *testing.T) {
		mockRepo.EXPECT().DeleteDriverDocument(gomock.Any(), docID, driverID).Return(nil)
		err := svc.DeleteDocument(context.Background(), driverID, docID)
		assert.NoError(t, err)
	})
}

func TestUserService_Settings(t *testing.T) {
	svc, mockRepo, ctrl := newTestService(t)
	defer ctrl.Finish()

	userID := uuid.New()

	t.Run("GetSettings success", func(t *testing.T) {
		mockRepo.EXPECT().GetSettings(gomock.Any(), userID).Return(&models.UserSettings{UserID: userID, Language: "en"}, nil)

		resp, err := svc.GetSettings(context.Background(), userID)
		assert.NoError(t, err)
		assert.Equal(t, "en", resp.Language)
	})

	t.Run("UpdateSettings success", func(t *testing.T) {
		req := models.UpdateSettingsRequest{Language: "id"}
		mockRepo.EXPECT().GetSettings(gomock.Any(), userID).Return(&models.UserSettings{UserID: userID, Language: "en"}, nil)
		mockRepo.EXPECT().UpdateSettings(gomock.Any(), gomock.Any()).Return(nil)

		resp, err := svc.UpdateSettings(context.Background(), userID, req)
		assert.NoError(t, err)
		assert.Equal(t, "id", resp.Language)
	})
}
