package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MenuRepositoryInterface defines persistence operations for menu catalog.
type MenuRepositoryInterface interface {
	ListCategories(ctx context.Context, merchantID string) ([]model.MenuCategory, error)
	CreateCategory(ctx context.Context, merchantID string, req model.CreateMenuCategoryRequest) (*model.MenuCategory, error)
	UpdateCategory(ctx context.Context, categoryID, merchantID string, req model.UpdateMenuCategoryRequest) (*model.MenuCategory, error)
	DeleteCategory(ctx context.Context, categoryID, merchantID string) error
	ReorderCategories(ctx context.Context, merchantID string, req model.ReorderCategoriesRequest) ([]model.MenuCategory, error)
	ListItems(ctx context.Context, merchantID string, categoryID string, isAvailable *bool) ([]model.MenuItem, error)
	CreateItem(ctx context.Context, merchantID string, req model.CreateMenuItemRequest) (*model.MenuItem, error)
	GetItemByID(ctx context.Context, itemID, merchantID string) (*model.MenuItem, error)
	UpdateItem(ctx context.Context, itemID, merchantID string, req model.UpdateMenuItemRequest) (*model.MenuItem, error)
	DeleteItem(ctx context.Context, itemID, merchantID string) error
	ToggleAvailability(ctx context.Context, itemID, merchantID string, available bool) (*model.MenuItem, error)
	BatchGetItems(ctx context.Context, itemIDs []string) ([]model.MenuItem, error)
}

// MenuRepository handles MongoDB persistence for menu categories and items.
type MenuRepository struct {
	db *mongo.Database
}

// NewMenuRepository creates a new MenuRepository.
func NewMenuRepository(db *mongo.Database) *MenuRepository {
	return &MenuRepository{db: db}
}

func (r *MenuRepository) categories() *mongo.Collection {
	return r.db.Collection("menu_categories")
}

func (r *MenuRepository) items() *mongo.Collection {
	return r.db.Collection("menu_items")
}

// ── Categories ────────────────────────────────────────────────────────────────

func (r *MenuRepository) ListCategories(ctx context.Context, merchantID string) ([]model.MenuCategory, error) {
	cursor, err := r.categories().Find(ctx,
		bson.M{"merchant_id": merchantID, "is_active": true},
		options.Find().SetSort(bson.D{{Key: "display_order", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var cats []model.MenuCategory
	if err := cursor.All(ctx, &cats); err != nil {
		return nil, err
	}
	return cats, nil
}

func (r *MenuRepository) CreateCategory(ctx context.Context, merchantID string, req model.CreateMenuCategoryRequest) (*model.MenuCategory, error) {
	cat := &model.MenuCategory{
		ID:           uuid.New().String(),
		MerchantID:   merchantID,
		Name:         req.Name,
		Description:  req.Description,
		DisplayOrder: req.DisplayOrder,
		IsActive:     true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_, err := r.categories().InsertOne(ctx, cat)
	return cat, err
}

func (r *MenuRepository) UpdateCategory(ctx context.Context, categoryID, merchantID string, req model.UpdateMenuCategoryRequest) (*model.MenuCategory, error) {
	update := bson.M{"updated_at": time.Now().UTC()}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.DisplayOrder != nil {
		update["display_order"] = *req.DisplayOrder
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var cat model.MenuCategory
	err := r.categories().FindOneAndUpdate(ctx,
		bson.M{"_id": categoryID, "merchant_id": merchantID},
		bson.M{"$set": update},
		opts,
	).Decode(&cat)
	return &cat, err
}

func (r *MenuRepository) DeleteCategory(ctx context.Context, categoryID, merchantID string) error {
	_, err := r.categories().UpdateOne(ctx,
		bson.M{"_id": categoryID, "merchant_id": merchantID},
		bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now().UTC()}},
	)
	return err
}

func (r *MenuRepository) ReorderCategories(ctx context.Context, merchantID string, req model.ReorderCategoriesRequest) ([]model.MenuCategory, error) {
	for _, o := range req.Orders {
		_, _ = r.categories().UpdateOne(ctx,
			bson.M{"_id": o.CategoryID, "merchant_id": merchantID},
			bson.M{"$set": bson.M{"display_order": o.DisplayOrder, "updated_at": time.Now().UTC()}},
		)
	}
	return r.ListCategories(ctx, merchantID)
}

// ── Menu Items ────────────────────────────────────────────────────────────────

func (r *MenuRepository) ListItems(ctx context.Context, merchantID string, categoryID string, isAvailable *bool) ([]model.MenuItem, error) {
	filter := bson.M{"merchant_id": merchantID}
	if categoryID != "" {
		filter["category_id"] = categoryID
	}
	if isAvailable != nil {
		filter["is_available"] = *isAvailable
	}

	cursor, err := r.items().Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "name", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []model.MenuItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *MenuRepository) CreateItem(ctx context.Context, merchantID string, req model.CreateMenuItemRequest) (*model.MenuItem, error) {
	item := &model.MenuItem{
		ID:          uuid.New().String(),
		MerchantID:  merchantID,
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  req.PriceCents,
		ImageURL:    req.ImageURL,
		IsAvailable: true,
		Variants:    req.Variants,
		AddOns:      req.AddOns,
		Tags:        req.Tags,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if item.Variants == nil {
		item.Variants = []model.MenuItemVariant{}
	}
	if item.AddOns == nil {
		item.AddOns = []model.MenuItemAddOn{}
	}
	_, err := r.items().InsertOne(ctx, item)
	return item, err
}

func (r *MenuRepository) GetItemByID(ctx context.Context, itemID, merchantID string) (*model.MenuItem, error) {
	var item model.MenuItem
	filter := bson.M{"_id": itemID}
	if merchantID != "" {
		filter["merchant_id"] = merchantID
	}
	err := r.items().FindOne(ctx, filter).Decode(&item)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &item, err
}

func (r *MenuRepository) UpdateItem(ctx context.Context, itemID, merchantID string, req model.UpdateMenuItemRequest) (*model.MenuItem, error) {
	update := bson.M{"updated_at": time.Now().UTC()}
	if req.CategoryID != nil {
		update["category_id"] = *req.CategoryID
	}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.PriceCents != nil {
		update["price_cents"] = *req.PriceCents
	}
	if req.ImageURL != nil {
		update["image_url"] = *req.ImageURL
	}
	if req.Variants != nil {
		update["variants"] = req.Variants
	}
	if req.AddOns != nil {
		update["add_ons"] = req.AddOns
	}
	if req.Tags != nil {
		update["tags"] = req.Tags
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var item model.MenuItem
	err := r.items().FindOneAndUpdate(ctx,
		bson.M{"_id": itemID, "merchant_id": merchantID},
		bson.M{"$set": update},
		opts,
	).Decode(&item)
	return &item, err
}

func (r *MenuRepository) DeleteItem(ctx context.Context, itemID, merchantID string) error {
	_, err := r.items().UpdateOne(ctx,
		bson.M{"_id": itemID, "merchant_id": merchantID},
		bson.M{"$set": bson.M{"is_available": false, "updated_at": time.Now().UTC()}},
	)
	return err
}

func (r *MenuRepository) ToggleAvailability(ctx context.Context, itemID, merchantID string, available bool) (*model.MenuItem, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var item model.MenuItem
	err := r.items().FindOneAndUpdate(ctx,
		bson.M{"_id": itemID, "merchant_id": merchantID},
		bson.M{"$set": bson.M{"is_available": available, "updated_at": time.Now().UTC()}},
		opts,
	).Decode(&item)
	return &item, err
}

func (r *MenuRepository) BatchGetItems(ctx context.Context, itemIDs []string) ([]model.MenuItem, error) {
	cursor, err := r.items().Find(ctx, bson.M{"_id": bson.M{"$in": itemIDs}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []model.MenuItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}
