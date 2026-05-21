# clay-ride-order-service

Manages the full lifecycle of ride orders (GoRide / GoCar) on the Clay platform.

## Stack
- **Language:** Go 1.25
- **HTTP:** stdlib `net/http` + `clay-shared` middleware (RequestID, Logger, Recovery, CORS)
- **DB:** PostgreSQL (`ride_orders`, `order_state_logs`, `trip_details`, `order_fare_breakdown`)
- **Cache:** Redis (hot order state, anti-double-booking lock)
- **Tests:** `go-sqlmock` + `go.uber.org/mock` for unit tests, real Postgres for functional tests

## Layout
```
.
├── main.go                              # service entrypoint
├── internal/
│   ├── handler/                         # HTTP layer (1:1 with openapi)
│   ├── service/                         # business logic + DTOs
│   └── repository/                      # PostgreSQL data access
├── mocks/                               # gomock-generated mocks
│   ├── mock_ride_order_service.go       #   service interface mock
│   └── repomock/                        #   repository interface mock
├── test/functional/                     # E2E DB integration tests
├── scripts/init.sql                     # auto-applied schema (docker-compose)
├── docker-compose.yml                   # postgres + redis + adminer
├── Dockerfile
├── Jenkinsfile                          # CI pipeline
└── openapi.yaml
```

## Running locally
```bash
docker compose up -d
go run ./main.go
```
- Service: `http://localhost:3003`
- Adminer: `http://localhost:9003` (server: `postgres-ride-order`, user: `clay_user`, pass: `clay_password`, db: `ride_order_db`)

## Tests

### Unit tests (gomock + sqlmock — no DB required)
```bash
go test -tags=unit -v -race ./...
```

### Functional tests (require docker compose)
```bash
docker compose up -d
go test -tags=functional -v ./test/functional/...
```
The functional tests connect directly to the PostgreSQL container exposed on
`localhost:5433`. They will **fail** when the stack is not running — once you
`docker compose up -d`, they pass.

## Regenerating mocks
```bash
go install go.uber.org/mock/mockgen@latest
go generate ./...
```
