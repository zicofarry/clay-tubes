module github.com/zicofarry/clay-app/backend/services/notification-service

go 1.25.0

replace github.com/zicofarry/clay-app/backend/pkg => ../../pkg

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/lib/pq v1.12.3
	github.com/zicofarry/clay-app/backend/pkg v0.0.0-00010101000000-000000000000
	go.uber.org/mock v0.6.0
)

require (
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver v1.17.9 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
