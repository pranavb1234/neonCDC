package main

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestHasMessages(t *testing.T) {
	if hasMessages([]redis.XStream{{Stream: "cdc:events"}}) {
		t.Fatal("empty stream container should not count as a message")
	}
	if !hasMessages([]redis.XStream{{Stream: "cdc:events", Messages: []redis.XMessage{{ID: "1-0"}}}}) {
		t.Fatal("stream message should be detected")
	}
}

func TestSignIsStableForTimestampAndBody(t *testing.T) {
	signature := sign("demo-secret", "1731690000", []byte(`{"id":"event-1"}`))
	const expected = "42d87ffb736de34a60795be6dec5b00a22c492953fae39c2525dd3b05354264f"
	if signature != expected {
		t.Fatalf("signature = %s", signature)
	}
}
