package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	SourceDBURL     string
	RedisURL        string
	ReplicationSlot string
	PublicationName string
	PostgresHost    string
	RedisHost       string
}

type DeliveryConfig struct {
	RedisURL   string
	WebhookURL string
	RedisHost  string
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
	replicationSlot := envOrDefault("REPLICATION_SLOT", "cdc_lite_slot")
	publicationName := envOrDefault("PUBLICATION_NAME", "cdc_lite_publication")
	if !isIdentifier(replicationSlot) || !isIdentifier(publicationName) {
		return Config{}, errors.New("REPLICATION_SLOT and PUBLICATION_NAME must be lowercase PostgreSQL identifiers")
	}

	return Config{
		SourceDBURL:     sourceDBURL,
		RedisURL:        redisURL,
		ReplicationSlot: replicationSlot,
		PublicationName: publicationName,
		PostgresHost:    source.Hostname(),
		RedisHost:       redis.Hostname(),
	}, nil
}

func LoadDelivery() (DeliveryConfig, error) {
	redisURL := os.Getenv("REDIS_URL")
	webhookURL := os.Getenv("WEBHOOK_URL")
	if redisURL == "" {
		return DeliveryConfig{}, errors.New("REDIS_URL is required")
	}
	if webhookURL == "" {
		return DeliveryConfig{}, errors.New("WEBHOOK_URL is required")
	}

	redis, err := url.Parse(redisURL)
	if err != nil || (redis.Scheme != "redis" && redis.Scheme != "rediss") || redis.Hostname() == "" {
		return DeliveryConfig{}, errors.New("REDIS_URL must be a Redis connection URL")
	}
	webhook, err := url.Parse(webhookURL)
	if err != nil || (webhook.Scheme != "http" && webhook.Scheme != "https") || webhook.Hostname() == "" {
		return DeliveryConfig{}, errors.New("WEBHOOK_URL must be an HTTP or HTTPS URL")
	}

	return DeliveryConfig{RedisURL: redisURL, WebhookURL: webhookURL, RedisHost: redis.Hostname()}, nil
}

func secureSSLMode(mode string) bool {
	return mode == "require" || mode == "verify-ca" || mode == "verify-full"
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isIdentifier(value string) bool {
	if value == "" || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z') && !(character >= '0' && character <= '9') && character != '_' {
			return false
		}
	}
	return true
}
