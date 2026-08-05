package server

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a new connection to redis.
func NewRedisClient() (*redis.Client, error) {
	redisURL := os.Getenv("REDIS_URL")

	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}
