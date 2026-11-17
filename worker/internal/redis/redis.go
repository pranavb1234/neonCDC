package redis

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/pranavb1234/walflow/worker/internal/replication"
	"github.com/redis/go-redis/v9"
)

const StreamName = "cdc:events"

type StreamPublisher struct {
	client     *redis.Client
	pipelineID int64
}

func NewStreamPublisher(client *redis.Client) *StreamPublisher {
	return &StreamPublisher{client: client}
}

func NewPipelineStreamPublisher(client *redis.Client, pipelineID int64) *StreamPublisher {
	return &StreamPublisher{client: client, pipelineID: pipelineID}
}

func Connect(ctx context.Context, redisURL string) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, errors.New("could not parse Redis connection settings")
	}

	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, errors.New("could not connect to Redis")
	}

	return client, nil
}

func (p *StreamPublisher) Publish(ctx context.Context, events []replication.Event) error {
	if len(events) == 0 {
		return nil
	}

	pipeline := p.client.TxPipeline()
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			return errors.New("could not encode CDC event")
		}
		pipeline.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamName,
			Values: streamValues(event, payload, p.pipelineID),
		})
	}

	if _, err := pipeline.Exec(ctx); err != nil {
		return errors.New("could not write CDC events to Redis Stream")
	}
	return nil
}

func streamValues(event replication.Event, payload []byte, pipelineID int64) map[string]any {
	return map[string]any{
		"event_id":    event.ID,
		"table":       event.Table,
		"operation":   event.Operation,
		"commit_lsn":  event.CommitLSN,
		"payload":     string(payload),
		"pipeline_id": pipelineID,
	}
}
