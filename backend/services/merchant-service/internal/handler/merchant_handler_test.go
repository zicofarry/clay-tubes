//go:build unit

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func newHandler(ctrl *gomock.Controller) (*MerchantHandler, *mocks.MockMerchantServiceInterface) {
	mockSvc := mocks.NewMockMerchantServiceInterface(ctrl)
	return NewMerchantHandler(mockSvc, newTestLogger()), mockSvc
}

func withUserID(r *http.Request, userID string) *http.Request {
	r.Header.Set("X-User-ID", userID)
	return r
}

// ── RegisterMerchant ──────────────────────────────────────────────────────────

func TestRegisterMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, _ := newHandler(ctrl)
	// No EXPECT — RegisterMerchant requires auth context (middleware sets userID).
	// In unit tests without full middleware stack, the handler returns 401.
	body := `{"name":"Warung Test","category":"food","phone_number":"08111","address":"Jl A","city":"Jakarta","lat":-6.2,"lng":106.8}`
	req := httptest.NewRequest("POST", "/merchants", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.RegisterMerchant(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (no auth ctx), got %d", w.Code)
	}
}

func TestRegisterMerchant_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, _ := newHandler(ctrl)
	// No mock EXPECT — handler returns 401 (no userID in context) before reaching service
	body := `{"name":"Warung","category":"food","phone_number":"08111","address":"Jl A","city":"Jakarta"}`
	req := httptest.NewRequest("POST", "/merchants", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.RegisterMerchant(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (no auth), got %d", w.Code)
	}
}

func TestRegisterMerchant_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, _ := newHandler(ctrl)

	req := httptest.NewRequest("POST", "/merchants", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()
	h.RegisterMerchant(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (no auth), got %d", w.Code)
	}
}

// ── GetMyMerchant ─────────────────────────────────────────────────────────────

func TestGetMyMerchant_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().GetMyMerchant(gomock.Any(), gomock.Any()).Return(nil, service.ErrMerchantNotFound)

	req := httptest.NewRequest("GET", "/merchants/me", nil)
	w := httptest.NewRecorder()
	h.GetMyMerchant(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetMyMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	m := &model.Merchant{ID: "m-1", Name: "Test"}
	mockSvc.EXPECT().GetMyMerchant(gomock.Any(), gomock.Any()).Return(m, nil)

	req := httptest.NewRequest("GET", "/merchants/me", nil)
	w := httptest.NewRecorder()
	h.GetMyMerchant(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp response.SuccessResp
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
}

// ── GetMerchantByID ───────────────────────────────────────────────────────────

func TestGetMerchantByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	m := &model.Merchant{ID: "m-abc", Name: "Merchant ABC", Status: model.StatusActive}
	mockSvc.EXPECT().GetMerchantByID(gomock.Any(), "m-abc").Return(m, nil)

	req := httptest.NewRequest("GET", "/merchants/m-abc", nil)
	req.SetPathValue("merchantId", "m-abc")
	w := httptest.NewRecorder()
	h.GetMerchantByID(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetMerchantByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().GetMerchantByID(gomock.Any(), "missing").Return(nil, service.ErrMerchantNotFound)

	req := httptest.NewRequest("GET", "/merchants/missing", nil)
	req.SetPathValue("merchantId", "missing")
	w := httptest.NewRecorder()
	h.GetMerchantByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── UpdateMerchantStatus ──────────────────────────────────────────────────────

func TestUpdateMerchantStatus_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().UpdateMerchantStatus(gomock.Any(), "m-1", gomock.Any(), gomock.Any()).Return(nil, service.ErrForbidden)

	body := `{"status":"closed"}`
	req := httptest.NewRequest("PATCH", "/merchants/m-1/status", strings.NewReader(body))
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.UpdateMerchantStatus(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestUpdateMerchantStatus_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, _ := newHandler(ctrl)

	req := httptest.NewRequest("PATCH", "/merchants/m-1/status", strings.NewReader(`bad`))
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.UpdateMerchantStatus(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── GetOperatingHours ─────────────────────────────────────────────────────────

func TestGetOperatingHours_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	hours := []model.OperatingHours{{ID: "h-1", DayOfWeek: 1, OpenTime: "08:00", CloseTime: "22:00"}}
	mockSvc.EXPECT().GetOperatingHours(gomock.Any(), "m-1").Return(hours, nil)

	req := httptest.NewRequest("GET", "/merchants/m-1/operating-hours", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.GetOperatingHours(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── ListBankAccounts ──────────────────────────────────────────────────────────

func TestListBankAccounts_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().ListBankAccounts(gomock.Any(), "m-1", gomock.Any()).Return(nil, service.ErrForbidden)

	req := httptest.NewRequest("GET", "/merchants/m-1/bank-accounts", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.ListBankAccounts(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestAddBankAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	ba := &model.BankAccount{ID: "ba-1", BankCode: "BCA", AccountNumber: "123", AccountName: "Test"}
	mockSvc.EXPECT().AddBankAccount(gomock.Any(), "m-1", gomock.Any(), gomock.Any()).Return(ba, nil)

	body := `{"bank_code":"BCA","account_number":"123","account_name":"Test"}`
	req := httptest.NewRequest("POST", "/merchants/m-1/bank-accounts", strings.NewReader(body))
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.AddBankAccount(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestDeleteBankAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().DeleteBankAccount(gomock.Any(), "m-1", "ba-1", gomock.Any()).Return(nil)

	req := httptest.NewRequest("DELETE", "/merchants/m-1/bank-accounts/ba-1", nil)
	req.SetPathValue("merchantId", "m-1")
	req.SetPathValue("accountId", "ba-1")
	w := httptest.NewRecorder()
	h.DeleteBankAccount(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── Menu Categories ───────────────────────────────────────────────────────────

func TestListCategories_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	cats := []model.MenuCategory{{ID: "cat-1", Name: "Makanan Berat"}}
	mockSvc.EXPECT().ListCategories(gomock.Any(), "m-1").Return(cats, nil)

	req := httptest.NewRequest("GET", "/merchants/m-1/menu/categories", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.ListCategories(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateCategory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	cat := &model.MenuCategory{ID: "cat-1", Name: "Minuman"}
	mockSvc.EXPECT().CreateCategory(gomock.Any(), "m-1", gomock.Any(), gomock.Any()).Return(cat, nil)

	body := `{"name":"Minuman","display_order":1}`
	req := httptest.NewRequest("POST", "/merchants/m-1/menu/categories", strings.NewReader(body))
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.CreateCategory(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestDeleteCategory_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().DeleteCategory(gomock.Any(), "m-1", "cat-x", gomock.Any()).Return(service.ErrCategoryNotFound)

	req := httptest.NewRequest("DELETE", "/merchants/m-1/menu/categories/cat-x", nil)
	req.SetPathValue("merchantId", "m-1")
	req.SetPathValue("categoryId", "cat-x")
	w := httptest.NewRecorder()
	h.DeleteCategory(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── Menu Items ────────────────────────────────────────────────────────────────

func TestListItems_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	items := []model.MenuItem{{ID: "item-1", Name: "Nasi Goreng", PriceCents: 25000}}
	mockSvc.EXPECT().ListItems(gomock.Any(), "m-1", "", gomock.Any()).Return(items, nil)

	req := httptest.NewRequest("GET", "/merchants/m-1/menu/items", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.ListItems(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	item := &model.MenuItem{ID: "item-1", Name: "Mie Goreng", PriceCents: 20000, IsAvailable: true}
	mockSvc.EXPECT().CreateItem(gomock.Any(), "m-1", gomock.Any(), gomock.Any()).Return(item, nil)

	body := `{"category_id":"cat-1","name":"Mie Goreng","price_cents":20000}`
	req := httptest.NewRequest("POST", "/merchants/m-1/menu/items", strings.NewReader(body))
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.CreateItem(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestGetItem_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().GetItem(gomock.Any(), "m-1", "item-x").Return(nil, nil)

	req := httptest.NewRequest("GET", "/merchants/m-1/menu/items/item-x", nil)
	req.SetPathValue("merchantId", "m-1")
	req.SetPathValue("itemId", "item-x")
	w := httptest.NewRecorder()
	h.GetItem(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().DeleteItem(gomock.Any(), "m-1", "item-1", gomock.Any()).Return(nil)

	req := httptest.NewRequest("DELETE", "/merchants/m-1/menu/items/item-1", nil)
	req.SetPathValue("merchantId", "m-1")
	req.SetPathValue("itemId", "item-1")
	w := httptest.NewRecorder()
	h.DeleteItem(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestToggleAvailability_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	item := &model.MenuItem{ID: "item-1", IsAvailable: false}
	mockSvc.EXPECT().ToggleAvailability(gomock.Any(), "m-1", "item-1", gomock.Any(), gomock.Any()).Return(item, nil)

	body := `{"is_available":false}`
	req := httptest.NewRequest("PATCH", "/merchants/m-1/menu/items/item-1/availability", strings.NewReader(body))
	req.SetPathValue("merchantId", "m-1")
	req.SetPathValue("itemId", "item-1")
	w := httptest.NewRecorder()
	h.ToggleAvailability(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── Internal Endpoints ────────────────────────────────────────────────────────

func TestInternalGetMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	m := &model.Merchant{ID: "m-1", Status: model.StatusActive}
	mockSvc.EXPECT().GetMerchantByID(gomock.Any(), "m-1").Return(m, nil)

	req := httptest.NewRequest("GET", "/internal/merchants/m-1", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.InternalGetMerchant(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestInternalIsOpen_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	mockSvc.EXPECT().IsOpen(gomock.Any(), "m-1").Return(&model.IsOpenResponse{IsOpen: true}, nil)

	req := httptest.NewRequest("GET", "/internal/merchants/m-1/is-open", nil)
	req.SetPathValue("merchantId", "m-1")
	w := httptest.NewRecorder()
	h.InternalIsOpen(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestInternalBatchGetItems_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	h, mockSvc := newHandler(ctrl)

	items := []model.MenuItem{{ID: "item-1"}, {ID: "item-2"}}
	mockSvc.EXPECT().BatchGetItems(gomock.Any(), gomock.Any()).Return(items, nil)

	body := `{"item_ids":["item-1","item-2"]}`
	req := httptest.NewRequest("POST", "/internal/menu-items/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.InternalBatchGetItems(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
