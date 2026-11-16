package redis

import (
	"encoding/json"
	"testing"

	"github.com/pranavb1234/neonCDC/worker/internal/replication"
)

func TestStreamValuesContainEventMetadataAndJSONPayload(t *testing.T) {
	event := replication.Event{
		ID:        "0/1C25B20:1",
		Table:     "users",
		Operation: "update",
		CommitLSN: "0/1C25B20",
		Data:      map[string]any{"id": int64(1), "name": "Ada"},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	values := streamValues(event, payload, 7)
	if values["event_id"] != event.ID || values["table"] != "users" || values["operation"] != "update" || values["commit_lsn"] != event.CommitLSN {
		t.Fatalf("unexpected stream values: %#v", values)
	}
	if values["payload"] != string(payload) {
		t.Fatalf("payload = %#v", values["payload"])
	}
	if values["pipeline_id"] != int64(7) {
		t.Fatalf("pipeline_id = %#v", values["pipeline_id"])
	}
}
