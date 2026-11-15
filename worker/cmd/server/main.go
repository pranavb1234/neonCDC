package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pranavb1234/neonCDC/worker/internal/logging"
	"github.com/pranavb1234/neonCDC/worker/internal/metadata"
	"github.com/redis/go-redis/v9"
)

//go:embed all:web
var frontend embed.FS

const streamName = "cdc:events"

type app struct {
	store  *metadata.Store
	redis  *redis.Client
	logger *slog.Logger
}

type event struct {
	ID        string         `json:"id"`
	Table     string         `json:"table"`
	Operation string         `json:"operation"`
	CommitLSN string         `json:"commit_lsn"`
	Data      map[string]any `json:"data"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
	CreatedAt string         `json:"created_at"`
	StreamID  string         `json:"stream_id"`
}

func main() {
	logger := logging.New()
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := metadata.Open(ctx, metadata.Config{Path: envOrDefault("METADATA_DB_PATH", "/data/cdc-lite.db"), EncryptionKey: os.Getenv("APP_ENCRYPTION_KEY")})
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	if err := bootstrapAdmin(ctx, store); err != nil {
		return err
	}

	options, err := redis.ParseURL(envOrDefault("REDIS_URL", "redis://redis:6379"))
	if err != nil {
		return errors.New("could not parse REDIS_URL")
	}
	client := redis.NewClient(options)
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		return errors.New("could not connect to Redis")
	}

	application := &app{store: store, redis: client, logger: logger}
	server := &http.Server{Addr: ":" + envOrDefault("PORT", "3000"), Handler: application.routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	logger.Info("cdc-lite server started", "port", envOrDefault("PORT", "3000"))
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func bootstrapAdmin(ctx context.Context, store *metadata.Store) error {
	email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" && password == "" {
		exists, err := store.HasAdministrator(ctx)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD are required for the first startup")
		}
		return nil
	}
	if email == "" || password == "" {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	_, err := store.BootstrapAdministrator(ctx, email, []byte(password))
	return err
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/overview", a.overview)
	mux.HandleFunc("GET /api/events", a.events)
	mux.HandleFunc("GET /api/events/stream", a.stream)
	return a.static(mux)
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.redis.Ping(ctx).Err(); err != nil {
		http.Error(w, "Redis unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
}

func (a *app) overview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	count, err := a.redis.XLen(ctx, streamName).Result()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Redis is unavailable"})
		return
	}
	groups, _ := a.redis.XInfoGroups(ctx, streamName).Result()
	var pending, consumers int64
	for _, group := range groups {
		if group.Name == "delivery" {
			pending, consumers = group.Pending, group.Consumers
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"eventCount": count, "pending": pending, "consumers": consumers, "webhookUrl": "Configured in a saved sink"})
}

func (a *app) events(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	entries, err := a.redis.XRevRangeN(ctx, streamName, "+", "-", 100).Result()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"events": []event{}, "error": "Redis is unavailable"})
		return
	}
	items := make([]event, 0, len(entries))
	for _, entry := range entries {
		if item, err := decodeEvent(entry); err == nil {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": items})
}

func (a *app) stream(w http.ResponseWriter, r *http.Request) {
	lastID := r.URL.Query().Get("after")
	if lastID == "" {
		lastID = "$"
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "retry: 1000\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		readCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		streams, err := a.redis.XRead(readCtx, &redis.XReadArgs{Streams: []string{streamName, lastID}, Count: 20, Block: time.Second}).Result()
		cancel()
		if err != nil && !errors.Is(err, redis.Nil) && !errors.Is(err, context.DeadlineExceeded) {
			return
		}
		for _, stream := range streams {
			for _, entry := range stream.Messages {
				lastID = entry.ID
				item, err := decodeEvent(entry)
				if err != nil {
					continue
				}
				body, _ := json.Marshal(map[string]any{"stream_id": entry.ID, "event": item})
				fmt.Fprintf(w, "id: %s\nevent: cdc\ndata: %s\n\n", entry.ID, body)
				flusher.Flush()
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, "event: heartbeat\ndata: {}\n\n")
			flusher.Flush()
		default:
		}
	}
}

func decodeEvent(message redis.XMessage) (event, error) {
	payload, ok := message.Values["payload"].(string)
	if !ok {
		return event{}, errors.New("invalid event payload")
	}
	var item event
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		return event{}, err
	}
	item.StreamID = message.ID
	return item, nil
}

func (a *app) static(api http.Handler) http.Handler {
	assets, _ := fs.Sub(frontend, "web")
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			api.ServeHTTP(w, r)
			return
		}
		if !strings.Contains(r.URL.Path, ".") {
			page := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/"), "/")
			if page == "" {
				page = "index"
			} else {
				page += "/index"
			}
			if content, err := fs.ReadFile(assets, page+".html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(content)
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
