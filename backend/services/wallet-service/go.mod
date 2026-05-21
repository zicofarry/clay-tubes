module github.com/zicofarry/clay-app/backend/services/wallet-service

go 1.25.0

replace github.com/zicofarry/clay-app/backend/pkg => ../../pkg

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.12.3
	github.com/zicofarry/clay-app/backend/pkg v0.0.0-00010101000000-000000000000
	go.uber.org/mock v0.6.0
)
