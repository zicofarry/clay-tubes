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

	// 1. Cek Koneksi ke Elasticsearch (dengan retry karena Elasticsearch butuh beberapa detik untuk boot up)
	t.Run("Elasticsearch Connection", func(t *testing.T) {
		client := &http.Client{Timeout: 2 * time.Second}
		var lastErr error
		var lastStatus int
		success := false

		maxAttempts := 30
		for i := 0; i < maxAttempts; i++ {
			resp, err := client.Get("http://localhost:9200")
			if err == nil {
				lastStatus = resp.StatusCode
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					success = true
					break
				}
			} else {
				lastErr = err
			}
			t.Logf("Waiting for Elasticsearch to be ready... (attempt %d/%d, error: %v)", i+1, maxAttempts, err)
			time.Sleep(2 * time.Second)
		}

		if !success {
			if lastErr != nil {
				t.Fatalf("Elasticsearch is NOT running or not ready. Error: %v", lastErr)
			} else {
				t.Fatalf("Elasticsearch returned status %d", lastStatus)
			}
		}
		t.Log("Successfully connected to Elasticsearch!")
	})

	// 2. Cek Koneksi ke Redis (dengan retry karena database docker butuh waktu singkat untuk init)
	t.Run("Redis Connection", func(t *testing.T) {
		rdb := redis.NewClient(&redis.Options{
			Addr: "localhost:6389", // Sesuai dengan docker-compose.yml port 6389
		})

		var lastErr error
		success := false
		maxAttempts := 10
		for i := 0; i < maxAttempts; i++ {
			err := rdb.Ping(ctx).Err()
			if err == nil {
				success = true
				break
			}
			lastErr = err
			t.Logf("Waiting for Redis to be ready... (attempt %d/%d, error: %v)", i+1, maxAttempts, err)
			time.Sleep(1 * time.Second)
		}

		if !success {
			t.Fatalf("Redis is NOT running on port 6389. Please run 'docker compose up -d'. Error: %v", lastErr)
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
