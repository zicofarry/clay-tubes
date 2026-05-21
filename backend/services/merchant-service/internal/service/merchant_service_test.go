//go:build unit

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/mocks/repomock"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
	"go.uber.org/mock/gomock"
)

func testLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(os.Stdout, nil)) }

func newSvc(ctrl *gomock.Controller) (*MerchantService, *repomock.MockMerchantRepositoryInterface, *repomock.MockMenuRepositoryInterface) {
	mr := repomock.NewMockMerchantRepositoryInterface(ctrl)
	mu := repomock.NewMockMenuRepositoryInterface(ctrl)
	svc := NewMerchantService(mr, mu, sharedKafka.NewNoopProducer(), testLogger())
	return svc, mr, mu
}

func fakeMerchant(id, userID string, status model.MerchantStatus) *model.Merchant {
	return &model.Merchant{ID: id, UserID: userID, Name: "Warung Test", Status: status}
}

// -- RegisterMerchant ----------------------------------------------------------

func TestServiceRegisterMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().ExistsByUserID(gomock.Any(), "u-1").Return(false, nil)
	mr.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	m, err := svc.RegisterMerchant(context.Background(), "u-1", model.RegisterMerchantRequest{
		Name: "Warung", Category: model.CategoryFood, PhoneNumber: "081", Address: "Jl A", City: "Kota",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected merchant")
	}
}

func TestServiceRegisterMerchant_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().ExistsByUserID(gomock.Any(), "u-1").Return(true, nil)

	_, err := svc.RegisterMerchant(context.Background(), "u-1", model.RegisterMerchantRequest{})
	if err != ErrMerchantAlreadyExists {
		t.Errorf("expected ErrMerchantAlreadyExists, got %v", err)
	}
}

func TestServiceRegisterMerchant_CheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().ExistsByUserID(gomock.Any(), "u-1").Return(false, fmt.Errorf("db error"))

	_, err := svc.RegisterMerchant(context.Background(), "u-1", model.RegisterMerchantRequest{})
	if err == nil {
		t.Error("expected error")
	}
}

// -- GetMyMerchant -------------------------------------------------------------

func TestServiceGetMyMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByUserID(gomock.Any(), "u-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)

	m, err := svc.GetMyMerchant(context.Background(), "u-1")
	if err != nil || m == nil {
		t.Fatalf("err=%v m=%v", err, m)
	}
}

func TestServiceGetMyMerchant_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByUserID(gomock.Any(), "u-1").Return(nil, nil)

	_, err := svc.GetMyMerchant(context.Background(), "u-1")
	if err != ErrMerchantNotFound {
		t.Errorf("expected ErrMerchantNotFound, got %v", err)
	}
}

// -- GetMerchantByID -----------------------------------------------------------

func TestServiceGetMerchantByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)

	m, err := svc.GetMerchantByID(context.Background(), "m-1")
	if err != nil || m == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceGetMerchantByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-x").Return(nil, nil)

	_, err := svc.GetMerchantByID(context.Background(), "m-x")
	if err != ErrMerchantNotFound {
		t.Errorf("expected ErrMerchantNotFound, got %v", err)
	}
}

// -- UpdateMyMerchant ----------------------------------------------------------

func TestServiceUpdateMyMerchant_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	existing := fakeMerchant("m-1", "u-1", model.StatusActive)
	updated := fakeMerchant("m-1", "u-1", model.StatusActive)
	updated.Name = "New Name"

	mr.EXPECT().GetByUserID(gomock.Any(), "u-1").Return(existing, nil)
	mr.EXPECT().Update(gomock.Any(), "m-1", gomock.Any()).Return(updated, nil)

	name := "New Name"
	m, err := svc.UpdateMyMerchant(context.Background(), "u-1", model.UpdateMerchantRequest{Name: &name})
	if err != nil || m.Name != "New Name" {
		t.Fatalf("err=%v name=%v", err, m)
	}
}

func TestServiceUpdateMyMerchant_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByUserID(gomock.Any(), "u-x").Return(nil, nil)

	_, err := svc.UpdateMyMerchant(context.Background(), "u-x", model.UpdateMerchantRequest{})
	if err != ErrMerchantNotFound {
		t.Errorf("expected ErrMerchantNotFound, got %v", err)
	}
}

// -- UpdateMerchantStatus ------------------------------------------------------

func TestServiceUpdateMerchantStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	m := fakeMerchant("m-1", "u-1", model.StatusActive)
	updated := fakeMerchant("m-1", "u-1", model.StatusClosed)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(m, nil)
	mr.EXPECT().UpdateStatus(gomock.Any(), "m-1", model.StatusClosed).Return(updated, nil)

	result, err := svc.UpdateMerchantStatus(context.Background(), "m-1", "u-1", model.UpdateMerchantStatusRequest{Status: model.StatusClosed})
	if err != nil || result.Status != model.StatusClosed {
		t.Fatalf("err=%v status=%v", err, result)
	}
}

func TestServiceUpdateMerchantStatus_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner-1", model.StatusActive), nil)

	_, err := svc.UpdateMerchantStatus(context.Background(), "m-1", "other-user", model.UpdateMerchantStatusRequest{Status: model.StatusClosed})
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceUpdateMerchantStatus_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-x").Return(nil, nil)

	_, err := svc.UpdateMerchantStatus(context.Background(), "m-x", "u-1", model.UpdateMerchantStatusRequest{})
	if err != ErrMerchantNotFound {
		t.Errorf("expected ErrMerchantNotFound, got %v", err)
	}
}

// -- IsOpen --------------------------------------------------------------------

func TestServiceIsOpen_MerchantNotActive(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusClosed), nil)

	result, err := svc.IsOpen(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if result.IsOpen {
		t.Error("expected is_open=false for closed merchant")
	}
	if result.Reason != "merchant_not_active" {
		t.Errorf("expected reason merchant_not_active, got %s", result.Reason)
	}
}

func TestServiceIsOpen_NoSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().GetOperatingHours(gomock.Any(), "m-1").Return(nil, nil)

	result, err := svc.IsOpen(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !result.IsOpen {
		t.Error("expected is_open=true when no schedule set")
	}
}

func TestServiceIsOpen_DayClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	now := time.Now()
	dayOfWeek := int(now.Weekday())

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().GetOperatingHours(gomock.Any(), "m-1").Return([]model.OperatingHours{
		{DayOfWeek: dayOfWeek, IsClosed: true},
	}, nil)

	result, err := svc.IsOpen(context.Background(), "m-1")
	if err != nil || result.IsOpen {
		t.Fatalf("expected closed: err=%v isOpen=%v", err, result.IsOpen)
	}
	if result.Reason != "day_closed" {
		t.Errorf("expected reason day_closed, got %s", result.Reason)
	}
}

// -- Operating Hours -----------------------------------------------------------

func TestServiceGetOperatingHours_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	hours := []model.OperatingHours{{DayOfWeek: 1, OpenTime: "08:00", CloseTime: "22:00"}}
	mr.EXPECT().GetOperatingHours(gomock.Any(), "m-1").Return(hours, nil)

	result, err := svc.GetOperatingHours(context.Background(), "m-1")
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}

func TestServiceUpsertOperatingHours_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner", model.StatusActive), nil)

	_, err := svc.UpsertOperatingHours(context.Background(), "m-1", "hacker", model.UpsertOperatingHoursRequest{})
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceUpsertOperatingHours_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	req := model.UpsertOperatingHoursRequest{Hours: []model.UpsertOperatingHoursItem{{DayOfWeek: 1, OpenTime: "08:00", CloseTime: "22:00"}}}
	hours := []model.OperatingHours{{DayOfWeek: 1, OpenTime: "08:00", CloseTime: "22:00"}}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().UpsertOperatingHours(gomock.Any(), "m-1", req).Return(hours, nil)

	result, err := svc.UpsertOperatingHours(context.Background(), "m-1", "u-1", req)
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}

// -- Bank Accounts -------------------------------------------------------------

func TestServiceListBankAccounts_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().ListBankAccounts(gomock.Any(), "m-1").Return([]model.BankAccount{{ID: "ba-1"}}, nil)

	result, err := svc.ListBankAccounts(context.Background(), "m-1", "u-1")
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}

func TestServiceListBankAccounts_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner", model.StatusActive), nil)

	_, err := svc.ListBankAccounts(context.Background(), "m-1", "hacker")
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceAddBankAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	req := model.AddBankAccountRequest{BankCode: "BCA", AccountNumber: "123", AccountName: "Budi"}
	ba := &model.BankAccount{ID: "ba-1", BankCode: "BCA"}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().AddBankAccount(gomock.Any(), "m-1", req).Return(ba, nil)

	result, err := svc.AddBankAccount(context.Background(), "m-1", "u-1", req)
	if err != nil || result == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceDeleteBankAccount_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mr.EXPECT().DeleteBankAccount(gomock.Any(), "ba-1").Return(nil)

	err := svc.DeleteBankAccount(context.Background(), "m-1", "ba-1", "u-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceSetPrimaryBankAccount_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner", model.StatusActive), nil)

	_, err := svc.SetPrimaryBankAccount(context.Background(), "m-1", "ba-1", "hacker")
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// -- Menu Categories -----------------------------------------------------------

func TestServiceListCategories_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, mu := newSvc(ctrl)

	cats := []model.MenuCategory{{ID: "cat-1", Name: "Makanan Berat"}}
	mu.EXPECT().ListCategories(gomock.Any(), "m-1").Return(cats, nil)

	result, err := svc.ListCategories(context.Background(), "m-1")
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}

func TestServiceCreateCategory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	req := model.CreateMenuCategoryRequest{Name: "Minuman", DisplayOrder: 1}
	cat := &model.MenuCategory{ID: "cat-1", Name: "Minuman"}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().CreateCategory(gomock.Any(), "m-1", req).Return(cat, nil)

	result, err := svc.CreateCategory(context.Background(), "m-1", "u-1", req)
	if err != nil || result == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceCreateCategory_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner", model.StatusActive), nil)

	_, err := svc.CreateCategory(context.Background(), "m-1", "hacker", model.CreateMenuCategoryRequest{})
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceDeleteCategory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().DeleteCategory(gomock.Any(), "cat-1", "m-1").Return(nil)

	err := svc.DeleteCategory(context.Background(), "m-1", "cat-1", "u-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceReorderCategories_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	req := model.ReorderCategoriesRequest{Orders: []struct {
		CategoryID   string `json:"category_id"`
		DisplayOrder int    `json:"display_order"`
	}{{CategoryID: "cat-1", DisplayOrder: 0}}}
	cats := []model.MenuCategory{{ID: "cat-1"}}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().ReorderCategories(gomock.Any(), "m-1", req).Return(cats, nil)

	result, err := svc.ReorderCategories(context.Background(), "m-1", "u-1", req)
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v", err)
	}
}

// -- Menu Items ----------------------------------------------------------------

func TestServiceListItems_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, mu := newSvc(ctrl)

	items := []model.MenuItem{{ID: "item-1", Name: "Nasi Goreng", PriceCents: 25000}}
	mu.EXPECT().ListItems(gomock.Any(), "m-1", "", (*bool)(nil)).Return(items, nil)

	result, err := svc.ListItems(context.Background(), "m-1", "", nil)
	if err != nil || len(result) == 0 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}

func TestServiceCreateItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	req := model.CreateMenuItemRequest{CategoryID: "cat-1", Name: "Mie Goreng", PriceCents: 20000}
	item := &model.MenuItem{ID: "item-1", Name: "Mie Goreng", PriceCents: 20000, IsAvailable: true}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().CreateItem(gomock.Any(), "m-1", req).Return(item, nil)

	result, err := svc.CreateItem(context.Background(), "m-1", "u-1", req)
	if err != nil || result == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceCreateItem_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, _ := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "owner", model.StatusActive), nil)

	_, err := svc.CreateItem(context.Background(), "m-1", "hacker", model.CreateMenuItemRequest{})
	if err != ErrForbidden {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceGetItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, mu := newSvc(ctrl)

	item := &model.MenuItem{ID: "item-1", Name: "Sate Ayam"}
	mu.EXPECT().GetItemByID(gomock.Any(), "item-1", "m-1").Return(item, nil)

	result, err := svc.GetItem(context.Background(), "m-1", "item-1")
	if err != nil || result == nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceDeleteItem_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().DeleteItem(gomock.Any(), "item-1", "m-1").Return(nil)

	err := svc.DeleteItem(context.Background(), "m-1", "item-1", "u-1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestServiceToggleAvailability_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, mr, mu := newSvc(ctrl)

	item := &model.MenuItem{ID: "item-1", IsAvailable: false}

	mr.EXPECT().GetByID(gomock.Any(), "m-1").Return(fakeMerchant("m-1", "u-1", model.StatusActive), nil)
	mu.EXPECT().ToggleAvailability(gomock.Any(), "item-1", "m-1", false).Return(item, nil)

	result, err := svc.ToggleAvailability(context.Background(), "m-1", "item-1", "u-1", model.ToggleAvailabilityRequest{IsAvailable: false})
	if err != nil || result.IsAvailable {
		t.Fatalf("err=%v isAvail=%v", err, result.IsAvailable)
	}
}

func TestServiceBatchGetItems_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc, _, mu := newSvc(ctrl)

	items := []model.MenuItem{{ID: "item-1"}, {ID: "item-2"}}
	mu.EXPECT().BatchGetItems(gomock.Any(), []string{"item-1", "item-2"}).Return(items, nil)

	result, err := svc.BatchGetItems(context.Background(), model.BatchGetMenuItemsRequest{ItemIDs: []string{"item-1", "item-2"}})
	if err != nil || len(result) != 2 {
		t.Fatalf("err=%v len=%d", err, len(result))
	}
}
