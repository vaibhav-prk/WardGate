package server

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/vaibhav-prk/Wardgate/internal/config"
)

// NewRedisClient creates a new connection to redis.
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		PoolSize: cfg.PoolSize,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return rdb, nil
}
