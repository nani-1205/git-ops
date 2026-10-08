package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// RedisClient is the globally accessible Redis connection
	RedisClient *redis.Client
	
	// Ctx is a background context for Redis operations
	Ctx = context.Background()
)

// InitRedis connects to the Redis server and returns error if failed
func InitRedis(host, port, password string) error {
	addr := fmt.Sprintf("%s:%s", host, port)
	
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password, // no password set
		DB:       0,        // use default DB
	})

	// Test connection
	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis at %s: %w", addr, err)
	}

	log.Printf("Successfully connected to Redis at %s", addr)
	return nil
}
