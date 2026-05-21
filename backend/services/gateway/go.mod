module github.com/zicofarry/clay-app/backend/services/gateway

go 1.23.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/redis/go-redis/v9 v9.18.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/zicofarry/clay-app/backend/pkg => ../../pkg
