package metadata

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
)

func TestMetadataStoresEncryptedSecretsAndDeliveryHistory(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	store, err := Open(context.Background(), Config{Path: filepath.Join(t.TempDir(), "metadata.db"), EncryptionKey: key})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if created, err := store.BootstrapAdministrator(context.Background(), "admin@example.com", []byte("a-safe-password")); err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}

	connectionURL := "postgresql://owner:password@db.example.com/app?sslmode=require"
	connectionID, err := store.CreateSourceConnection(context.Background(), "Primary", connectionURL)
	if err != nil {
		t.Fatal(err)
	}
	var encrypted []byte
	if err := store.db.QueryRowContext(context.Background(), `SELECT connection_url_ciphertext FROM source_connections WHERE id = ?`, connectionID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if string(encrypted) == connectionURL {
		t.Fatal("connection URL was stored in plaintext")
	}

	pipelineID, err := store.CreatePipeline(context.Background(), "Primary users", connectionID, "cdc_publication", "cdc_slot", []PipelineTable{{Schema: "public", Table: "users"}})
	if err != nil {
		t.Fatal(err)
	}
	sinkID, err := store.CreateWebhookSink(context.Background(), pipelineID, "Receiver", "https://receiver.example.com/webhook", "signing-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDeliveryAttempt(context.Background(), DeliveryAttempt{PipelineID: pipelineID, SinkID: sinkID, EventID: "event-1", RedisStreamID: "1-0", Status: "failed", Error: errors.New("request to postgresql://owner:password@db.example.com failed")}); err != nil {
		t.Fatal(err)
	}
	var lastError string
	if err := store.db.QueryRowContext(context.Background(), `SELECT last_error FROM sinks WHERE id = ?`, sinkID).Scan(&lastError); err != nil {
		t.Fatal(err)
	}
	if lastError == "" || lastError == "request to postgresql://owner:password@db.example.com failed" {
		t.Fatal("sink error was not safely redacted")
	}
}

func TestSinkHealthRequiresDeliveryEvidence(t *testing.T) {
	if health, _, _ := sinkHealth(true, nil); health != "unknown" {
		t.Fatalf("health without deliveries = %q", health)
	}
	success := []DeliveryAttemptRecord{{Status: "success", AttemptedAt: "2026-01-01T00:00:00Z"}}
	if health, _, _ := sinkHealth(true, success); health != "healthy" {
		t.Fatalf("health after success = %q", health)
	}
	failures := []DeliveryAttemptRecord{{Status: "failed"}, {Status: "failed"}, {Status: "failed"}}
	if health, _, _ := sinkHealth(true, failures); health != "failing" {
		t.Fatalf("health after repeated failures = %q", health)
	}
	if health, _, _ := sinkHealth(false, success); health != "paused" {
		t.Fatalf("disabled sink health = %q", health)
	}
}
