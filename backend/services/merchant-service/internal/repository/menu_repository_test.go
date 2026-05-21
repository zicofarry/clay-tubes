//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// -- ListCategories ------------------------------------------------------------

func TestMenuRepo_ListCategories_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("list active categories", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		first := mtest.CreateCursorResponse(1, "test.menu_categories", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "cat-1"},
				{Key: "merchant_id", Value: "m-1"},
				{Key: "name", Value: "Makanan Berat"},
				{Key: "display_order", Value: 0},
				{Key: "is_active", Value: true},
			})
		killCursors := mtest.CreateCursorResponse(0, "test.menu_categories", mtest.NextBatch)
		mt.AddMockResponses(first, killCursors)

		cats, err := repo.ListCategories(context.Background(), "m-1")
		if err != nil {
			mt.Fatalf("ListCategories: %v", err)
		}
		if len(cats) != 1 {
			mt.Errorf("expected 1, got %d", len(cats))
		}
		if cats[0].Name != "Makanan Berat" {
			mt.Errorf("name: %s", cats[0].Name)
		}
	})
}

func TestMenuRepo_ListCategories_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("empty result", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		first := mtest.CreateCursorResponse(0, "test.menu_categories", mtest.FirstBatch)
		mt.AddMockResponses(first)

		cats, err := repo.ListCategories(context.Background(), "m-empty")
		if err != nil {
			mt.Fatalf("ListCategories: %v", err)
		}
		if len(cats) != 0 {
			mt.Errorf("expected 0, got %d", len(cats))
		}
	})
}

// -- GetItemByID ---------------------------------------------------------------

func TestMenuRepo_GetItemByID_Found(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("item found", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "test.menu_items", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "item-1"},
				{Key: "merchant_id", Value: "m-1"},
				{Key: "category_id", Value: "cat-1"},
				{Key: "name", Value: "Nasi Goreng"},
				{Key: "price_cents", Value: int64(25000)},
				{Key: "is_available", Value: true},
			},
		))

		item, err := repo.GetItemByID(context.Background(), "item-1", "m-1")
		if err != nil || item == nil {
			mt.Fatalf("GetItemByID: err=%v item=%v", err, item)
		}
		if item.Name != "Nasi Goreng" {
			mt.Errorf("name: %s", item.Name)
		}
		if item.PriceCents != 25000 {
			mt.Errorf("price: %d", item.PriceCents)
		}
	})
}

func TestMenuRepo_GetItemByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("item not found", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.menu_items", mtest.FirstBatch))

		item, err := repo.GetItemByID(context.Background(), "item-x", "m-1")
		if err != nil {
			mt.Fatalf("expected nil err, got %v", err)
		}
		if item != nil {
			mt.Errorf("expected nil item")
		}
	})
}

// -- ListItems -----------------------------------------------------------------

func TestMenuRepo_ListItems_AllItems(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("all items for merchant", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		first := mtest.CreateCursorResponse(1, "test.menu_items", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "item-1"},
				{Key: "merchant_id", Value: "m-1"},
				{Key: "name", Value: "Nasi Goreng"},
				{Key: "price_cents", Value: int64(25000)},
				{Key: "is_available", Value: true},
			})
		killCursors := mtest.CreateCursorResponse(0, "test.menu_items", mtest.NextBatch)
		mt.AddMockResponses(first, killCursors)

		items, err := repo.ListItems(context.Background(), "m-1", "", nil)
		if err != nil {
			mt.Fatalf("ListItems: %v", err)
		}
		if len(items) != 1 {
			mt.Errorf("expected 1, got %d", len(items))
		}
	})
}

func TestMenuRepo_ListItems_FilterByCategory(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("filtered by category", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		first := mtest.CreateCursorResponse(0, "test.menu_items", mtest.FirstBatch)
		mt.AddMockResponses(first)

		avail := true
		items, err := repo.ListItems(context.Background(), "m-1", "cat-99", &avail)
		if err != nil {
			mt.Fatalf("ListItems: %v", err)
		}
		if len(items) != 0 {
			mt.Errorf("expected 0, got %d", len(items))
		}
	})
}

// -- DeleteItem ----------------------------------------------------------------

func TestMenuRepo_DeleteItem_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("soft delete item", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: 1},
			{Key: "nModified", Value: 1},
		})

		err := repo.DeleteItem(context.Background(), "item-1", "m-1")
		if err != nil {
			mt.Fatalf("DeleteItem: %v", err)
		}
	})
}

// -- DeleteCategory ------------------------------------------------------------

func TestMenuRepo_DeleteCategory_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("soft delete category", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: 1},
			{Key: "nModified", Value: 1},
		})

		err := repo.DeleteCategory(context.Background(), "cat-1", "m-1")
		if err != nil {
			mt.Fatalf("DeleteCategory: %v", err)
		}
	})
}

// -- BatchGetItems -------------------------------------------------------------

func TestMenuRepo_BatchGetItems_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("batch get 2 items", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		first := mtest.CreateCursorResponse(1, "test.menu_items", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "item-1"},
				{Key: "merchant_id", Value: "m-1"},
				{Key: "name", Value: "Nasi Goreng"},
				{Key: "price_cents", Value: int64(25000)},
				{Key: "is_available", Value: true},
			},
			bson.D{
				{Key: "_id", Value: "item-2"},
				{Key: "merchant_id", Value: "m-1"},
				{Key: "name", Value: "Mie Goreng"},
				{Key: "price_cents", Value: int64(20000)},
				{Key: "is_available", Value: true},
			},
		)
		killCursors := mtest.CreateCursorResponse(0, "test.menu_items", mtest.NextBatch)
		mt.AddMockResponses(first, killCursors)

		items, err := repo.BatchGetItems(context.Background(), []string{"item-1", "item-2"})
		if err != nil {
			mt.Fatalf("BatchGetItems: %v", err)
		}
		if len(items) != 2 {
			mt.Errorf("expected 2, got %d", len(items))
		}
	})
}

// -- CreateCategory ------------------------------------------------------------

func TestMenuRepo_CreateCategory_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("create category", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		cat, err := repo.CreateCategory(context.Background(), "m-1", model.CreateMenuCategoryRequest{
			Name:         "Makanan Berat",
			DisplayOrder: 0,
		})
		if err != nil || cat == nil {
			mt.Fatalf("CreateCategory: err=%v", err)
		}
		if cat.Name != "Makanan Berat" {
			mt.Errorf("name: %s", cat.Name)
		}
		if cat.MerchantID != "m-1" {
			mt.Errorf("merchant_id: %s", cat.MerchantID)
		}
		if !cat.IsActive {
			mt.Error("expected is_active=true")
		}
	})
}

// -- CreateItem ----------------------------------------------------------------

func TestMenuRepo_CreateItem_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("create item", func(mt *mtest.T) {
		repo := NewMenuRepository(mt.DB)

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		item, err := repo.CreateItem(context.Background(), "m-1", model.CreateMenuItemRequest{
			CategoryID: "cat-1",
			Name:       "Nasi Goreng Special",
			PriceCents: 30000,
			Tags:       []string{"rice", "spicy"},
		})
		if err != nil || item == nil {
			mt.Fatalf("CreateItem: err=%v", err)
		}
		if item.Name != "Nasi Goreng Special" {
			mt.Errorf("name: %s", item.Name)
		}
		if item.PriceCents != 30000 {
			mt.Errorf("price: %d", item.PriceCents)
		}
		if !item.IsAvailable {
			mt.Error("expected is_available=true")
		}
		if len(item.Variants) != 0 {
			mt.Errorf("expected empty variants, got %d", len(item.Variants))
		}
	})
}
