# clay-shared

Shared Go packages for all Clay microservices. This module provides common
types, middleware, helpers, and abstractions so that every service stays
consistent without duplicating code.

## Module Path

```
github.com/zicofarry/clay-shared
```

## Packages

| Package | Import | Description |
|---------|--------|-------------|
| `response` | `pkg/response` | Standard JSON response wrappers (`Success`, `Error`, `Paginated`, `Health`) |
| `middleware` | `pkg/middleware` | HTTP middleware: `AuthContext`, `RequestID`, `Logger`, `Recovery`, `CORS` |
| `kafka` | `pkg/kafka` | Kafka event envelope, Producer/Consumer interfaces |
| `database` | `pkg/database` | PostgreSQL pool + Redis config helpers, `WithTransaction` helper |
| `validator` | `pkg/validator` | JSON body decoding, query param parsing, pagination, validation errors |
| `idempotency` | `pkg/idempotency` | Redis-backed idempotency key checking for financial operations |

## Usage

In any Clay microservice's `go.mod`:

```go
require github.com/zicofarry/clay-shared v0.1.0
```

Then import what you need:

```go
import (
    "github.com/zicofarry/clay-shared/pkg/response"
    "github.com/zicofarry/clay-shared/pkg/middleware"
    "github.com/zicofarry/clay-shared/pkg/validator"
)

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
    userID := middleware.GetUserID(r.Context())
    page := validator.ParsePagination(r, 50)

    orders, total, err := svc.ListOrders(userID, page.Offset, page.Limit)
    if err != nil {
        response.Error(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
        return
    }

    response.Paginated(w, http.StatusOK, orders, total, page.Page, page.Limit)
}
```

## Running Tests

```bash
# Unit tests only (no DB/Redis required)
go test -tags=unit -v ./...
```

## Structure

```
clay-shared/
├── go.mod
├── go.sum
├── README.md
└── pkg/
    ├── response/
    │   ├── response.go
    │   └── response_test.go
    ├── middleware/
    │   ├── middleware.go
    │   └── middleware_test.go
    ├── kafka/
    │   ├── kafka.go
    │   └── kafka_test.go
    ├── database/
    │   ├── database.go
    │   └── database_test.go
    ├── validator/
    │   ├── validator.go
    │   └── validator_test.go
    └── idempotency/
        ├── idempotency.go
        └── idempotency_test.go
```
