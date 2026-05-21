package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/repository"
)

//go:generate mockgen -source=user_service.go -destination=../../mocks/mock_user_service.go -package=mocks
type UserServiceInterface interface {
	// Profile
	GetProfile(ctx context.Context, userID uuid.UUID) (*models.ProfileResponse, error)
	CreateProfile(ctx context.Context, userID uuid.UUID, req models.CreateProfileRequest) (*models.ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req models.UpdateProfileRequest) (*models.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, targetUserID uuid.UUID) (*models.PublicProfileResponse, error)
	ApplyReferralCode(ctx context.Context, userID uuid.UUID, referralCode string) error

	// Address
	ListAddresses(ctx context.Context, userID uuid.UUID) ([]models.AddressResponse, error)
	CreateAddress(ctx context.Context, userID uuid.UUID, req models.AddressRequest) (*models.AddressResponse, error)
	UpdateAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID, req models.AddressRequest) (*models.AddressResponse, error)
	DeleteAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID) error
	SetDefaultAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID) error

	// Driver
	GetDriverProfile(ctx context.Context, userID uuid.UUID) (*models.DriverProfileResponse, error)
	CreateDriverProfile(ctx context.Context, userID uuid.UUID, req models.CreateDriverProfileRequest) (*models.DriverProfileResponse, error)
	UpdateDriverProfile(ctx context.Context, userID uuid.UUID, req models.UpdateDriverProfileRequest) (*models.DriverProfileResponse, error)
	GetPublicDriverProfile(ctx context.Context, driverID uuid.UUID) (*models.DriverPublicProfileResponse, error)
	ToggleDriverOnline(ctx context.Context, driverID uuid.UUID, isOnline bool) (*models.DriverProfileResponse, error)

	// Documents
	ListDriverDocuments(ctx context.Context, driverID uuid.UUID) ([]models.DocumentResponse, error)
	GetDocument(ctx context.Context, documentID uuid.UUID) (*models.DocumentResponse, error)
	DeleteDocument(ctx context.Context, driverID uuid.UUID, documentID uuid.UUID) error
	UploadDocument(ctx context.Context, driverID uuid.UUID, docType string, fileURL string) (*models.DocumentResponse, error)
	GetDriverVerificationStatus(ctx context.Context, driverID uuid.UUID) ([]models.DocumentResponse, error)
	VerifyDocument(ctx context.Context, documentID uuid.UUID, status string, rejectionReason string) (*models.DocumentResponse, error)

	// Settings
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.SettingsResponse, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, req models.UpdateSettingsRequest) (*models.SettingsResponse, error)

	// Internal
	LookupUserByPhone(ctx context.Context, phone string) (*models.LookupUserByPhoneResponse, error)
}

type UserService struct {
	repo   repository.UserRepositoryInterface
	logger *slog.Logger
}

func NewUserService(repo repository.UserRepositoryInterface, logger *slog.Logger) *UserService {
	return &UserService{repo: repo, logger: logger}
}

func generateReferralCode() string {
	return "CLAY-" + uuid.New().String()[:6]
}

// Map helper functions
func mapProfile(p *models.UserProfile) *models.ProfileResponse {
	if p == nil {
		return nil
	}
	resp := &models.ProfileResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		FullName:     p.FullName,
		AvatarURL:    p.AvatarURL,
		Gender:       p.Gender,
		ReferralCode: p.ReferralCode,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
	if p.BirthDate != nil {
		resp.BirthDate = *p.BirthDate
	}
	return resp
}

func mapAddress(a *models.UserAddress) models.AddressResponse {
	return models.AddressResponse{
		ID:        a.ID,
		Label:     a.Label,
		Address:   a.Address,
		Lat:       a.Lat,
		Lng:       a.Lng,
		Notes:     a.Notes,
		IsDefault: a.IsDefault,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

func mapDriverProfile(p *models.DriverProfile) *models.DriverProfileResponse {
	if p == nil {
		return nil
	}
	return &models.DriverProfileResponse{
		ID:                 p.ID,
		UserID:             p.UserID,
		VehicleType:        p.VehicleType,
		PlateNumber:        p.PlateNumber,
		VehicleBrand:       p.VehicleBrand,
		VehicleModel:       p.VehicleModel,
		VehicleYear:        p.VehicleYear,
		VehicleColor:       p.VehicleColor,
		SimNumber:          p.SimNumber,
		KtpNumber:          p.KtpNumber,
		VerificationStatus: p.VerificationStatus,
		RatingAvg:          p.RatingAvg,
		TotalTrips:         p.TotalTrips,
		IsOnline:           p.IsOnline,
		LastOnlineAt:       p.LastOnlineAt,
		CreatedAt:          p.CreatedAt,
	}
}

func mapDocument(d *models.DriverDocument) models.DocumentResponse {
	return models.DocumentResponse{
		ID:              d.ID,
		Type:            d.Type,
		FileURL:         d.FileURL,
		Status:          d.Status,
		RejectionReason: d.RejectionReason,
		VerifiedAt:      d.VerifiedAt,
		CreatedAt:       d.CreatedAt,
	}
}

// --- Profile Service ---

func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (*models.ProfileResponse, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("profile not found")
	}
	return mapProfile(p), nil
}

func (s *UserService) CreateProfile(ctx context.Context, userID uuid.UUID, req models.CreateProfileRequest) (*models.ProfileResponse, error) {
	existing, _ := s.repo.GetProfileByUserID(ctx, userID)
	if existing != nil {
		return nil, errors.New("profile already exists")
	}

	p := &models.UserProfile{
		ID:           uuid.New(),
		UserID:       userID,
		FullName:     req.FullName,
		Gender:       req.Gender,
		ReferralCode: generateReferralCode(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if req.BirthDate != "" {
		p.BirthDate = &req.BirthDate
	}

	err := s.repo.CreateProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	return mapProfile(p), nil
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req models.UpdateProfileRequest) (*models.ProfileResponse, error) {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("profile not found")
	}

	if req.FullName != "" {
		p.FullName = req.FullName
	}
	if req.BirthDate != "" {
		p.BirthDate = &req.BirthDate
	}
	if req.Gender != "" {
		p.Gender = req.Gender
	}
	p.UpdatedAt = time.Now()

	err = s.repo.UpdateProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	return mapProfile(p), nil
}

func (s *UserService) GetPublicProfile(ctx context.Context, targetUserID uuid.UUID) (*models.PublicProfileResponse, error) {
	p, err := s.repo.GetProfileByUserID(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("profile not found")
	}
	return &models.PublicProfileResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		FullName:  p.FullName,
		AvatarURL: p.AvatarURL,
	}, nil
}

func (s *UserService) ApplyReferralCode(ctx context.Context, userID uuid.UUID, referralCode string) error {
	p, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("profile not found")
	}
	if p.ReferredBy != nil {
		return errors.New("already referred")
	}
	return errors.New("not implemented: need GetProfileByReferralCode in repo")
}

// --- Address Service ---

func (s *UserService) ListAddresses(ctx context.Context, userID uuid.UUID) ([]models.AddressResponse, error) {
	addresses, err := s.repo.ListAddresses(ctx, userID)
	if err != nil {
		return nil, err
	}
	var res []models.AddressResponse
	for _, a := range addresses {
		res = append(res, mapAddress(&a))
	}
	return res, nil
}

func (s *UserService) CreateAddress(ctx context.Context, userID uuid.UUID, req models.AddressRequest) (*models.AddressResponse, error) {
	a := &models.UserAddress{
		ID:        uuid.New(),
		UserID:    userID,
		Label:     req.Label,
		Address:   req.Address,
		Lat:       req.Lat,
		Lng:       req.Lng,
		Notes:     req.Notes,
		IsDefault: req.IsDefault,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := s.repo.CreateAddress(ctx, a)
	if err != nil {
		return nil, err
	}
	if req.IsDefault {
		_ = s.repo.SetDefaultAddress(ctx, a.ID, userID)
	}

	mapped := mapAddress(a)
	return &mapped, nil
}

func (s *UserService) UpdateAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID, req models.AddressRequest) (*models.AddressResponse, error) {
	a, err := s.repo.GetAddress(ctx, addressID)
	if err != nil {
		return nil, err
	}
	if a == nil || a.UserID != userID {
		return nil, errors.New("address not found")
	}

	a.Label = req.Label
	a.Address = req.Address
	a.Lat = req.Lat
	a.Lng = req.Lng
	a.Notes = req.Notes
	a.IsDefault = req.IsDefault
	a.UpdatedAt = time.Now()

	err = s.repo.UpdateAddress(ctx, a)
	if err != nil {
		return nil, err
	}
	if a.IsDefault {
		_ = s.repo.SetDefaultAddress(ctx, a.ID, userID)
	}

	mapped := mapAddress(a)
	return &mapped, nil
}

func (s *UserService) DeleteAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID) error {
	return s.repo.DeleteAddress(ctx, addressID, userID)
}

func (s *UserService) SetDefaultAddress(ctx context.Context, userID uuid.UUID, addressID uuid.UUID) error {
	return s.repo.SetDefaultAddress(ctx, addressID, userID)
}

// --- Driver Service ---

func (s *UserService) GetDriverProfile(ctx context.Context, userID uuid.UUID) (*models.DriverProfileResponse, error) {
	p, err := s.repo.GetDriverProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("driver profile not found")
	}
	return mapDriverProfile(p), nil
}

func (s *UserService) CreateDriverProfile(ctx context.Context, userID uuid.UUID, req models.CreateDriverProfileRequest) (*models.DriverProfileResponse, error) {
	existing, _ := s.repo.GetDriverProfileByUserID(ctx, userID)
	if existing != nil {
		return nil, errors.New("driver profile already exists")
	}

	p := &models.DriverProfile{
		ID:                 uuid.New(),
		UserID:             userID,
		VehicleType:        req.VehicleType,
		PlateNumber:        req.PlateNumber,
		VehicleBrand:       req.VehicleBrand,
		VehicleModel:       req.VehicleModel,
		VehicleYear:        req.VehicleYear,
		VehicleColor:       req.VehicleColor,
		SimNumber:          req.SimNumber,
		KtpNumber:          req.KtpNumber,
		VerificationStatus: "pending",
		RatingAvg:          5.0,
		TotalTrips:         0,
		IsOnline:           false,
		CreatedAt:          time.Now(),
	}

	err := s.repo.CreateDriverProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	return mapDriverProfile(p), nil
}

func (s *UserService) UpdateDriverProfile(ctx context.Context, userID uuid.UUID, req models.UpdateDriverProfileRequest) (*models.DriverProfileResponse, error) {
	p, err := s.repo.GetDriverProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("driver profile not found")
	}

	if req.VehicleType != "" {
		p.VehicleType = req.VehicleType
	}
	if req.PlateNumber != "" {
		p.PlateNumber = req.PlateNumber
	}
	if req.VehicleBrand != "" {
		p.VehicleBrand = req.VehicleBrand
	}
	if req.VehicleModel != "" {
		p.VehicleModel = req.VehicleModel
	}
	if req.VehicleYear != 0 {
		p.VehicleYear = req.VehicleYear
	}
	if req.VehicleColor != "" {
		p.VehicleColor = req.VehicleColor
	}

	err = s.repo.UpdateDriverProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	return mapDriverProfile(p), nil
}

func (s *UserService) GetPublicDriverProfile(ctx context.Context, driverID uuid.UUID) (*models.DriverPublicProfileResponse, error) {
	p, err := s.repo.GetDriverProfileByID(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("driver profile not found")
	}
	return &models.DriverPublicProfileResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		VehicleType:  p.VehicleType,
		PlateNumber:  p.PlateNumber,
		VehicleBrand: p.VehicleBrand,
		VehicleColor: p.VehicleColor,
		RatingAvg:    p.RatingAvg,
	}, nil
}

func (s *UserService) ToggleDriverOnline(ctx context.Context, driverID uuid.UUID, isOnline bool) (*models.DriverProfileResponse, error) {
	p, err := s.repo.GetDriverProfileByID(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("driver profile not found")
	}
	if p.VerificationStatus != "verified" && isOnline {
		return nil, errors.New("driver is not verified")
	}

	err = s.repo.ToggleDriverOnline(ctx, driverID, isOnline)
	if err != nil {
		return nil, err
	}
	
	now := time.Now()
	p.IsOnline = isOnline
	p.LastOnlineAt = &now

	return mapDriverProfile(p), nil
}

// --- Document Service ---

func (s *UserService) ListDriverDocuments(ctx context.Context, driverID uuid.UUID) ([]models.DocumentResponse, error) {
	docs, err := s.repo.ListDriverDocuments(ctx, driverID)
	if err != nil {
		return nil, err
	}
	var res []models.DocumentResponse
	for _, d := range docs {
		res = append(res, mapDocument(&d))
	}
	return res, nil
}

func (s *UserService) GetDocument(ctx context.Context, documentID uuid.UUID) (*models.DocumentResponse, error) {
	d, err := s.repo.GetDriverDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, errors.New("document not found")
	}
	mapped := mapDocument(d)
	return &mapped, nil
}

func (s *UserService) DeleteDocument(ctx context.Context, driverID uuid.UUID, documentID uuid.UUID) error {
	return s.repo.DeleteDriverDocument(ctx, documentID, driverID)
}

func (s *UserService) UploadDocument(ctx context.Context, driverID uuid.UUID, docType string, fileURL string) (*models.DocumentResponse, error) {
	d := &models.DriverDocument{
		ID:        uuid.New(),
		DriverID:  driverID,
		Type:      docType,
		FileURL:   fileURL,
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := s.repo.CreateDriverDocument(ctx, d)
	if err != nil {
		return nil, err
	}
	mapped := mapDocument(d)
	return &mapped, nil
}

func (s *UserService) GetDriverVerificationStatus(ctx context.Context, driverID uuid.UUID) ([]models.DocumentResponse, error) {
	return s.ListDriverDocuments(ctx, driverID)
}

func (s *UserService) VerifyDocument(ctx context.Context, documentID uuid.UUID, status string, rejectionReason string) (*models.DocumentResponse, error) {
	if status != "approved" && status != "rejected" {
		return nil, errors.New("invalid status")
	}
	if status == "rejected" && rejectionReason == "" {
		return nil, errors.New("rejection reason is required")
	}

	err := s.repo.UpdateDriverDocumentStatus(ctx, documentID, status, rejectionReason)
	if err != nil {
		return nil, err
	}

	d, _ := s.repo.GetDriverDocument(ctx, documentID)
	mapped := mapDocument(d)
	return &mapped, nil
}

// --- Settings Service ---

func (s *UserService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.SettingsResponse, error) {
	st, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &models.SettingsResponse{
		Language:         st.Language,
		NotifEnabled:     st.NotifEnabled,
		MarketingEnabled: st.MarketingEnabled,
	}, nil
}

func (s *UserService) UpdateSettings(ctx context.Context, userID uuid.UUID, req models.UpdateSettingsRequest) (*models.SettingsResponse, error) {
	st, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	if req.Language != "" {
		st.Language = req.Language
	}
	if req.NotifEnabled != nil {
		st.NotifEnabled = *req.NotifEnabled
	}
	if req.MarketingEnabled != nil {
		st.MarketingEnabled = *req.MarketingEnabled
	}

	err = s.repo.UpdateSettings(ctx, st)
	if err != nil {
		return nil, err
	}
	return &models.SettingsResponse{
		Language:         st.Language,
		NotifEnabled:     st.NotifEnabled,
		MarketingEnabled: st.MarketingEnabled,
	}, nil
}

// --- Internal ---

func (s *UserService) LookupUserByPhone(ctx context.Context, phone string) (*models.LookupUserByPhoneResponse, error) {
	p, err := s.repo.LookupUserByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &models.LookupUserByPhoneResponse{Found: false}, nil
	}
	return &models.LookupUserByPhoneResponse{
		Found:    true,
		UserID:   &p.UserID,
		FullName: p.FullName,
	}, nil
}
