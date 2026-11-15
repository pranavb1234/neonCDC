package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pranavb1234/neonCDC/worker/internal/config"
	"github.com/pranavb1234/neonCDC/worker/internal/logging"
	rediscache "github.com/pranavb1234/neonCDC/worker/internal/redis"
	"github.com/redis/go-redis/v9"
)

const (
	consumerGroup = "delivery"
	consumerName  = "delivery-1"
)

func main() {
	logger := logging.New()
	if err := run(logger); err != nil {
		logger.Error("delivery worker stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.LoadDelivery()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := rediscache.Connect(connectCtx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := ensureConsumerGroup(ctx, client); err != nil {
		return err
	}
	logger.Info("delivery worker connected", "redis_host", cfg.RedisHost)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	return consume(ctx, client, httpClient, cfg.WebhookURL, cfg.WebhookSecret, logger)
}

func ensureConsumerGroup(ctx context.Context, client *redis.Client) error {
	err := client.XGroupCreateMkStream(ctx, rediscache.StreamName, consumerGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return errors.New("could not create Redis consumer group")
	}
	return nil
}

func consume(ctx context.Context, client *redis.Client, httpClient *http.Client, webhookURL, webhookSecret string, logger *slog.Logger) error {
	if err := consumePending(ctx, client, httpClient, webhookURL, webhookSecret, logger); err != nil {
		return err
	}

	for {
		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{rediscache.StreamName, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("could not read from Redis Stream")
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				if err := handleMessage(ctx, client, httpClient, webhookURL, webhookSecret, message, logger); err != nil {
					return err
				}
			}
		}
	}
}

func consumePending(ctx context.Context, client *redis.Client, httpClient *http.Client, webhookURL, webhookSecret string, logger *slog.Logger) error {
	for {
		streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{rediscache.StreamName, "0"},
			Count:    1,
		}).Result()
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if err != nil {
			return errors.New("could not read pending Redis Stream messages")
		}
		if !hasMessages(streams) {
			return nil
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if err := handleMessage(ctx, client, httpClient, webhookURL, webhookSecret, message, logger); err != nil {
					return err
				}
			}
		}
	}
}

func hasMessages(streams []redis.XStream) bool {
	for _, stream := range streams {
		if len(stream.Messages) > 0 {
			return true
		}
	}
	return false
}

func handleMessage(ctx context.Context, client *redis.Client, httpClient *http.Client, webhookURL, webhookSecret string, message redis.XMessage, logger *slog.Logger) error {
	if err := deliver(ctx, httpClient, webhookURL, webhookSecret, message); err != nil {
		logger.Error("webhook delivery failed", "event_id", eventID(message), "error", err)
		return err
	}
	if err := client.XAck(ctx, rediscache.StreamName, consumerGroup, message.ID).Err(); err != nil {
		logger.Error("Redis message acknowledgement failed", "event_id", eventID(message), "error", err)
		return err
	}
	logger.Info("webhook delivered", "event_id", eventID(message))
	return nil
}

func deliver(ctx context.Context, client *http.Client, webhookURL, webhookSecret string, message redis.XMessage) error {
	payload, ok := message.Values["payload"].(string)
	if !ok || payload == "" {
		return errors.New("Redis message does not contain a JSON payload")
	}

	body := []byte(payload)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return errors.New("could not create webhook request")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CDC-Event-Id", eventID(message))
	request.Header.Set("X-CDC-Timestamp", timestamp)
	request.Header.Set("X-CDC-Signature", "v1="+sign(webhookSecret, timestamp, body))

	response, err := client.Do(request)
	if err != nil {
		return errors.New("webhook request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return nil
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func eventID(message redis.XMessage) string {
	if id, ok := message.Values["event_id"].(string); ok {
		return id
	}
	return "unknown"
}
