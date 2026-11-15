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

type DeliveryAttempt struct {
	PipelineID    int64
	SinkID        int64
	EventID       string
	RedisStreamID string
	Status        string
	HTTPStatus    int
	Error         error
}

type SchemaObject struct {
	Type string
	Name string
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

func (s *Store) CreateSourceConnection(ctx context.Context, name, connectionURL string) (int64, error) {
	if strings.TrimSpace(name) == "" || !postgresURL(connectionURL) {
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
