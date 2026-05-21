//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
	"github.com/zicofarry/clay-app/backend/services/user-service/mocks"
	"go.uber.org/mock/gomock"
)

func newTestHandler(t *testing.T) (*UserHandler, *mocks.MockUserServiceInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockUserServiceInterface(ctrl)
	handler := NewUserHandler(mockSvc)
	return handler, mockSvc, ctrl
}

func TestUserHandler_GetMyProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		expectedResp := &models.ProfileResponse{
			FullName: "John Doe",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().GetProfile(gomock.Any(), expectedUserID).Return(expectedResp, nil)

		req, _ := http.NewRequest("GET", "/users/me", nil)
		rr := httptest.NewRecorder()

		handler.GetMyProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		
		var res map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&res)
		data := res["data"].(map[string]interface{})
		assert.Equal(t, "John Doe", data["full_name"])
	})
}

func TestUserHandler_CreateProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.CreateProfileRequest{
			FullName: "Jane Doe",
			Gender:   "female",
		}
		expectedResp := &models.ProfileResponse{
			FullName: "Jane Doe",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().CreateProfile(gomock.Any(), expectedUserID, reqBody).Return(expectedResp, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/users/me", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.CreateProfile(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestUserHandler_UpdateProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.UpdateProfileRequest{
			FullName: "Jane Updated",
		}
		expectedResp := &models.ProfileResponse{
			FullName: "Jane Updated",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().UpdateProfile(gomock.Any(), expectedUserID, reqBody).Return(expectedResp, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/users/me", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.UpdateProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetProfileByUserId(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		targetUserID := uuid.New()
		expectedResp := &models.PublicProfileResponse{
			FullName: "Target User",
		}
		
		mockSvc.EXPECT().GetPublicProfile(gomock.Any(), targetUserID).Return(expectedResp, nil)

		req, _ := http.NewRequest("GET", "/users/"+targetUserID.String(), nil)
		req.SetPathValue("userId", targetUserID.String())
		rr := httptest.NewRecorder()

		handler.GetProfileByUserId(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_ApplyReferralCode(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := struct {
			ReferralCode string `json:"referral_code"`
		}{ReferralCode: "REF123"}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().ApplyReferralCode(gomock.Any(), expectedUserID, "REF123").Return(nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/users/me/referral/apply", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.ApplyReferralCode(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_ListAddresses(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().ListAddresses(gomock.Any(), expectedUserID).Return([]models.AddressResponse{}, nil)

		req, _ := http.NewRequest("GET", "/addresses", nil)
		rr := httptest.NewRecorder()

		handler.ListAddresses(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_CreateAddress(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.AddressRequest{
			Label:   "Home",
			Address: "Street 1",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().CreateAddress(gomock.Any(), expectedUserID, reqBody).Return(&models.AddressResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/addresses", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.CreateAddress(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestUserHandler_UpdateAddress(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		addrID := uuid.New()
		reqBody := models.AddressRequest{
			Label:   "Work",
			Address: "Street 2",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().UpdateAddress(gomock.Any(), expectedUserID, addrID, reqBody).Return(&models.AddressResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/addresses/"+addrID.String(), bytes.NewBuffer(body))
		req.SetPathValue("addressId", addrID.String())
		rr := httptest.NewRecorder()

		handler.UpdateAddress(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_DeleteAddress(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		addrID := uuid.New()
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().DeleteAddress(gomock.Any(), expectedUserID, addrID).Return(nil)

		req, _ := http.NewRequest("DELETE", "/addresses/"+addrID.String(), nil)
		req.SetPathValue("addressId", addrID.String())
		rr := httptest.NewRecorder()

		handler.DeleteAddress(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_SetDefaultAddress(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		addrID := uuid.New()
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().SetDefaultAddress(gomock.Any(), expectedUserID, addrID).Return(nil)

		req, _ := http.NewRequest("PUT", "/addresses/"+addrID.String()+"/default", nil)
		req.SetPathValue("addressId", addrID.String())
		rr := httptest.NewRecorder()

		handler.SetDefaultAddress(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetDriverProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().GetDriverProfile(gomock.Any(), expectedUserID).Return(&models.DriverProfileResponse{}, nil)

		req, _ := http.NewRequest("GET", "/drivers/me", nil)
		rr := httptest.NewRecorder()

		handler.GetDriverProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_CreateDriverProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.CreateDriverProfileRequest{
			VehicleType: "car",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().CreateDriverProfile(gomock.Any(), expectedUserID, reqBody).Return(&models.DriverProfileResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/drivers/register", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.CreateDriverProfile(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestUserHandler_UpdateDriverProfile(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.UpdateDriverProfileRequest{
			VehicleColor: "red",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().UpdateDriverProfile(gomock.Any(), expectedUserID, reqBody).Return(&models.DriverProfileResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/drivers/me", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.UpdateDriverProfile(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_ToggleDriverOnline(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		driverID := uuid.New()
		reqBody := struct {
			IsOnline bool `json:"is_online"`
		}{IsOnline: true}
		
		mockSvc.EXPECT().ToggleDriverOnline(gomock.Any(), driverID, true).Return(&models.DriverProfileResponse{IsOnline: true}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/drivers/"+driverID.String()+"/status", bytes.NewBuffer(body))
		req.SetPathValue("driverId", driverID.String())
		rr := httptest.NewRecorder()

		handler.ToggleDriverOnline(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_ListDriverDocuments(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().ListDriverDocuments(gomock.Any(), expectedUserID).Return([]models.DocumentResponse{}, nil)

		req, _ := http.NewRequest("GET", "/drivers/me/documents", nil)
		rr := httptest.NewRecorder()

		handler.ListDriverDocuments(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_UploadDocument(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := struct {
			Type string `json:"type"`
		}{Type: "ktp"}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		// The handler currently uses a hardcoded URL "https://example.com/doc.jpg"
		mockSvc.EXPECT().UploadDocument(gomock.Any(), expectedUserID, "ktp", "https://example.com/doc.jpg").Return(&models.DocumentResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/drivers/me/documents", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.UploadDocument(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestUserHandler_VerifyDocument(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		docID := uuid.New()
		reqBody := struct {
			Status          string `json:"status"`
			RejectionReason string `json:"rejection_reason"`
		}{Status: "approved"}
		
		mockSvc.EXPECT().VerifyDocument(gomock.Any(), docID, "approved", "").Return(&models.DocumentResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/admin/documents/"+docID.String()+"/verify", bytes.NewBuffer(body))
		req.SetPathValue("documentId", docID.String())
		rr := httptest.NewRecorder()

		handler.VerifyDocument(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetSettings(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().GetSettings(gomock.Any(), expectedUserID).Return(&models.SettingsResponse{}, nil)

		req, _ := http.NewRequest("GET", "/settings", nil)
		rr := httptest.NewRecorder()

		handler.GetSettings(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_UpdateSettings(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.UpdateSettingsRequest{
			Language: "id",
		}
		
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().UpdateSettings(gomock.Any(), expectedUserID, reqBody).Return(&models.SettingsResponse{}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/settings", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.UpdateSettings(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetDriverProfileById(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		driverID := uuid.New()
		mockSvc.EXPECT().GetPublicDriverProfile(gomock.Any(), driverID).Return(&models.DriverPublicProfileResponse{}, nil)

		req, _ := http.NewRequest("GET", "/drivers/"+driverID.String(), nil)
		req.SetPathValue("driverId", driverID.String())
		rr := httptest.NewRecorder()

		handler.GetDriverProfileById(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetDocument(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		docID := uuid.New()
		mockSvc.EXPECT().GetDocument(gomock.Any(), docID).Return(&models.DocumentResponse{}, nil)

		req, _ := http.NewRequest("GET", "/drivers/me/documents/"+docID.String(), nil)
		req.SetPathValue("documentId", docID.String())
		rr := httptest.NewRecorder()

		handler.GetDocument(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_DeleteDocument(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		docID := uuid.New()
		expectedUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
		mockSvc.EXPECT().DeleteDocument(gomock.Any(), expectedUserID, docID).Return(nil)

		req, _ := http.NewRequest("DELETE", "/drivers/me/documents/"+docID.String(), nil)
		req.SetPathValue("documentId", docID.String())
		rr := httptest.NewRecorder()

		handler.DeleteDocument(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_GetDriverVerificationStatus(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		driverID := uuid.New()
		mockSvc.EXPECT().GetDriverVerificationStatus(gomock.Any(), driverID).Return([]models.DocumentResponse{}, nil)

		req, _ := http.NewRequest("GET", "/drivers/"+driverID.String()+"/verification", nil)
		req.SetPathValue("driverId", driverID.String())
		rr := httptest.NewRecorder()

		handler.GetDriverVerificationStatus(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestUserHandler_LookupUserByPhone(t *testing.T) {
	handler, mockSvc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		reqBody := models.LookupUserByPhoneRequest{
			Phone: "+628123456789",
		}
		
		mockSvc.EXPECT().LookupUserByPhone(gomock.Any(), reqBody.Phone).Return(&models.LookupUserByPhoneResponse{Found: true}, nil)

		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("POST", "/internal/users/lookup-by-phone", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()

		handler.LookupUserByPhone(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
