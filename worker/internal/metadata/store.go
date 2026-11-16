package metadata

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed migrations/0001_initial.sql
var initialSchema string

type Config struct {
	Path          string
	EncryptionKey string
}

type Store struct {
	db        *sql.DB
	encryptor *encryptor
}

type PipelineTable struct {
	Schema string
	Table  string
}

type Pipeline struct {
	ID                 int64           `json:"id"`
	Name               string          `json:"name"`
	SourceConnectionID int64           `json:"source_connection_id"`
	PublicationName    string          `json:"publication_name"`
	SlotName           string          `json:"slot_name"`
	Status             string          `json:"status"`
	LastError          *string         `json:"last_error,omitempty"`
	Tables             []PipelineTable `json:"tables"`
}

type Sink struct {
	ID                 int64                   `json:"id"`
	PipelineID         int64                   `json:"pipeline_id"`
	PipelineName       string                  `json:"pipeline_name,omitempty"`
	Name               string                  `json:"name"`
	Type               string                  `json:"type"`
	WebhookURL         string                  `json:"webhook_url"`
	Enabled            bool                    `json:"enabled"`
	LastDeliveryStatus *string                 `json:"last_delivery_status,omitempty"`
	LastDeliveryAt     *string                 `json:"last_delivery_at,omitempty"`
	LastError          *string                 `json:"last_error,omitempty"`
	Health             string                  `json:"health"`
	LastSuccessfulAt   *string                 `json:"last_successful_at,omitempty"`
	LastFailedAt       *string                 `json:"last_failed_at,omitempty"`
	RecentAttempts     []DeliveryAttemptRecord `json:"recent_attempts"`
}

type DeliveryAttempt struct {
	PipelineID    int64
	SinkID        int64
	EventID       string
	RedisStreamID string
	Status        string
	HTTPStatus    int
	Error         error
}

// DeliveryAttemptRecord is intentionally limited to operational metadata. It
// never contains a webhook secret or event payload.
type DeliveryAttemptRecord struct {
	EventID     string  `json:"event_id"`
	Status      string  `json:"status"`
	HTTPStatus  *int    `json:"http_status,omitempty"`
	Error       *string `json:"error,omitempty"`
	AttemptedAt string  `json:"attempted_at"`
}

const deliveryHistoryLimit = 20

type SchemaObject struct {
	Type string
	Name string
}

type SourceConnection struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	LastTestedAt *string `json:"last_tested_at,omitempty"`
	LastError    *string `json:"last_error,omitempty"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func Open(ctx context.Context, cfg Config) (*Store, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, errors.New("METADATA_DB_PATH is required")
	}
	encryptor, err := newEncryptor(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
		return nil, errors.New("could not create metadata database directory")
	}
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, errors.New("could not open metadata database")
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("could not connect to metadata database")
	}
	for _, statement := range []string{"PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000"} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, errors.New("could not configure metadata database")
		}
	}
	return &Store{db: db, encryptor: encryptor}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return errors.New("could not initialize metadata migrations")
	}
	var applied bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = 1)`).Scan(&applied); err != nil {
		return errors.New("could not read metadata migrations")
	}
	if applied {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("could not start metadata migration")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, initialSchema); err != nil {
		return errors.New("could not apply metadata migration 1")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (1)`); err != nil {
		return errors.New("could not record metadata migration 1")
	}
	if err := tx.Commit(); err != nil {
		return errors.New("could not commit metadata migration")
	}
	return nil
}

func (s *Store) BootstrapAdministrator(ctx context.Context, email string, password []byte) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 12 {
		return false, errors.New("administrator email and a password of at least 12 characters are required")
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return false, errors.New("could not hash administrator password")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO administrators (id, email, password_hash) VALUES (1, ?, ?) ON CONFLICT(id) DO NOTHING`, email, hash)
	if err != nil {
		return false, errors.New("could not create administrator")
	}
	created, err := result.RowsAffected()
	return created == 1, err
}

func (s *Store) HasAdministrator(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM administrators)`).Scan(&exists); err != nil {
		return false, errors.New("could not read administrator")
	}
	return exists, nil
}

func (s *Store) UpdateAdministratorPassword(ctx context.Context, password []byte) error {
	if len(password) < 12 {
		return errors.New("administrator password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return errors.New("could not hash administrator password")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE administrators SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`, hash)
	if err != nil {
		return errors.New("could not update administrator password")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("administrator has not been initialized")
	}
	return nil
}

func (s *Store) CreateSourceConnection(ctx context.Context, name, connectionURL string) (int64, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(connectionURL) == "" {
		return 0, errors.New("connection name and PostgreSQL connection URL are required")
	}
	ciphertext, err := s.encryptor.encrypt(connectionURL)
	if err != nil {
		return 0, errors.New("could not encrypt PostgreSQL connection URL")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO source_connections (name, connection_url_ciphertext) VALUES (?, ?)`, name, ciphertext)
	if err != nil {
		return 0, errors.New("could not save source connection")
	}
	return result.LastInsertId()
}

func (s *Store) SourceConnectionURL(ctx context.Context, id int64) (string, error) {
	var ciphertext []byte
	if err := s.db.QueryRowContext(ctx, `SELECT connection_url_ciphertext FROM source_connections WHERE id = ?`, id).Scan(&ciphertext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("database connection was not found")
		}
		return "", errors.New("could not read database connection")
	}
	return s.encryptor.decrypt(ciphertext)
}

func (s *Store) ListSourceConnections(ctx context.Context) ([]SourceConnection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, status, last_tested_at, last_error, created_at, updated_at FROM source_connections ORDER BY name`)
	if err != nil {
		return nil, errors.New("could not list database connections")
	}
	defer rows.Close()
	connections := []SourceConnection{}
	for rows.Next() {
		var item SourceConnection
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.LastTestedAt, &item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, errors.New("could not read database connection")
		}
		connections = append(connections, item)
	}
	return connections, rows.Err()
}

func (s *Store) RenameSourceConnection(ctx context.Context, id int64, name string) error {
	if id <= 0 || strings.TrimSpace(name) == "" {
		return errors.New("database display name is required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE source_connections SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, strings.TrimSpace(name), id)
	if err != nil {
		return errors.New("could not update database display name")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("database connection was not found")
	}
	return nil
}

func (s *Store) SetSourceConnectionTest(ctx context.Context, id int64, status string, lastError string) error {
	if status != "healthy" && status != "failed" {
		return errors.New("invalid database connection status")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE source_connections SET status = ?, last_tested_at = CURRENT_TIMESTAMP, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, nullableString(lastError), id)
	if err != nil {
		return errors.New("could not save database test status")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("database connection was not found")
	}
	return nil
}

func (s *Store) DeleteSourceConnection(ctx context.Context, id int64) error {
	var dependencies int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipelines WHERE source_connection_id = ?`, id).Scan(&dependencies); err != nil {
		return errors.New("could not check database dependencies")
	}
	if dependencies > 0 {
		return errors.New("database connection has pipelines and cannot be deleted")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM source_connections WHERE id = ?`, id)
	if err != nil {
		return errors.New("could not delete database connection")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("database connection was not found")
	}
	return nil
}

func (s *Store) CreatePipeline(ctx context.Context, name string, sourceConnectionID int64, publicationName, slotName string, tables []PipelineTable) (int64, error) {
	if strings.TrimSpace(name) == "" || sourceConnectionID <= 0 || strings.TrimSpace(publicationName) == "" || strings.TrimSpace(slotName) == "" || len(tables) == 0 {
		return 0, errors.New("pipeline name, source connection, publication, slot, and at least one table are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errors.New("could not start pipeline transaction")
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO pipelines (name, source_connection_id, publication_name, slot_name) VALUES (?, ?, ?, ?)`, name, sourceConnectionID, publicationName, slotName)
	if err != nil {
		return 0, errors.New("could not save pipeline")
	}
	pipelineID, err := result.LastInsertId()
	if err != nil {
		return 0, errors.New("could not read saved pipeline")
	}
	for _, table := range tables {
		if strings.TrimSpace(table.Schema) == "" || strings.TrimSpace(table.Table) == "" {
			return 0, errors.New("pipeline tables require schema and table names")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO pipeline_tables (pipeline_id, schema_name, table_name) VALUES (?, ?, ?)`, pipelineID, table.Schema, table.Table); err != nil {
			return 0, errors.New("could not save pipeline table")
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, errors.New("could not commit pipeline")
	}
	return pipelineID, nil
}

func (s *Store) ListPipelines(ctx context.Context, sourceConnectionID int64) ([]Pipeline, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, source_connection_id, publication_name, slot_name, status, last_error FROM pipelines WHERE source_connection_id = ? ORDER BY name`, sourceConnectionID)
	if err != nil {
		return nil, errors.New("could not list pipelines")
	}
	items := []Pipeline{}
	for rows.Next() {
		var item Pipeline
		if err := rows.Scan(&item.ID, &item.Name, &item.SourceConnectionID, &item.PublicationName, &item.SlotName, &item.Status, &item.LastError); err != nil {
			return nil, errors.New("could not read pipeline")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, errors.New("could not finish listing pipelines")
	}
	// SQLite uses a single connection in this small installation. Finish the
	// pipeline query before querying each pipeline's table selection.
	for index := range items {
		tables, err := s.pipelineTables(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		items[index].Tables = tables
	}
	return items, nil
}

func (s *Store) ListAllPipelines(ctx context.Context) ([]Pipeline, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, source_connection_id, publication_name, slot_name, status, last_error FROM pipelines ORDER BY name`)
	if err != nil {
		return nil, errors.New("could not list pipelines")
	}
	items := []Pipeline{}
	for rows.Next() {
		var item Pipeline
		if err := rows.Scan(&item.ID, &item.Name, &item.SourceConnectionID, &item.PublicationName, &item.SlotName, &item.Status, &item.LastError); err != nil {
			rows.Close()
			return nil, errors.New("could not read pipeline")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range items {
		tables, err := s.pipelineTables(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		items[index].Tables = tables
	}
	return items, nil
}

func (s *Store) MarkInterruptedPipelinesStopped(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pipelines SET status = 'stopped', last_error = 'Stopped because CDC Lite restarted.', updated_at = CURRENT_TIMESTAMP WHERE status IN ('running', 'starting')`)
	if err != nil {
		return errors.New("could not recover pipeline statuses")
	}
	return nil
}

func (s *Store) Pipeline(ctx context.Context, id int64) (Pipeline, error) {
	var item Pipeline
	err := s.db.QueryRowContext(ctx, `SELECT id, name, source_connection_id, publication_name, slot_name, status, last_error FROM pipelines WHERE id = ?`, id).Scan(&item.ID, &item.Name, &item.SourceConnectionID, &item.PublicationName, &item.SlotName, &item.Status, &item.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return Pipeline{}, errors.New("pipeline was not found")
	}
	if err != nil {
		return Pipeline{}, errors.New("could not read pipeline")
	}
	tables, err := s.pipelineTables(ctx, id)
	if err != nil {
		return Pipeline{}, err
	}
	item.Tables = tables
	return item, nil
}

func (s *Store) SetPipelineStatus(ctx context.Context, id int64, status, lastError string) error {
	if status != "stopped" && status != "starting" && status != "running" && status != "failed" {
		return errors.New("invalid pipeline status")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE pipelines SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, nullableString(lastError), id)
	if err != nil {
		return errors.New("could not update pipeline status")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("pipeline was not found")
	}
	return nil
}

func (s *Store) DeletePipeline(ctx context.Context, id int64) error {
	var status string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM pipelines WHERE id = ?`, id).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return errors.New("pipeline was not found")
	} else if err != nil {
		return errors.New("could not read pipeline")
	}
	if status == "running" || status == "starting" {
		return errors.New("stop the running pipeline before deleting it")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pipelines WHERE id = ?`, id); err != nil {
		return errors.New("could not delete pipeline")
	}
	return nil
}

func (s *Store) pipelineTables(ctx context.Context, pipelineID int64) ([]PipelineTable, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT schema_name, table_name FROM pipeline_tables WHERE pipeline_id = ? ORDER BY schema_name, table_name`, pipelineID)
	if err != nil {
		return nil, errors.New("could not read pipeline tables")
	}
	defer rows.Close()
	tables := []PipelineTable{}
	for rows.Next() {
		var table PipelineTable
		if err := rows.Scan(&table.Schema, &table.Table); err != nil {
			return nil, errors.New("could not read pipeline table")
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (s *Store) CreateWebhookSink(ctx context.Context, pipelineID int64, name, webhookURL, signingSecret string) (int64, error) {
	parsed, err := url.Parse(webhookURL)
	if pipelineID <= 0 || strings.TrimSpace(name) == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || signingSecret == "" {
		return 0, errors.New("pipeline, sink name, HTTP webhook URL, and signing secret are required")
	}
	ciphertext, err := s.encryptor.encrypt(signingSecret)
	if err != nil {
		return 0, errors.New("could not encrypt webhook signing secret")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO sinks (pipeline_id, name, type, webhook_url, signing_secret_ciphertext) VALUES (?, ?, 'webhook', ?, ?)`, pipelineID, name, webhookURL, ciphertext)
	if err != nil {
		return 0, errors.New("could not save webhook sink")
	}
	return result.LastInsertId()
}

func (s *Store) ListSinks(ctx context.Context) ([]Sink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.pipeline_id, p.name, s.name, s.type, s.webhook_url, s.enabled, s.last_delivery_status, s.last_delivery_at, s.last_error FROM sinks s JOIN pipelines p ON p.id = s.pipeline_id ORDER BY s.name`)
	if err != nil {
		return nil, errors.New("could not list sinks")
	}
	sinks := []Sink{}
	for rows.Next() {
		var item Sink
		if err := rows.Scan(&item.ID, &item.PipelineID, &item.PipelineName, &item.Name, &item.Type, &item.WebhookURL, &item.Enabled, &item.LastDeliveryStatus, &item.LastDeliveryAt, &item.LastError); err != nil {
			return nil, errors.New("could not read sink")
		}
		sinks = append(sinks, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, errors.New("could not finish listing sinks")
	}
	for index := range sinks {
		attempts, err := s.deliveryAttempts(ctx, sinks[index].ID, deliveryHistoryLimit)
		if err != nil {
			return nil, err
		}
		sinks[index].RecentAttempts = attempts
		sinks[index].Health, sinks[index].LastSuccessfulAt, sinks[index].LastFailedAt = sinkHealth(sinks[index].Enabled, attempts)
	}
	return sinks, nil
}

func (s *Store) Sink(ctx context.Context, sinkID int64) (Sink, error) {
	var sink Sink
	err := s.db.QueryRowContext(ctx, `SELECT s.id, s.pipeline_id, p.name, s.name, s.type, s.webhook_url, s.enabled, s.last_delivery_status, s.last_delivery_at, s.last_error FROM sinks s JOIN pipelines p ON p.id = s.pipeline_id WHERE s.id = ?`, sinkID).Scan(&sink.ID, &sink.PipelineID, &sink.PipelineName, &sink.Name, &sink.Type, &sink.WebhookURL, &sink.Enabled, &sink.LastDeliveryStatus, &sink.LastDeliveryAt, &sink.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return Sink{}, errors.New("sink was not found")
	}
	if err != nil {
		return Sink{}, errors.New("could not read sink")
	}
	return sink, nil
}

func (s *Store) deliveryAttempts(ctx context.Context, sinkID int64, limit int) ([]DeliveryAttemptRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, status, http_status, error_message, attempted_at FROM delivery_attempts WHERE sink_id = ? ORDER BY id DESC LIMIT ?`, sinkID, limit)
	if err != nil {
		return nil, errors.New("could not list delivery attempts")
	}
	defer rows.Close()
	attempts := []DeliveryAttemptRecord{}
	for rows.Next() {
		var item DeliveryAttemptRecord
		if err := rows.Scan(&item.EventID, &item.Status, &item.HTTPStatus, &item.Error, &item.AttemptedAt); err != nil {
			return nil, errors.New("could not read delivery attempt")
		}
		attempts = append(attempts, item)
	}
	return attempts, rows.Err()
}

func sinkHealth(enabled bool, attempts []DeliveryAttemptRecord) (string, *string, *string) {
	if len(attempts) == 0 {
		if !enabled {
			return "paused", nil, nil
		}
		return "unknown", nil, nil
	}
	var lastSuccess, lastFailure *string
	failures := 0
	for _, attempt := range attempts {
		if attempt.Status == "success" {
			if lastSuccess == nil {
				value := attempt.AttemptedAt
				lastSuccess = &value
			}
		} else {
			failures++
			if lastFailure == nil {
				value := attempt.AttemptedAt
				lastFailure = &value
			}
		}
	}
	if !enabled {
		return "paused", lastSuccess, lastFailure
	}
	if len(attempts) >= 3 && failures == len(attempts) {
		return "failing", lastSuccess, lastFailure
	}
	if attempts[0].Status == "failed" {
		return "degraded", lastSuccess, lastFailure
	}
	return "healthy", lastSuccess, lastFailure
}

func (s *Store) EnabledSinksForPipeline(ctx context.Context, pipelineID int64) ([]Sink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, pipeline_id, name, type, webhook_url, enabled, last_delivery_status, last_delivery_at, last_error FROM sinks WHERE pipeline_id = ? AND enabled = 1 ORDER BY id`, pipelineID)
	if err != nil {
		return nil, errors.New("could not list pipeline sinks")
	}
	defer rows.Close()
	sinks := []Sink{}
	for rows.Next() {
		var item Sink
		if err := rows.Scan(&item.ID, &item.PipelineID, &item.Name, &item.Type, &item.WebhookURL, &item.Enabled, &item.LastDeliveryStatus, &item.LastDeliveryAt, &item.LastError); err != nil {
			return nil, errors.New("could not read sink")
		}
		sinks = append(sinks, item)
	}
	return sinks, rows.Err()
}

func (s *Store) WebhookSigningSecret(ctx context.Context, sinkID int64) (string, error) {
	var ciphertext []byte
	if err := s.db.QueryRowContext(ctx, `SELECT signing_secret_ciphertext FROM sinks WHERE id = ?`, sinkID).Scan(&ciphertext); err != nil {
		return "", errors.New("could not read sink secret")
	}
	return s.encryptor.decrypt(ciphertext)
}

func (s *Store) SetSinkEnabled(ctx context.Context, sinkID int64, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `UPDATE sinks SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, enabled, sinkID)
	if err != nil {
		return errors.New("could not update sink")
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return errors.New("sink was not found")
	}
	return nil
}

func (s *Store) RecordDeliveryAttempt(ctx context.Context, attempt DeliveryAttempt) error {
	if attempt.PipelineID <= 0 || attempt.SinkID <= 0 || attempt.EventID == "" || attempt.RedisStreamID == "" || (attempt.Status != "success" && attempt.Status != "failed") {
		return errors.New("invalid delivery attempt")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("could not start delivery history transaction")
	}
	defer tx.Rollback()
	message := safeError(attempt.Error)
	if _, err := tx.ExecContext(ctx, `INSERT INTO delivery_attempts (pipeline_id, sink_id, event_id, redis_stream_id, status, http_status, error_message) VALUES (?, ?, ?, ?, ?, ?, ?)`, attempt.PipelineID, attempt.SinkID, attempt.EventID, attempt.RedisStreamID, attempt.Status, nullableStatus(attempt.HTTPStatus), nullableString(message)); err != nil {
		return errors.New("could not record delivery attempt")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sinks SET last_delivery_status = ?, last_delivery_at = ?, last_error = ?, updated_at = ? WHERE id = ?`, attempt.Status, time.Now().UTC().Format(time.RFC3339Nano), nullableString(message), time.Now().UTC().Format(time.RFC3339Nano), attempt.SinkID); err != nil {
		return errors.New("could not update sink delivery status")
	}
	// Keep at most 100 records per sink. The overview returns only the newest
	// 20, so delivery history cannot grow without bound on a small installation.
	if _, err := tx.ExecContext(ctx, `DELETE FROM delivery_attempts WHERE sink_id = ? AND id NOT IN (SELECT id FROM delivery_attempts WHERE sink_id = ? ORDER BY id DESC LIMIT 100)`, attempt.SinkID, attempt.SinkID); err != nil {
		return errors.New("could not trim delivery history")
	}
	if err := tx.Commit(); err != nil {
		return errors.New("could not commit delivery history")
	}
	return nil
}

func (s *Store) Schema(ctx context.Context) ([]SchemaObject, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT type, name FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		return nil, errors.New("could not inspect metadata schema")
	}
	defer rows.Close()
	objects := []SchemaObject{}
	for rows.Next() {
		var object SchemaObject
		if err := rows.Scan(&object.Type, &object.Name); err != nil {
			return nil, errors.New("could not read metadata schema")
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}

func postgresURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") && parsed.Hostname() != ""
}

func nullableStatus(status int) any {
	if status == 0 {
		return nil
	}
	return status
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var urlPattern = regexp.MustCompile(`(?i)(?:postgres(?:ql)?|https?)://[^\s]+`)
var sensitivePairPattern = regexp.MustCompile(`(?i)(password|secret|token)=([^\s&]+)`)

func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := sensitivePairPattern.ReplaceAllString(err.Error(), "$1=[redacted]")
	message = urlPattern.ReplaceAllString(message, "[redacted-url]")
	if len(message) > 500 {
		return fmt.Sprintf("%s…", message[:499])
	}
	return message
}
