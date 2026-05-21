//go:build unit

package service

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/ride-order-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

// helper to build a service with a gomock'd repo
func newTestService(t *testing.T) (*RideOrderService, *repomock.MockRideOrderRepositoryInterface, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := repomock.NewMockRideOrderRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewRideOrderService(repo, logger), repo, ctrl
}

func validCreateReq() *CreateRideOrderRequest {
	return &CreateRideOrderRequest{
		ServiceType:   "goride",
		VehicleType:   "motor",
		OriginLat:     -6.914744,
		OriginLng:     107.609810,
		DestLat:       -6.921000,
		DestLng:       107.607000,
		PaymentMethod: "gopay",
		FareEstimate:  18000,
	}
}

// ── EstimateFare ────────────────────────────────────────────────────────────

func TestEstimateFare_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	resp, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		OriginLat: -6.914744, OriginLng: 107.609810,
		DestLat: -6.921000, DestLng: 107.607000,
		VehicleType: "motor",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.VehicleType != "motor" {
		t.Errorf("want motor, got %s", resp.VehicleType)
	}
	if resp.Breakdown.Total <= 0 {
		t.Errorf("want positive total, got %.2f", resp.Breakdown.Total)
	}
}

func TestEstimateFare_BadVehicleType(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		OriginLat: 0, OriginLng: 0, DestLat: 0, DestLng: 0,
		VehicleType: "bicycle",
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestEstimateFare_BadLatLng(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.EstimateFare(context.Background(), &FareEstimateRequest{
		OriginLat: 200, // out of range
		OriginLng: 0, DestLat: 0, DestLng: 0,
		VehicleType: "motor",
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

// ── CreateOrder ─────────────────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetActiveOrderByUserID(gomock.Any(), "user-1").
		Return(nil, sql.ErrNoRows)

	repo.EXPECT().CreateOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, o *repository.RideOrder) (*repository.RideOrder, error) {
			o.ID = "order-1"
			o.Status = "finding_driver"
			return o, nil
		})

	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).
		Return(nil)

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
		Return(&repository.RideOrder{ID: "active-1"}, nil)

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

func TestCreateOrder_BadServiceType(t *testing.T) {
	svc, _, _ := newTestService(t)
	r := validCreateReq()
	r.ServiceType = "go-flight"
	_, err := svc.CreateOrder(context.Background(), "user-1", r)
	if err == nil {
		t.Fatal("want validation error, got nil")
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
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "finding_driver"}, nil)

	resp, err := svc.GetActiveOrder(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.ID != "order-1" {
		t.Errorf("want order-1, got %s", resp.ID)
	}
}

// ── GetOrder authorization ──────────────────────────────────────────────────

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
		Return(&repository.RideOrder{ID: "order-1", UserID: "owner"}, nil)

	_, err := svc.GetOrder(context.Background(), "stranger", "user", "order-1")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestGetOrder_DriverAccessAllowed(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID:       "order-1",
			UserID:   "owner",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
			Status:   "assigned",
			OTPCode:  sql.NullString{String: "847291", Valid: true},
		}, nil)

	repo.EXPECT().GetTripDetails(gomock.Any(), "order-1").
		Return(nil, sql.ErrNoRows)
	repo.EXPECT().ListStateLogs(gomock.Any(), "order-1").
		Return(nil, nil)

	resp, err := svc.GetOrder(context.Background(), "driver-1", "driver", "order-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.OTPCode != "847291" {
		t.Errorf("driver should see otp; got %q", resp.OTPCode)
	}
}

// ── CancelOrder ─────────────────────────────────────────────────────────────

func TestCancelOrder_OnTripBlocked(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "on_trip"}, nil)

	_, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{Reason: "x"})
	if err != ErrCannotCancelOnTrip {
		t.Errorf("want ErrCannotCancelOnTrip, got %v", err)
	}
}

func TestCancelOrder_NonCancellableState(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "completed"}, nil)

	_, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{})
	if err != ErrInvalidStateTransition {
		t.Errorf("want ErrInvalidStateTransition, got %v", err)
	}
}

func TestCancelOrder_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "owner", Status: "assigned"}, nil)

	_, err := svc.CancelOrder(context.Background(), "stranger", "order-1", &CancelOrderRequest{})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestCancelOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "finding_driver"}, nil)

	repo.EXPECT().SetCancelled(gomock.Any(), "order-1", "Salah pilih", "user").
		Return(nil)

	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).
		Return(nil)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "cancelled"}, nil)

	resp, err := svc.CancelOrder(context.Background(), "user-1", "order-1", &CancelOrderRequest{Reason: "Salah pilih"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "cancelled" {
		t.Errorf("want cancelled, got %s", resp.Status)
	}
}

// ── DriverAcceptOrder ───────────────────────────────────────────────────────

func TestDriverAcceptOrder_AlreadyTaken(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", Status: "assigned"}, nil)

	_, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != ErrOrderAlreadyTaken {
		t.Errorf("want ErrOrderAlreadyTaken, got %v", err)
	}
}

func TestDriverAcceptOrder_RaceCondition(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", Status: "finding_driver"}, nil)
	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1", gomock.Any()).
		Return(sql.ErrNoRows)

	_, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != ErrOrderAlreadyTaken {
		t.Errorf("want ErrOrderAlreadyTaken on race, got %v", err)
	}
}

func TestDriverAcceptOrder_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID:            "order-1",
			UserID:        "user-1",
			Status:        "finding_driver",
			OriginLat:     1.0, OriginLng: 2.0,
			PaymentMethod: "gopay",
		}, nil)

	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1", gomock.Any()).
		Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID:            "order-1",
			UserID:        "user-1",
			DriverID:      sql.NullString{String: "driver-1", Valid: true},
			OTPCode:       sql.NullString{String: "111111", Valid: true},
			Status:        "assigned",
			OriginLat:     1.0, OriginLng: 2.0,
			PaymentMethod: "gopay",
		}, nil)

	resp, err := svc.DriverAcceptOrder(context.Background(), "driver-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "assigned" {
		t.Errorf("want assigned, got %s", resp.Status)
	}
	if len(resp.OTPCode) != 6 {
		t.Errorf("want 6-digit OTP, got %q", resp.OTPCode)
	}
}

// ── DriverUpdateOrderStatus ─────────────────────────────────────────────────

func TestDriverUpdateOrderStatus_UnknownAction(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action: "fly",
	})
	if err == nil {
		t.Fatal("want validation err, got nil")
	}
}

func TestDriverUpdateOrderStatus_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "assigned",
			DriverID: sql.NullString{String: "other-driver", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action: "arrived_at_pickup",
	})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_StartTrip_BadOTP(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "on_pickup",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
			OTPCode:  sql.NullString{String: "111111", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action:  "start_trip",
		OTPCode: "999999",
	})
	if err != ErrInvalidOTP {
		t.Errorf("want ErrInvalidOTP, got %v", err)
	}
}

func TestDriverUpdateOrderStatus_CompleteTrip_MissingFields(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "on_trip", VehicleType: "motor",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	_, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action: "complete_trip",
	})
	if err == nil {
		t.Fatal("want validation err, got nil")
	}
}

func TestDriverUpdateOrderStatus_ArrivedAtPickup_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "assigned",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	repo.EXPECT().UpdateStatus(gomock.Any(), "order-1", "assigned", "on_pickup").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "on_pickup",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	resp, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action: "arrived_at_pickup",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "on_pickup" {
		t.Errorf("want on_pickup, got %s", resp.Status)
	}
}

func TestDriverUpdateOrderStatus_CompleteTrip_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "on_trip", VehicleType: "motor",
			DriverID:     sql.NullString{String: "driver-1", Valid: true},
			FareEstimate: sql.NullFloat64{Float64: 18000, Valid: true},
		}, nil)

	repo.EXPECT().UpdateStatus(gomock.Any(), "order-1", "on_trip", "completed").Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().UpsertTripDetails(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().UpsertFareBreakdown(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().SetFareFinal(gomock.Any(), "order-1", gomock.Any()).Return(nil)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{
			ID: "order-1", Status: "completed",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
		}, nil)

	resp, err := svc.DriverUpdateOrderStatus(context.Background(), "driver-1", "order-1", &DriverUpdateStatusRequest{
		Action:            "complete_trip",
		ActualDistanceKm:  3.2,
		ActualDurationMin: 18,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("want completed, got %s", resp.Status)
	}
}

// ── SubmitRating ────────────────────────────────────────────────────────────

func TestSubmitRating_BadScore(t *testing.T) {
	svc, _, _ := newTestService(t)
	err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 0})
	if err == nil {
		t.Fatal("want validation err, got nil")
	}
}

func TestSubmitRating_NotCompleted(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "on_trip"}, nil)

	err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 5})
	if err != ErrOrderNotCompleted {
		t.Errorf("want ErrOrderNotCompleted, got %v", err)
	}
}

func TestSubmitRating_Forbidden(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "owner", Status: "completed"}, nil)

	err := svc.SubmitRating(context.Background(), "stranger", "order-1", &SubmitRatingRequest{Score: 5})
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestSubmitRating_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "completed"}, nil)

	if err := svc.SubmitRating(context.Background(), "user-1", "order-1", &SubmitRatingRequest{Score: 5, Comment: "👍"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ── GetFareBreakdown ────────────────────────────────────────────────────────

func TestGetFareBreakdown_NotFinalized(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "completed"}, nil)
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
		Return(&repository.RideOrder{
			ID:       "order-1",
			UserID:   "owner",
			DriverID: sql.NullString{String: "driver-1", Valid: true},
			Status:   "completed",
		}, nil)

	_, err := svc.GetFareBreakdown(context.Background(), "stranger", "order-1")
	if err != ErrForbidden {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestGetFareBreakdown_Success(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().GetOrderByID(gomock.Any(), "order-1").
		Return(&repository.RideOrder{ID: "order-1", UserID: "user-1", Status: "completed"}, nil)

	repo.EXPECT().GetFareBreakdown(gomock.Any(), "order-1").
		Return(&repository.FareBreakdown{Total: 15000.0}, nil)

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
		Return(&repository.RideOrder{ID: "order-1", Status: "finding_driver"}, nil)

	repo.EXPECT().AssignDriver(gomock.Any(), "order-1", "driver-1", gomock.Any()).Return(nil)
	repo.EXPECT().InsertStateLog(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.InternalAssignDriver(context.Background(), "order-1", &InternalAssignDriverRequest{
		DriverID: "driver-1", ETASeconds: 180,
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if resp.Status != "assigned" {
		t.Errorf("want assigned, got %s", resp.Status)
	}
	if len(resp.OTPCode) != 6 {
		t.Errorf("want 6-digit OTP, got %q", resp.OTPCode)
	}
}

// ── GetOrderHistory ─────────────────────────────────────────────────────────

func TestGetOrderHistory_DefaultPaging(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().ListUserHistory(gomock.Any(), "user-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, f repository.HistoryFilter) ([]*repository.RideOrder, int, error) {
			if f.Limit != 10 || f.Offset != 0 {
				t.Errorf("expected default Limit=10 Offset=0, got %+v", f)
			}
			return []*repository.RideOrder{}, 0, nil
		})

	_, err := svc.GetOrderHistory(context.Background(), "user-1", HistoryQuery{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestGetOrderHistory_LimitClamped(t *testing.T) {
	svc, repo, _ := newTestService(t)

	repo.EXPECT().ListUserHistory(gomock.Any(), "user-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, f repository.HistoryFilter) ([]*repository.RideOrder, int, error) {
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
