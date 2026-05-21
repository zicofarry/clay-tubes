//go:build functional

package functional

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/search-service/internal/repository"
)

func TestSearchIntegration_Dependencies(t *testing.T) {
	t.Log("Starting functional test for Search Service Dependencies...")

	ctx := context.Background()

	// 1. Cek Koneksi ke Elasticsearch
	t.Run("Elasticsearch Connection", func(t *testing.T) {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://localhost:9200")
		if err != nil {
			t.Fatalf("Elasticsearch is NOT running. Please run 'docker-compose up -d'. Error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Elasticsearch returned status %d", resp.StatusCode)
		}
		t.Log("Successfully connected to Elasticsearch!")
	})

	// 2. Cek Koneksi ke Redis
	t.Run("Redis Connection", func(t *testing.T) {
		rdb := redis.NewClient(&redis.Options{
			Addr: "localhost:6389", // Sesuai dengan docker-compose.yml port 6389
		})

		err := rdb.Ping(ctx).Err()
		if err != nil {
			t.Fatalf("Redis is NOT running on port 6389. Please run 'docker-compose up -d'. Error: %v", err)
		}
		t.Log("Successfully connected to Redis!")
	})

	// 3. Test Repository sungguhan (bukan mock)
	t.Run("Repository Ping", func(t *testing.T) {
		es, _ := elasticsearch.NewDefaultClient()
		repo := repository.NewSearchRepository(es)
		err := repo.Ping(ctx)
		if err != nil {
			t.Errorf("Repository Ping failed: %v", err)
		}
	})

	// Kedepannya, kamu bisa menambah test untuk indexing document ke Elasticsearch di sini.
}
