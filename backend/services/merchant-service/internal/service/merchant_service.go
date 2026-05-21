package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/repository"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
)

const (
	TopicMerchantUpdated = "merchant.updated"
	TopicMenuUpdated     = "merchant.menu_updated"
	ServiceName          = "clay-merchant-service"
)

// MerchantServiceInterface defines all business logic operations for merchants and menus.
type MerchantServiceInterface interface {
	// Merchant
	RegisterMerchant(ctx context.Context, userID string, req model.RegisterMerchantRequest) (*model.Merchant, error)
	GetMyMerchant(ctx context.Context, userID string) (*model.Merchant, error)
	UpdateMyMerchant(ctx context.Context, userID string, req model.UpdateMerchantRequest) (*model.Merchant, error)
	GetMerchantByID(ctx context.Context, merchantID string) (*model.Merchant, error)
	UpdateMerchantStatus(ctx context.Context, merchantID, callerUserID string, req model.UpdateMerchantStatusRequest) (*model.Merchant, error)
	IsOpen(ctx context.Context, merchantID string) (*model.IsOpenResponse, error)
	// Operating Hours
	GetOperatingHours(ctx context.Context, merchantID string) ([]model.OperatingHours, error)
	UpsertOperatingHours(ctx context.Context, merchantID, callerUserID string, req model.UpsertOperatingHoursRequest) ([]model.OperatingHours, error)
	// Bank Accounts
	ListBankAccounts(ctx context.Context, merchantID, callerUserID string) ([]model.BankAccount, error)
	AddBankAccount(ctx context.Context, merchantID, callerUserID string, req model.AddBankAccountRequest) (*model.BankAccount, error)
	DeleteBankAccount(ctx context.Context, merchantID, accountID, callerUserID string) error
	SetPrimaryBankAccount(ctx context.Context, merchantID, accountID, callerUserID string) (*model.BankAccount, error)
	// Menu Categories
	ListCategories(ctx context.Context, merchantID string) ([]model.MenuCategory, error)
	CreateCategory(ctx context.Context, merchantID, callerUserID string, req model.CreateMenuCategoryRequest) (*model.MenuCategory, error)
	UpdateCategory(ctx context.Context, merchantID, categoryID, callerUserID string, req model.UpdateMenuCategoryRequest) (*model.MenuCategory, error)
	DeleteCategory(ctx context.Context, merchantID, categoryID, callerUserID string) error
	ReorderCategories(ctx context.Context, merchantID, callerUserID string, req model.ReorderCategoriesRequest) ([]model.MenuCategory, error)
	// Menu Items
	ListItems(ctx context.Context, merchantID, categoryID string, isAvailable *bool) ([]model.MenuItem, error)
	CreateItem(ctx context.Context, merchantID, callerUserID string, req model.CreateMenuItemRequest) (*model.MenuItem, error)
	GetItem(ctx context.Context, merchantID, itemID string) (*model.MenuItem, error)
	UpdateItem(ctx context.Context, merchantID, itemID, callerUserID string, req model.UpdateMenuItemRequest) (*model.MenuItem, error)
	DeleteItem(ctx context.Context, merchantID, itemID, callerUserID string) error
	ToggleAvailability(ctx context.Context, merchantID, itemID, callerUserID string, req model.ToggleAvailabilityRequest) (*model.MenuItem, error)
	BatchGetItems(ctx context.Context, req model.BatchGetMenuItemsRequest) ([]model.MenuItem, error)
}

// MerchantService encapsulates business logic for merchants and menus.
type MerchantService struct {
	merchantRepo repository.MerchantRepositoryInterface
	menuRepo     repository.MenuRepositoryInterface
	producer     sharedKafka.Producer
	logger       *slog.Logger
}

// NewMerchantService creates a new MerchantService.
func NewMerchantService(
	merchantRepo repository.MerchantRepositoryInterface,
	menuRepo repository.MenuRepositoryInterface,
	producer sharedKafka.Producer,
	logger *slog.Logger,
) *MerchantService {
	return &MerchantService{
		merchantRepo: merchantRepo,
		menuRepo:     menuRepo,
		producer:     producer,
		logger:       logger,
	}
}

// ── Merchant ──────────────────────────────────────────────────────────────────

func (s *MerchantService) RegisterMerchant(ctx context.Context, userID string, req model.RegisterMerchantRequest) (*model.Merchant, error) {
	exists, err := s.merchantRepo.ExistsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check existing: %w", err)
	}
	if exists {
		return nil, ErrMerchantAlreadyExists
	}

	m := &model.Merchant{
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		Category:       req.Category,
		PhoneNumber:    req.PhoneNumber,
		Email:          req.Email,
		Address:        req.Address,
		City:           req.City,
		Lat:            req.Lat,
		Lng:            req.Lng,
		MinOrderCents:  req.MinOrderCents,
		EstDeliveryMin: req.EstDeliveryMin,
	}

	if err := s.merchantRepo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create merchant: %w", err)
	}

	s.publishMerchantUpdated(ctx, m)
	return m, nil
}

func (s *MerchantService) GetMyMerchant(ctx context.Context, userID string) (*model.Merchant, error) {
	m, err := s.merchantRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	return m, nil
}

func (s *MerchantService) UpdateMyMerchant(ctx context.Context, userID string, req model.UpdateMerchantRequest) (*model.Merchant, error) {
	existing, err := s.merchantRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrMerchantNotFound
	}

	m, err := s.merchantRepo.Update(ctx, existing.ID, req)
	if err != nil {
		return nil, fmt.Errorf("update merchant: %w", err)
	}
	s.publishMerchantUpdated(ctx, m)
	return m, nil
}

func (s *MerchantService) GetMerchantByID(ctx context.Context, merchantID string) (*model.Merchant, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	return m, nil
}

func (s *MerchantService) UpdateMerchantStatus(ctx context.Context, merchantID, callerUserID string, req model.UpdateMerchantStatusRequest) (*model.Merchant, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return nil, ErrForbidden
	}

	updated, err := s.merchantRepo.UpdateStatus(ctx, merchantID, req.Status)
	if err != nil {
		return nil, err
	}
	s.publishMerchantUpdated(ctx, updated)
	return updated, nil
}

// IsOpen checks if the merchant is currently open based on operating hours.
func (s *MerchantService) IsOpen(ctx context.Context, merchantID string) (*model.IsOpenResponse, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.Status != model.StatusActive {
		return &model.IsOpenResponse{IsOpen: false, Reason: "merchant_not_active"}, nil
	}

	hours, err := s.merchantRepo.GetOperatingHours(ctx, merchantID)
	if err != nil || len(hours) == 0 {
		return &model.IsOpenResponse{IsOpen: true}, nil
	}

	now := time.Now()
	dayOfWeek := int(now.Weekday())
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	for _, h := range hours {
		if h.DayOfWeek != dayOfWeek {
			continue
		}
		if h.IsClosed {
			return &model.IsOpenResponse{IsOpen: false, Reason: "day_closed"}, nil
		}
		if currentTime >= h.OpenTime && currentTime < h.CloseTime {
			return &model.IsOpenResponse{IsOpen: true}, nil
		}
		return &model.IsOpenResponse{IsOpen: false, Reason: "outside_hours"}, nil
	}

	return &model.IsOpenResponse{IsOpen: false, Reason: "no_schedule"}, nil
}

// ── Operating Hours ───────────────────────────────────────────────────────────

func (s *MerchantService) GetOperatingHours(ctx context.Context, merchantID string) ([]model.OperatingHours, error) {
	return s.merchantRepo.GetOperatingHours(ctx, merchantID)
}

func (s *MerchantService) UpsertOperatingHours(ctx context.Context, merchantID, callerUserID string, req model.UpsertOperatingHoursRequest) ([]model.OperatingHours, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return nil, ErrForbidden
	}
	return s.merchantRepo.UpsertOperatingHours(ctx, merchantID, req)
}

// ── Bank Accounts ─────────────────────────────────────────────────────────────

func (s *MerchantService) ListBankAccounts(ctx context.Context, merchantID, callerUserID string) ([]model.BankAccount, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return nil, ErrForbidden
	}
	return s.merchantRepo.ListBankAccounts(ctx, merchantID)
}

func (s *MerchantService) AddBankAccount(ctx context.Context, merchantID, callerUserID string, req model.AddBankAccountRequest) (*model.BankAccount, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return nil, ErrForbidden
	}
	return s.merchantRepo.AddBankAccount(ctx, merchantID, req)
}

func (s *MerchantService) DeleteBankAccount(ctx context.Context, merchantID, accountID, callerUserID string) error {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return ErrForbidden
	}
	return s.merchantRepo.DeleteBankAccount(ctx, accountID)
}

func (s *MerchantService) SetPrimaryBankAccount(ctx context.Context, merchantID, accountID, callerUserID string) (*model.BankAccount, error) {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrMerchantNotFound
	}
	if m.UserID != callerUserID {
		return nil, ErrForbidden
	}
	return s.merchantRepo.SetPrimaryBankAccount(ctx, merchantID, accountID)
}

// ── Menu Categories ───────────────────────────────────────────────────────────

func (s *MerchantService) ListCategories(ctx context.Context, merchantID string) ([]model.MenuCategory, error) {
	return s.menuRepo.ListCategories(ctx, merchantID)
}

func (s *MerchantService) CreateCategory(ctx context.Context, merchantID, callerUserID string, req model.CreateMenuCategoryRequest) (*model.MenuCategory, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	return s.menuRepo.CreateCategory(ctx, merchantID, req)
}

func (s *MerchantService) UpdateCategory(ctx context.Context, merchantID, categoryID, callerUserID string, req model.UpdateMenuCategoryRequest) (*model.MenuCategory, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	return s.menuRepo.UpdateCategory(ctx, categoryID, merchantID, req)
}

func (s *MerchantService) DeleteCategory(ctx context.Context, merchantID, categoryID, callerUserID string) error {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return err
	}
	return s.menuRepo.DeleteCategory(ctx, categoryID, merchantID)
}

func (s *MerchantService) ReorderCategories(ctx context.Context, merchantID, callerUserID string, req model.ReorderCategoriesRequest) ([]model.MenuCategory, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	return s.menuRepo.ReorderCategories(ctx, merchantID, req)
}

// ── Menu Items ────────────────────────────────────────────────────────────────

func (s *MerchantService) ListItems(ctx context.Context, merchantID, categoryID string, isAvailable *bool) ([]model.MenuItem, error) {
	return s.menuRepo.ListItems(ctx, merchantID, categoryID, isAvailable)
}

func (s *MerchantService) CreateItem(ctx context.Context, merchantID, callerUserID string, req model.CreateMenuItemRequest) (*model.MenuItem, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	item, err := s.menuRepo.CreateItem(ctx, merchantID, req)
	if err != nil {
		return nil, err
	}
	s.publishMenuUpdated(ctx, merchantID, item.ID)
	return item, nil
}

func (s *MerchantService) GetItem(ctx context.Context, merchantID, itemID string) (*model.MenuItem, error) {
	return s.menuRepo.GetItemByID(ctx, itemID, merchantID)
}

func (s *MerchantService) UpdateItem(ctx context.Context, merchantID, itemID, callerUserID string, req model.UpdateMenuItemRequest) (*model.MenuItem, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	item, err := s.menuRepo.UpdateItem(ctx, itemID, merchantID, req)
	if err != nil {
		return nil, err
	}
	s.publishMenuUpdated(ctx, merchantID, itemID)
	return item, nil
}

func (s *MerchantService) DeleteItem(ctx context.Context, merchantID, itemID, callerUserID string) error {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return err
	}
	if err := s.menuRepo.DeleteItem(ctx, itemID, merchantID); err != nil {
		return err
	}
	s.publishMenuUpdated(ctx, merchantID, itemID)
	return nil
}

func (s *MerchantService) ToggleAvailability(ctx context.Context, merchantID, itemID, callerUserID string, req model.ToggleAvailabilityRequest) (*model.MenuItem, error) {
	if err := s.assertOwner(ctx, merchantID, callerUserID); err != nil {
		return nil, err
	}
	return s.menuRepo.ToggleAvailability(ctx, itemID, merchantID, req.IsAvailable)
}

func (s *MerchantService) BatchGetItems(ctx context.Context, req model.BatchGetMenuItemsRequest) ([]model.MenuItem, error) {
	return s.menuRepo.BatchGetItems(ctx, req.ItemIDs)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *MerchantService) assertOwner(ctx context.Context, merchantID, userID string) error {
	m, err := s.merchantRepo.GetByID(ctx, merchantID)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrMerchantNotFound
	}
	if m.UserID != userID {
		return ErrForbidden
	}
	return nil
}

func (s *MerchantService) publishMerchantUpdated(ctx context.Context, m *model.Merchant) {
	event, err := sharedKafka.NewEvent("merchant.updated", ServiceName, map[string]interface{}{
		"merchant_id": m.ID,
		"name":        m.Name,
		"status":      m.Status,
		"lat":         m.Lat,
		"lng":         m.Lng,
	})
	if err != nil {
		return
	}
	if err := s.producer.Publish(ctx, TopicMerchantUpdated, m.ID, event); err != nil {
		s.logger.Error("publish merchant.updated failed", slog.Any("error", err))
	}
}

func (s *MerchantService) publishMenuUpdated(ctx context.Context, merchantID, itemID string) {
	event, err := sharedKafka.NewEvent("merchant.menu_updated", ServiceName, map[string]interface{}{
		"merchant_id": merchantID,
		"item_id":     itemID,
	})
	if err != nil {
		return
	}
	if err := s.producer.Publish(ctx, TopicMenuUpdated, merchantID, event); err != nil {
		s.logger.Error("publish merchant.menu_updated failed", slog.Any("error", err))
	}
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrMerchantNotFound     = fmt.Errorf("merchant not found")
	ErrMerchantAlreadyExists = fmt.Errorf("merchant already registered for this user")
	ErrForbidden            = fmt.Errorf("forbidden")
	ErrItemNotFound         = fmt.Errorf("menu item not found")
	ErrCategoryNotFound     = fmt.Errorf("category not found")
)
