//go:build unit

package service

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/delivery-order-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/delivery-order-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*DeliveryOrderService, *repomock.MockDeliveryOrderRepositoryInterface, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := repomock.NewMockDeliveryOrderRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewDeliveryOrderService(repo, logger), repo, ctrl
}

func validCreateReq() *CreateDeliveryOrderRequest {
	return &CreateDeliveryOrderRequest{
		SenderName:     "Budi Santoso",
		SenderPhone:    "+6281234567890",
		PickupLat:      -6.914744,
		PickupLng:      107.609810,
		PickupAddress:  "Jl. Braga No.1, Bandung",
		RecipientName:  "Siti Rahayu",
		RecipientPhone: "+6289876543210",
		DestLat:        -6.921000,
		DestLng:        107.607000,
		DestAddress:    "Jl. Dago No.5, Bandung",
		PaymentMethod:  "gopay",
		Package: PackageInput{
			Category: "document",
			Size:     "small",
		},
		FareEstimate: 15000,
	}
}

func validRepoOrder() *repository.DeliveryOrder {
	return &repository.DeliveryOrder{
		ID:             "order-1",
		UserID:         "user-1",
		Status:         "finding_driver",
		SenderName:     "Budi Santoso",
		SenderPhone:    "+6281234567890",
		PickupLat:      -6.914744,
		PickupLng:      107.609810,
		PickupAddress:  "Jl. Braga No.1, Bandung",
		RecipientName:  "Siti Rahayu",
		RecipientPhone: "+6289876543210",
		DestLat:        -6.921000,
		DestLng:        107.607000,
		DestAddress:    "Jl. Dago No.5, Bandung",
		PaymentMethod:  "gopay",
		FareEstimate:   sql.NullFloat64{Float64: 15000, Valid: true},
	}
}

// ── EstimateFare ────────────────────────────────────────────────────────────

func TestEstimateFare_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	resp, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		PickupLat: -6.914744, PickupLng: 107.609810,
		DestLat: -6.921000, DestLng: 107.607000,
		Package: PackageInput{Category: "document", Size: "small"},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.DistanceKm <= 0 {
		t.Errorf("want positive distance, got %.2f", resp.DistanceKm)
	}
	if resp.Breakdown.Total <= 0 {
		t.Errorf("want positive total, got %.2f", resp.Breakdown.Total)
	}
}

func TestEstimateFare_WithInsurance(t *testing.T) {
	svc, _, _ := newTestService(t)

	resp, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		PickupLat: -6.914744, PickupLng: 107.609810,
		DestLat: -6.921000, DestLng: 107.607000,
		Package: PackageInput{
			Category:       "electronics",
			Size:           "medium",
			WeightKg:       2.5,
			InsuranceValue: 500000,
		},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// With 2.5kg weight, should have weight surcharge
	if resp.Breakdown.WeightSurcharge <= 0 {
		t.Errorf("want weight surcharge > 0, got %.2f", resp.Breakdown.WeightSurcharge)
	}
	if resp.Breakdown.InsuranceFee <= 0 {
		t.Errorf("want insurance fee > 0, got %.2f", resp.Breakdown.InsuranceFee)
	}
}

func TestEstimateFare_BadLatLng(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		PickupLat: 200, // out of range
		Package:   PackageInput{Category: "document", Size: "small"},
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestEstimateFare_BadPackageCategory(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		PickupLat: -6.91, PickupLng: 107.6, DestLat: -6.92, DestLng: 107.61,
		Package: PackageInput{Category: "rocket", Size: "small"},
	})
	if err == nil {
		t.Fatal("want validation error, got nil")
	}
}

func TestEstimateFare_WithPromo(t *testing.T) {
	svc, _, _ := newTestService(t)

	resp, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		PickupLat: -6.914744, PickupLng: 107.609810,
		DestLat: -6.921000, DestLng: 107.607000,
		Package: PackageInput{Category: "document", Size: "small"},
		PromoID: "promo-123",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.PromoDiscount <= 0 {
		t.Errorf("want promo discount > 0, got %.2f", resp.PromoDiscount)
	}
	if resp.FareAfterPromo >= resp.FareEstimate {
		t.Error("fare after promo should be less than original estimate")
	}
}

// ── CreateOrder ─────────────────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetActiveOrderByUserID(gomock.Any(), "user-1").
		Return(nil, sql.ErrNoRows)

	repo.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, o *repository.DeliveryOrder, pkg *repository.DeliveryPackage) (*repository.DeliveryOrder, error) {
			o.ID = "order-1"
			o.Status = "finding_driver"
			return o, nil
		})

	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.CreateOrder(context.Background(), "user-1", validCreateReq())
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.ID != "order-1" {
		t.Errorf("want order-1, got %s", resp.ID)
	}
	if resp.Status != "finding_driver" {
		t.Errorf("want finding_driver, got %s", resp.Status)
	}
}

func TestCreateOrder_DoubleBooking(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetActiveOrderByUserID(gomock.Any(), "user-1").
		Return(&repository.DeliveryOrder{ID: "active-1"}, nil)

	_, err := svc.CreateOrder(context.Background(), "user-1", validCreateReq())
	if err != ErrActiveOrderExists {
		t.Errorf("want ErrActiveOrderExists, got %v", err)
	}
}

func TestCreateOrder_NoUser(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateOrder(context.Background(), "", validCreateReq())
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestCreateOrder_BadPaymentMethod(t *testing.T) {
	svc, _, _ := newTestService(t)
	r := validCreateReq()
	r.PaymentMethod = "bitcoin"
	_, err := svc.CreateOrder(context.Background(), "user-1", r)
	if err == nil {
		t.Fatal("want validation error, got nil")
	}
}

func TestCreateOrder_MissingSenderName(t *testing.T) {
	svc, _, _ := newTestService(t)
	r := validCreateReq()
	r.SenderName = ""
	_, err := svc.CreateOrder(context.Background(), "user-1", r)
	if err == nil {
		t.Fatal("want validation error for missing sender_name, got nil")
	}
}

func TestCreateOrder_InvalidPackageSize(t *testing.T) {
	svc, _, _ := newTestService(t)
	r := validCreateReq()
	r.Package.Size = "jumbo"
	_, err := svc.CreateOrder(context.Background(), "user-1", r)
	if err == nil {
		t.Fatal("want validation error for invalid package size, got nil")
	}
}

// ── GetActiveOrder ──────────────────────────────────────────────────────────

func TestGetActiveOrder_None(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetActiveOrderByUserID(gomock.Any(), "user-1").
		Return(nil, sql.ErrNoRows)

	_, err := svc.GetActiveOrder(context.Background(), "user-1")
	if err != ErrNoActiveOrder {
		t.Errorf("want ErrNoActiveOrder, got %v", err)
	}
}

func TestGetActiveOrder_Found(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetActiveOrderByUserID(gomock.Any(), "user-1").
		Return(validRepoOrder(), nil)

	resp, err := svc.GetActiveOrder(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.ID != "order-1" {
		t.Errorf("want order-1, got %s", resp.ID)
	}
}

func TestGetActiveOrder_NoUserID(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetActiveOrder(context.Background(), "")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

// ── GetOrder ────────────────────────────────────────────────────────────────

func TestGetOrder_NotFound(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "x").
		Return(nil, sql.ErrNoRows)

	_, err := svc.GetOrder(context.Background(), "user-1", "user", "x")
	if err != ErrOrderNotFound {
		t.Errorf("want ErrOrderNotFound, got %v", err)
	}
}

func TestGetOrder_ForbiddenForOtherUser(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "owner"}, nil)

	_, err := svc.GetOrder(context.Background(), "stranger", "user", "order-1")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestGetOrder_DriverAccessAllowed(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			UserID:   "owner",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
			Status:   "assigned",
		}, nil)

	repo.EXPECT().GetPackageByOrderID(gomock.Any(), "order-1").
		Return(&repository.DeliveryPackage{ID: "pkg-1", Category: "document", Size: "small"}, nil)
	repo.EXPECT().ListStateLogs(gomock.Any(), "order-1").
		Return(nil, nil)

	resp, err := svc.GetOrder(context.Background(), "driver-1", "driver", "order-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "assigned" {
		t.Errorf("want assigned, got %s", resp.Status)
	}
	if resp.Package == nil {
		t.Error("want package in detail response")
	}
}

func TestGetOrder_DriverForbiddenIfNotAssigned(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			UserID:   "owner",
			DriverID: sql.NullString{String: "other-driver", Valid: true},
			Status:   "assigned",
		}, nil)

	_, err := svc.GetOrder(context.Background(), "driver-1", "driver", "order-1")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

// ── CancelOrder ─────────────────────────────────────────────────────────────

func TestCancelOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "finding_driver"}, nil)

	repo.EXPECT().SetCancelled(gomock.Any(), "order-1", "Alamat salah", "user").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:     "order-1",
			UserID: "user-1",
			Status: "cancelled",
			CancelReason: sql.NullString{String: "Alamat salah", Valid: true},
			CancelledBy:  sql.NullString{String: "user", Valid: true},
		}, nil)

	resp, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{Reason: "Alamat salah"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "cancelled" {
		t.Errorf("want cancelled, got %s", resp.Status)
	}
}

func TestCancelOrder_PickedUpBlocked(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "picked_up"}, nil)

	_, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{})
	if err != ErrCannotCancelPickedUp {
		t.Errorf("want ErrCannotCancelPickedUp, got %v", err)
	}
}

func TestCancelOrder_OnDeliveryBlocked(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "on_delivery"}, nil)

	_, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{})
	if err != ErrCannotCancelPickedUp {
		t.Errorf("want ErrCannotCancelPickedUp, got %v", err)
	}
}

func TestCancelOrder_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "owner", Status: "assigned"}, nil)

	_, err := svc.CancelOrder(context.Background(), "stranger", "order-1", &CancelOrderRequest{})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestCancelOrder_DeliveredNonCancellable(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "delivered"}, nil)

	_, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{})
	if err != ErrInvalidStateTransition {
		t.Errorf("want ErrInvalidStateTransition, got %v", err)
	}
}

// ── DriverAcceptOrder ───────────────────────────────────────────────────────

func TestDriverAcceptOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	o := validRepoOrder()
	o.Status = "finding_driver"
	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").Return(o, nil)
	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().GetPackageByOrderID(gomock.Any(), "order-1").
		Return(&repository.DeliveryPackage{ID: "pkg-1", Category: "document", Size: "small"}, nil)

	resp, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "assigned" {
		t.Errorf("want assigned, got %s", resp.Status)
	}
	if resp.Package == nil {
		t.Error("want package in accept response")
	}
}

func TestDriverAcceptOrder_AlreadyTaken(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", Status: "assigned"}, nil)

	_, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != ErrOrderAlreadyTaken {
		t.Errorf("want ErrOrderAlreadyTaken, got %v", err)
	}
}

func TestDriverAcceptOrder_RaceCondition(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", Status: "finding_driver"}, nil)
	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1").Return(sql.ErrNoRows)

	_, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != ErrOrderAlreadyTaken {
		t.Errorf("want ErrOrderAlreadyTaken on race, got %v", err)
	}
}

// ── DriverUpdateOrderStatus ─────────────────────────────────────────────────

func TestDriverUpdateOrderStatus_UnknownAction(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "teleport"})
	if err == nil {
		t.Fatal("want error for unknown action, got nil")
	}
}

func TestDriverUpdateOrderStatus_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "assigned",
			DriverID: sql.NullString{String: "other-driver", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "arrived_at_pickup"})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_PickedUp_MissingPhoto(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "on_pickup",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "picked_up"})
	if err != ErrPickupPhotoRequired {
		t.Errorf("want ErrPickupPhotoRequired, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_CompleteDelivery_MissingFields(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "on_delivery",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "complete_delivery"})
	if err != ErrDeliveryFieldsRequired {
		t.Errorf("want ErrDeliveryFieldsRequired, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_CompleteDelivery_MissingPhoto(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "on_delivery",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{
			Action:            "complete_delivery",
			ActualDistanceKm:  4.1,
			ActualDurationMin: 22,
		})
	if err != ErrDeliveryPhotoRequired {
		t.Errorf("want ErrDeliveryPhotoRequired, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_ArrivedAtPickup_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "assigned",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), "order-1", "assigned", "on_pickup").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "on_pickup",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	resp, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "arrived_at_pickup"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "on_pickup" {
		t.Errorf("want on_pickup, got %s", resp.Status)
	}
}

func TestDriverUpdateOrderStatus_PickedUp_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "on_pickup",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), "order-1", "on_pickup", "picked_up").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().SetPickupProof(gomock.Any(), "order-1", "https://cdn.clay.id/pickup.jpg").Return(nil)
	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:             "order-1",
			Status:         "picked_up",
			DriverID:       sql.NullString{String: "driver-1", Valid: true},
			PickupPhotoURL: sql.NullString{String: "https://cdn.clay.id/pickup.jpg", Valid: true},
		}, nil)

	resp, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{Action: "picked_up", PickupPhotoURL: "https://cdn.clay.id/pickup.jpg"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "picked_up" {
		t.Errorf("want picked_up, got %s", resp.Status)
	}
}

func TestDriverUpdateOrderStatus_CompleteDelivery_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:           "order-1",
			Status:       "on_delivery",
			DriverID:     sql.NullString{String: "driver-1", Valid: true},
			FareEstimate: sql.NullFloat64{Float64: 15000, Valid: true},
		}, nil)
	repo.EXPECT().UpdateStatus(gomock.Any(), "order-1", "on_delivery", "delivered").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().SetDeliveryDetails(gomock.Any(), "order-1", "https://cdn.clay.id/delivery.jpg", 4.1, 22).Return(nil)
	repo.EXPECT().GetPackageByOrderID(gomock.Any(), "order-1").
		Return(&repository.DeliveryPackage{Category: "document", Size: "small"}, nil)
	repo.EXPECT().UpsertFareBreakdown(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().SetFareFinal(gomock.Any(), "order-1", gomock.Any()).Return(nil)
	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			Status:   "delivered",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	resp, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1",
		&DriverUpdateStatusRequest{
			Action:            "complete_delivery",
			ActualDistanceKm:  4.1,
			ActualDurationMin: 22,
			DeliveryPhotoURL:  "https://cdn.clay.id/delivery.jpg",
		})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "delivered" {
		t.Errorf("want delivered, got %s", resp.Status)
	}
}

// ── SubmitRating ────────────────────────────────────────────────────────────

func TestSubmitRating_BadScore(t *testing.T) {
	svc, _, _ := newTestService(t)
	err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 0})
	if err == nil {
		t.Fatal("want validation error, got nil")
	}
}

func TestSubmitRating_ScoreTooHigh(t *testing.T) {
	svc, _, _ := newTestService(t)
	err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 6})
	if err == nil {
		t.Fatal("want validation error, got nil")
	}
}

func TestSubmitRating_NotDelivered(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "on_delivery"}, nil)

	err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 5})
	if err != ErrOrderNotDelivered {
		t.Errorf("want ErrOrderNotDelivered, got %v", err)
	}
}

func TestSubmitRating_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "owner", Status: "delivered"}, nil)

	err := svc.SubmitRating(context.Background(), "stranger", "order-1", &SubmitRatingRequest{Score: 5})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestSubmitRating_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "delivered"}, nil)

	if err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 5, Comment: "Sangat bagus!"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ── GetFareBreakdown ────────────────────────────────────────────────────────

func TestGetFareBreakdown_NotFinalized(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "delivered"}, nil)
	repo.EXPECT().GetFareBreakdown(gomock.Any(), "order-1").
		Return(nil, sql.ErrNoRows)

	_, err := svc.GetFareBreakdown(context.Background(), "user-1", "order-1")
	if err != ErrFareNotFinalized {
		t.Errorf("want ErrFareNotFinalized, got %v", err)
	}
}

func TestGetFareBreakdown_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{
			ID:       "order-1",
			UserID:   "owner",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
			Status:   "delivered",
		}, nil)

	_, err := svc.GetFareBreakdown(context.Background(), "stranger", "order-1")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestGetFareBreakdown_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", UserID: "user-1", Status: "delivered"}, nil)
	repo.EXPECT().GetFareBreakdown(gomock.Any(), "order-1").
		Return(&repository.DeliveryFareBreakdown{
			BaseFare: 5000, DistanceFare: 9000, WeightSurcharge: 0,
			InsuranceFee: 0, PromoDiscount: 0, PlatformFee: 1000, Total: 15000,
		}, nil)

	resp, err := svc.GetFareBreakdown(context.Background(), "user-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Total != 15000.0 {
		t.Errorf("want 15000, got %.2f", resp.Total)
	}
}

// ── InternalAssignDriver ────────────────────────────────────────────────────

func TestInternalAssignDriver_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", Status: "finding_driver"}, nil)
	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.InternalAssignDriver(context.Background(), "order-1", &InternalAssignDriverRequest{
		DriverID: "driver-1", ETASeconds: 240,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "assigned" {
		t.Errorf("want assigned, got %s", resp.Status)
	}
	if resp.ETASeconds != 240 {
		t.Errorf("want eta=240, got %d", resp.ETASeconds)
	}
}

func TestInternalAssignDriver_AlreadyTaken(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", Status: "assigned"}, nil)

	_, err := svc.InternalAssignDriver(context.Background(), "order-1", &InternalAssignDriverRequest{
		DriverID: "driver-1", ETASeconds: 240,
	})
	if err != ErrOrderAlreadyTaken {
		t.Errorf("want ErrOrderAlreadyTaken, got %v", err)
	}
}

// ── GetOrderHistory ─────────────────────────────────────────────────────────

func TestGetOrderHistory_DefaultPaging(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().ListUserHistory(gomock.Any(), "user-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, f repository.HistoryFilter) ([]*repository.DeliveryOrder, int, error) {
			if f.Limit != 10 || f.Offset != 0 {
				t.Errorf("expected Limit=10 Offset=0, got %+v", f)
			}
			return []*repository.DeliveryOrder{}, 0, nil
		})

	_, err := svc.GetOrderHistory(context.Background(), "user-1", HistoryQuery{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestGetOrderHistory_LimitClamped(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().ListUserHistory(gomock.Any(), "user-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, f repository.HistoryFilter) ([]*repository.DeliveryOrder, int, error) {
			if f.Limit != 10 {
				t.Errorf("expected limit clamped to 10, got %d", f.Limit)
			}
			return nil, 0, nil
		})

	_, err := svc.GetOrderHistory(context.Background(), "user-1", HistoryQuery{Limit: 9999})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestGetOrderHistory_NoUserID(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetOrderHistory(context.Background(), "", HistoryQuery{})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

// ── InternalUpdateStatus ────────────────────────────────────────────────────

func TestInternalUpdateStatus_TerminalStateBlocked(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.DeliveryOrder{ID: "order-1", Status: "delivered"}, nil)

	_, err := svc.InternalUpdateStatus(context.Background(), "order-1", &InternalUpdateStatusRequest{
		Status:    "cancelled",
		ActorType: "system",
	})
	if err == nil {
		t.Fatal("want error for terminal state, got nil")
	}
}
