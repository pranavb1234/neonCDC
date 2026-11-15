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
