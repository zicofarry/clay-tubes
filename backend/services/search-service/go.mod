module github.com/zicofarry/clay-app/backend/services/search-service

go 1.25.0

replace github.com/zicofarry/clay-app/backend/pkg => ../../pkg

// require

require (
	github.com/elastic/go-elasticsearch/v8 v8.19.4
	github.com/redis/go-redis/v9 v9.18.0
	github.com/zicofarry/clay-app/backend/pkg v0.0.0-00010101000000-000000000000
	go.uber.org/mock v0.6.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/elastic/elastic-transport-go/v8 v8.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/otel v1.29.0 // indirect
	go.opentelemetry.io/otel/metric v1.29.0 // indirect
	go.opentelemetry.io/otel/trace v1.29.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)
