package replication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pglogrepl"
)

func TestDecodeTupleSupportsSampleTypes(t *testing.T) {
	relation := &pglogrepl.RelationMessage{RelationName: "orders", Columns: []*pglogrepl.RelationMessageColumn{
		{Name: "id", DataType: int4OID},
		{Name: "amount", DataType: numericOID},
		{Name: "paid", DataType: boolOID},
		{Name: "created_at", DataType: timestamptzOID},
		{Name: "notes", DataType: textOID},
	}}
	tuple := &pglogrepl.TupleData{Columns: []*pglogrepl.TupleDataColumn{
		textColumn("7"), textColumn("19.99"), textColumn("true"), textColumn("2026-11-14 12:00:00+00"), nullColumn(),
	}}

	values, err := decodeTuple(relation, tuple)
	if err != nil {
		t.Fatal(err)
	}
	if values["id"] != int64(7) || values["paid"] != true || values["notes"] != nil {
		t.Fatalf("unexpected decoded values: %#v", values)
	}
	if values["amount"] != json.Number("19.99") {
		t.Fatalf("numeric value = %#v", values["amount"])
	}
	if values["created_at"] != "2026-11-14T12:00:00Z" {
		t.Fatalf("timestamp value = %#v", values["created_at"])
	}
}

func TestDecoderBuffersChangesUntilCommit(t *testing.T) {
	decoder := newEventDecoder()
	relation := &pglogrepl.RelationMessage{RelationID: 42, RelationName: "users", Columns: []*pglogrepl.RelationMessageColumn{
		{Name: "id", DataType: int4OID}, {Name: "name", DataType: textOID},
	}}
	if _, err := decoder.Process(relation); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Process(&pglogrepl.BeginMessage{}); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.Process(&pglogrepl.UpdateMessage{
		RelationID: 42,
		OldTuple:   &pglogrepl.TupleData{Columns: []*pglogrepl.TupleDataColumn{textColumn("1"), textColumn("Ada")}},
		NewTuple:   &pglogrepl.TupleData{Columns: []*pglogrepl.TupleDataColumn{textColumn("1"), textColumn("Ada Updated")}},
	}); err != nil {
		t.Fatal(err)
	}

	events, err := decoder.Process(&pglogrepl.CommitMessage{CommitLSN: 0x1C25B20, CommitTime: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d", len(events))
	}
	event := events[0]
	if event.ID != "0/1C25B20:1" || event.Operation != "update" || event.Before["name"] != "Ada" || event.After["name"] != "Ada Updated" {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func textColumn(value string) *pglogrepl.TupleDataColumn {
	return &pglogrepl.TupleDataColumn{DataType: pglogrepl.TupleDataTypeText, Data: []byte(value)}
}

func nullColumn() *pglogrepl.TupleDataColumn {
	return &pglogrepl.TupleDataColumn{DataType: pglogrepl.TupleDataTypeNull}
}
