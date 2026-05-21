module github.com/zicofarry/clay-app/backend/services/geo-service

go 1.25.0

replace github.com/zicofarry/clay-app/backend/pkg => ../../pkg

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/lib/pq v1.12.3
	github.com/zicofarry/clay-app/backend/pkg v0.0.0-00010101000000-000000000000
	go.uber.org/mock v0.6.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/segmentio/kafka-go v0.4.47 // indirect
)
