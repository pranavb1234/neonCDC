package replication

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pglogrepl"
)

const (
	boolOID        = 16
	int2OID        = 21
	int4OID        = 23
	int8OID        = 20
	textOID        = 25
	varcharOID     = 1043
	bpcharOID      = 1042
	numericOID     = 1700
	timestampOID   = 1114
	timestamptzOID = 1184
)

type Event struct {
	ID        string         `json:"id"`
	Table     string         `json:"table"`
	Operation string         `json:"operation"`
	CommitLSN string         `json:"commit_lsn"`
	Data      map[string]any `json:"data"`
	Before    map[string]any `json:"before,omitempty"`
	After     map[string]any `json:"after,omitempty"`
	CreatedAt string         `json:"created_at"`
}

type rowChange struct {
	table     string
	operation string
	data      map[string]any
	before    map[string]any
	after     map[string]any
}

type eventDecoder struct {
	relations map[uint32]*pglogrepl.RelationMessage
	pending   []rowChange
}

func newEventDecoder() *eventDecoder {
	return &eventDecoder{relations: make(map[uint32]*pglogrepl.RelationMessage)}
}

func (d *eventDecoder) Process(message pglogrepl.Message) ([]Event, error) {
	switch message := message.(type) {
	case *pglogrepl.RelationMessage:
		d.relations[message.RelationID] = message
	case *pglogrepl.BeginMessage:
		d.pending = nil
	case *pglogrepl.InsertMessage:
		change, err := d.insert(message)
		if err != nil {
			return nil, err
		}
		d.pending = append(d.pending, change)
	case *pglogrepl.UpdateMessage:
		change, err := d.update(message)
		if err != nil {
			return nil, err
		}
		d.pending = append(d.pending, change)
	case *pglogrepl.DeleteMessage:
		change, err := d.delete(message)
		if err != nil {
			return nil, err
		}
		d.pending = append(d.pending, change)
	case *pglogrepl.CommitMessage:
		return d.commit(message), nil
	}

	return nil, nil
}

func (d *eventDecoder) insert(message *pglogrepl.InsertMessage) (rowChange, error) {
	relation, err := d.relation(message.RelationID)
	if err != nil {
		return rowChange{}, err
	}
	after, err := decodeTuple(relation, message.Tuple)
	if err != nil {
		return rowChange{}, err
	}
	return rowChange{table: relation.RelationName, operation: "insert", data: after, after: after}, nil
}

func (d *eventDecoder) update(message *pglogrepl.UpdateMessage) (rowChange, error) {
	relation, err := d.relation(message.RelationID)
	if err != nil {
		return rowChange{}, err
	}
	before, err := decodeTuple(relation, message.OldTuple)
	if err != nil {
		return rowChange{}, err
	}
	after, err := decodeTuple(relation, message.NewTuple)
	if err != nil {
		return rowChange{}, err
	}
	return rowChange{table: relation.RelationName, operation: "update", data: after, before: before, after: after}, nil
}

func (d *eventDecoder) delete(message *pglogrepl.DeleteMessage) (rowChange, error) {
	relation, err := d.relation(message.RelationID)
	if err != nil {
		return rowChange{}, err
	}
	before, err := decodeTuple(relation, message.OldTuple)
	if err != nil {
		return rowChange{}, err
	}
	return rowChange{table: relation.RelationName, operation: "delete", data: before, before: before}, nil
}

func (d *eventDecoder) relation(relationID uint32) (*pglogrepl.RelationMessage, error) {
	relation, ok := d.relations[relationID]
	if !ok {
		return nil, fmt.Errorf("received row change for unknown relation %d", relationID)
	}
	return relation, nil
}

func (d *eventDecoder) commit(message *pglogrepl.CommitMessage) []Event {
	commitLSN := message.CommitLSN.String()
	createdAt := message.CommitTime.UTC().Format(time.RFC3339Nano)
	events := make([]Event, 0, len(d.pending))
	for index, change := range d.pending {
		events = append(events, Event{
			ID:        fmt.Sprintf("%s:%d", commitLSN, index+1),
			Table:     change.table,
			Operation: change.operation,
			CommitLSN: commitLSN,
			Data:      change.data,
			Before:    change.before,
			After:     change.after,
			CreatedAt: createdAt,
		})
	}
	d.pending = nil
	return events
}

func decodeTuple(relation *pglogrepl.RelationMessage, tuple *pglogrepl.TupleData) (map[string]any, error) {
	if tuple == nil {
		return nil, nil
	}

	columns := relation.Columns
	if len(tuple.Columns) != len(columns) {
		columns = identityColumns(relation.Columns)
	}
	if len(tuple.Columns) != len(columns) {
		return nil, errors.New("tuple column count does not match relation metadata")
	}

	values := make(map[string]any, len(tuple.Columns))
	for index, column := range tuple.Columns {
		value, err := decodeValue(columns[index].DataType, column)
		if err != nil {
			return nil, fmt.Errorf("decode %s.%s: %w", relation.RelationName, columns[index].Name, err)
		}
		if column.DataType != pglogrepl.TupleDataTypeToast {
			values[columns[index].Name] = value
		}
	}
	return values, nil
}

func identityColumns(columns []*pglogrepl.RelationMessageColumn) []*pglogrepl.RelationMessageColumn {
	identity := make([]*pglogrepl.RelationMessageColumn, 0)
	for _, column := range columns {
		if column.Flags == 1 {
			identity = append(identity, column)
		}
	}
	return identity
}

func decodeValue(oid uint32, column *pglogrepl.TupleDataColumn) (any, error) {
	if column.DataType == pglogrepl.TupleDataTypeNull {
		return nil, nil
	}
	if column.DataType == pglogrepl.TupleDataTypeToast {
		return nil, nil
	}
	if column.DataType != pglogrepl.TupleDataTypeText {
		return nil, errors.New("only text-formatted pgoutput values are supported")
	}

	value := string(column.Data)
	switch oid {
	case boolOID:
		decoded, err := strconv.ParseBool(value)
		return decoded, err
	case int2OID, int4OID, int8OID:
		decoded, err := strconv.ParseInt(value, 10, 64)
		return decoded, err
	case numericOID:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, err
		}
		return json.Number(value), nil
	case timestampOID, timestamptzOID:
		return decodeTimestamp(value), nil
	case textOID, varcharOID, bpcharOID:
		return value, nil
	default:
		return value, nil
	}
}

func decodeTimestamp(value string) string {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z07",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z07",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	return value
}
