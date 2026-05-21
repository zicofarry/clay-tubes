package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoConfig holds MongoDB connection settings.
type MongoConfig struct {
	URI      string `json:"uri" env:"MONGO_URI"`
	Database string `json:"database" env:"MONGO_DATABASE"`
}

// DefaultMongoConfig returns sensible defaults for local development.
func DefaultMongoConfig(dbName string) MongoConfig {
	return MongoConfig{
		URI:      "mongodb://localhost:27017",
		Database: dbName,
	}
}

// NewMongoClient opens a MongoDB client and verifies connectivity.
// Caller is responsible for calling client.Disconnect() on shutdown.
func NewMongoClient(cfg MongoConfig) (*mongo.Client, *mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Client().ApplyURI(cfg.URI)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("ping mongo: %w", err)
	}

	db := client.Database(cfg.Database)
	return client, db, nil
}
