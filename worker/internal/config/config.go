package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	SourceDBURL  string
	RedisURL     string
	PostgresHost string
	RedisHost    string
}

func Load() (Config, error) {
	sourceDBURL := os.Getenv("SOURCE_DB_URL")
	redisURL := os.Getenv("REDIS_URL")

	if sourceDBURL == "" {
		return Config{}, errors.New("SOURCE_DB_URL is required")
	}
	if redisURL == "" {
		return Config{}, errors.New("REDIS_URL is required")
	}

	source, err := url.Parse(sourceDBURL)
	if err != nil || (source.Scheme != "postgres" && source.Scheme != "postgresql") || source.Hostname() == "" {
		return Config{}, errors.New("SOURCE_DB_URL must be a PostgreSQL connection URL")
	}
	if strings.Contains(source.Hostname(), "-pooler") {
		return Config{}, errors.New("SOURCE_DB_URL must use Neon's direct, unpooled endpoint")
	}
	if !secureSSLMode(source.Query().Get("sslmode")) {
		return Config{}, errors.New("SOURCE_DB_URL must use sslmode=require, verify-ca, or verify-full")
	}

	redis, err := url.Parse(redisURL)
	if err != nil || (redis.Scheme != "redis" && redis.Scheme != "rediss") || redis.Hostname() == "" {
		return Config{}, errors.New("REDIS_URL must be a Redis connection URL")
	}

	return Config{
		SourceDBURL:  sourceDBURL,
		RedisURL:     redisURL,
		PostgresHost: source.Hostname(),
		RedisHost:    redis.Hostname(),
	}, nil
}

func secureSSLMode(mode string) bool {
	return mode == "require" || mode == "verify-ca" || mode == "verify-full"
}
