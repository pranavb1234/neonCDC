package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, errors.New("could not parse Redis connection settings")
	}

	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, errors.New("could not connect to Redis")
	}

	return client, nil
}
