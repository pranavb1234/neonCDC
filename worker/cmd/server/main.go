package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pranavb1234/neonCDC/worker/internal/logging"
	"github.com/pranavb1234/neonCDC/worker/internal/metadata"
	"github.com/pranavb1234/neonCDC/worker/internal/postgres"
	rediscache "github.com/pranavb1234/neonCDC/worker/internal/redis"
	"github.com/pranavb1234/neonCDC/worker/internal/replication"
	"github.com/redis/go-redis/v9"
)

//go:embed all:web
var frontend embed.FS

const streamName = "cdc:events"

type app struct {
	store  *metadata.Store
	redis  *redis.Client
	logger *slog.Logger
	jobs   *pipelineManager
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

type databaseInput struct {
	Name             string `json:"name"`
	ConnectionString string `json:"connection_string"`
}
type renameDatabaseInput struct {
	Name string `json:"name"`
}
type connectionTest struct {
	SQLConnected       bool   `json:"sql_connected"`
	LogicalReplication bool   `json:"logical_replication"`
	Message            string `json:"message"`
}
type tableColumn struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	Default  *string `json:"default,omitempty"`
}
type tableDetails struct {
	Schema          string        `json:"schema"`
	Table           string        `json:"table"`
	Columns         []tableColumn `json:"columns"`
	PrimaryKey      []string      `json:"primary_key"`
	ReplicaIdentity string        `json:"replica_identity"`
	CaptureGuidance string        `json:"capture_guidance"`
}
type pipelineInput struct {
	Name   string                   `json:"name"`
	Tables []metadata.PipelineTable `json:"tables"`
}
type pipelineManager struct {
	mu      sync.Mutex
	running map[int64]context.CancelFunc
}

func main() {
	logger := logging.New()
	if len(os.Args) > 1 && os.Args[1] == "admin-password" {
		if err := resetAdminPassword(); err != nil {
			logger.Error("administrator password update failed", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func resetAdminPassword() error {
	flags := flag.NewFlagSet("admin-password", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	readStdin := flags.Bool("stdin", false, "read password from standard input")
	if err := flags.Parse(os.Args[2:]); err != nil || !*readStdin {
		return errors.New("use admin-password --stdin")
	}
	password, err := io.ReadAll(os.Stdin)
	if err != nil {
		return errors.New("could not read administrator password")
	}
	store, err := metadata.Open(context.Background(), metadata.Config{Path: envOrDefault("METADATA_DB_PATH", "/data/cdc-lite.db"), EncryptionKey: os.Getenv("APP_ENCRYPTION_KEY")})
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		return err
	}
	return store.UpdateAdministratorPassword(context.Background(), []byte(strings.TrimRight(string(password), "\r\n")))
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

	application := &app{store: store, redis: client, logger: logger, jobs: &pipelineManager{running: map[int64]context.CancelFunc{}}}
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
	exists, err := store.HasAdministrator(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if email == "" && password == "" {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD are required for the first startup")
	}
	if email == "" || password == "" {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	_, err = store.BootstrapAdministrator(ctx, email, []byte(password))
	return err
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/overview", a.overview)
	mux.HandleFunc("GET /api/events", a.events)
	mux.HandleFunc("GET /api/events/stream", a.stream)
	mux.HandleFunc("GET /api/databases", a.listDatabases)
	mux.HandleFunc("POST /api/databases", a.createDatabase)
	mux.HandleFunc("POST /api/databases/test", a.testDatabaseInput)
	mux.HandleFunc("POST /api/databases/{id}/test", a.testSavedDatabase)
	mux.HandleFunc("PATCH /api/databases/{id}", a.renameDatabase)
	mux.HandleFunc("DELETE /api/databases/{id}", a.deleteDatabase)
	mux.HandleFunc("GET /api/databases/{id}/schemas", a.listSchemas)
	mux.HandleFunc("GET /api/databases/{id}/schemas/{schema}/tables", a.listTables)
	mux.HandleFunc("GET /api/databases/{id}/schemas/{schema}/tables/{table}", a.tableDetails)
	mux.HandleFunc("GET /api/databases/{id}/schemas/{schema}/tables/{table}/preview", a.tablePreview)
	mux.HandleFunc("GET /api/databases/{id}/pipelines", a.listPipelines)
	mux.HandleFunc("POST /api/databases/{id}/pipelines", a.createPipeline)
	mux.HandleFunc("POST /api/pipelines/{id}/start", a.startPipeline)
	mux.HandleFunc("POST /api/pipelines/{id}/stop", a.stopPipeline)
	mux.HandleFunc("DELETE /api/pipelines/{id}", a.deletePipeline)
	return a.static(mux)
}

func (a *app) listDatabases(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.ListSourceConnections(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not list saved databases"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"databases": items})
}

func (a *app) testDatabaseInput(w http.ResponseWriter, r *http.Request) {
	input, err := decodeJSON[databaseInput](r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Enter a PostgreSQL connection string"})
		return
	}
	result, status := testPostgres(r.Context(), input.ConnectionString)
	writeJSON(w, status, result)
}

func (a *app) createDatabase(w http.ResponseWriter, r *http.Request) {
	input, err := decodeJSON[databaseInput](r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Enter a display name and PostgreSQL connection string"})
		return
	}
	result, status := testPostgres(r.Context(), input.ConnectionString)
	if status != http.StatusOK {
		writeJSON(w, status, result)
		return
	}
	id, err := a.store.CreateSourceConnection(r.Context(), input.Name, input.ConnectionString)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not save database. Choose a unique display name."})
		return
	}
	if err := a.store.SetSourceConnectionTest(r.Context(), id, "healthy", ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Database was saved but its test status could not be stored"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "message": "Database saved. CDC has not started."})
}

func (a *app) testSavedDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid database id"})
		return
	}
	connectionString, err := a.store.SourceConnectionURL(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Database was not found"})
		return
	}
	result, status := testPostgres(r.Context(), connectionString)
	dbStatus, lastError := "healthy", ""
	if status != http.StatusOK {
		dbStatus, lastError = "failed", result.Message
	}
	_ = a.store.SetSourceConnectionTest(r.Context(), id, dbStatus, lastError)
	writeJSON(w, status, result)
}

func (a *app) renameDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	input, inputErr := decodeJSON[renameDatabaseInput](r)
	if err != nil || inputErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Enter a database display name"})
		return
	}
	if err := a.store.RenameSourceConnection(r.Context(), id, input.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not update database display name"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Database display name updated"})
}

func (a *app) deleteDatabase(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid database id"})
		return
	}
	if err := a.store.DeleteSourceConnection(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "pipelines") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) listPipelines(w http.ResponseWriter, r *http.Request) {
	databaseID, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid database id"})
		return
	}
	pipelines, err := a.store.ListPipelines(r.Context(), databaseID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not list pipelines"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pipelines": pipelines, "active_limit": 1})
}

func (a *app) createPipeline(w http.ResponseWriter, r *http.Request) {
	databaseID, err := pathID(r)
	input, inputErr := decodeJSON[pipelineInput](r)
	if err != nil || inputErr != nil || strings.TrimSpace(input.Name) == "" || len(input.Tables) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Enter a pipeline name and select at least one table."})
		return
	}
	if err := a.validatePipelineTables(r, databaseID, input.Tables); err != nil {
		inspectionError(w, err)
		return
	}
	publicationName, slotName := pipelineNames(databaseID, input.Name)
	pipelineID, err := a.store.CreatePipeline(r.Context(), input.Name, databaseID, publicationName, slotName, input.Tables)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not save pipeline. Choose a unique pipeline name."})
		return
	}
	if err := a.launchPipeline(pipelineID); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": pipelineID, "message": "Pipeline created and starting."})
}

func (a *app) startPipeline(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid pipeline id"})
		return
	}
	if err := a.launchPipeline(id); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "Pipeline is starting."})
}

func (a *app) stopPipeline(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid pipeline id"})
		return
	}
	if !a.jobs.stop(id) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Pipeline is not running."})
		return
	}
	if err := a.store.SetPipelineStatus(r.Context(), id, "stopped", ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Could not update pipeline status."})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Pipeline stopped. Its slot and publication were preserved for restart."})
}

func (a *app) deletePipeline(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid pipeline id"})
		return
	}
	pipeline, err := a.store.Pipeline(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Pipeline was not found."})
		return
	}
	a.jobs.mu.Lock()
	running := a.jobs.running[id] != nil
	a.jobs.mu.Unlock()
	if running || pipeline.Status == "running" || pipeline.Status == "starting" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Stop the running pipeline before deleting it."})
		return
	}
	if err := a.cleanupPipeline(r.Context(), pipeline); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Could not remove this pipeline's PostgreSQL slot/publication. The pipeline remains saved."})
		return
	}
	if err := a.store.DeletePipeline(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) launchPipeline(id int64) error {
	a.jobs.mu.Lock()
	defer a.jobs.mu.Unlock()
	if len(a.jobs.running) >= 1 {
		return errors.New("CDC Lite first release supports one active pipeline at a time. Stop the active pipeline first.")
	}
	if a.jobs.running[id] != nil {
		return errors.New("pipeline is already running")
	}
	pipeline, err := a.store.Pipeline(context.Background(), id)
	if err != nil {
		return err
	}
	if err := a.store.SetPipelineStatus(context.Background(), id, "starting", ""); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.jobs.running[id] = cancel
	go func() {
		defer func() { a.jobs.mu.Lock(); delete(a.jobs.running, id); a.jobs.mu.Unlock() }()
		connectionString, err := a.store.SourceConnectionURL(context.Background(), pipeline.SourceConnectionID)
		if err != nil {
			_ = a.store.SetPipelineStatus(context.Background(), id, "failed", "Could not read saved database connection.")
			return
		}
		connectCtx, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		db, err := postgres.Connect(connectCtx, connectionString)
		if err != nil {
			_ = a.store.SetPipelineStatus(context.Background(), id, "failed", "Could not connect to source database.")
			return
		}
		defer db.Close(context.Background())
		publisher := rediscache.NewStreamPublisher(a.redis)
		_ = a.store.SetPipelineStatus(context.Background(), id, "running", "")
		tables := make([]replication.Table, 0, len(pipeline.Tables))
		for _, table := range pipeline.Tables {
			tables = append(tables, replication.Table{Schema: table.Schema, Name: table.Table})
		}
		if err := replication.Run(ctx, db, replication.Config{SourceDBURL: connectionString, SlotName: pipeline.SlotName, PublicationName: pipeline.PublicationName, Tables: tables}, publisher, a.logger); err != nil && ctx.Err() == nil {
			_ = a.store.SetPipelineStatus(context.Background(), id, "failed", "Logical replication stopped unexpectedly.")
		}
	}()
	return nil
}

func (m *pipelineManager) stop(id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancel := m.running[id]
	if cancel == nil {
		return false
	}
	cancel()
	delete(m.running, id)
	return true
}

func (a *app) validatePipelineTables(r *http.Request, databaseID int64, tables []metadata.PipelineTable) error {
	connectionString, err := a.store.SourceConnectionURL(r.Context(), databaseID)
	if err != nil {
		return err
	}
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return errors.New("saved database configuration is invalid")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("could not connect to saved database")
	}
	defer conn.Close(context.Background())
	var walLevel string
	if err := conn.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel); err != nil || walLevel != "logical" {
		return errors.New("logical replication is not enabled on this source database")
	}
	var canReplicate bool
	if err := conn.QueryRow(ctx, "SELECT rolreplication FROM pg_roles WHERE rolname = current_user").Scan(&canReplicate); err != nil || !canReplicate {
		return errors.New("source database role does not have REPLICATION permission")
	}
	for _, table := range tables {
		if !inspectableSchema(table.Schema) || !inspectableName(table.Table) {
			return errors.New("invalid selected table")
		}
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2 AND table_type = 'BASE TABLE')`, table.Schema, table.Table).Scan(&exists); err != nil || !exists {
			return errors.New("a selected table is no longer available")
		}
	}
	return nil
}

func pipelineNames(databaseID int64, name string) (string, string) {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s", databaseID, name))))[:12]
	return "cdc_lite_pub_" + hash, "cdc_lite_slot_" + hash
}

func (a *app) cleanupPipeline(parent context.Context, pipeline metadata.Pipeline) error {
	connectionString, err := a.store.SourceConnectionURL(parent, pipeline.SourceConnectionID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "SELECT pg_drop_replication_slot($1) WHERE EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1)", pipeline.SlotName); err != nil {
		return err
	}
	publicationIdentifier := pgx.Identifier{pipeline.PublicationName}.Sanitize()
	if _, err := conn.Exec(ctx, "DROP PUBLICATION IF EXISTS "+publicationIdentifier); err != nil {
		return err
	}
	return nil
}

func (a *app) listSchemas(w http.ResponseWriter, r *http.Request) {
	var schemas []string
	err := a.readOnlyDatabase(r, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT nspname FROM pg_namespace WHERE nspname <> 'information_schema' AND nspname NOT LIKE 'pg\_%' ESCAPE '\' AND has_schema_privilege(oid, 'USAGE') ORDER BY nspname`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var schema string
			if err := rows.Scan(&schema); err != nil {
				return err
			}
			schemas = append(schemas, schema)
		}
		return rows.Err()
	})
	if err != nil {
		inspectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schemas": schemas})
}

func (a *app) listTables(w http.ResponseWriter, r *http.Request) {
	schema := r.PathValue("schema")
	if !inspectableSchema(schema) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "System schemas cannot be browsed."})
		return
	}
	var tables []string
	err := a.readOnlyDatabase(r, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`, schema)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				return err
			}
			tables = append(tables, table)
		}
		return rows.Err()
	})
	if err != nil {
		inspectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (a *app) tableDetails(w http.ResponseWriter, r *http.Request) {
	schema, table := r.PathValue("schema"), r.PathValue("table")
	if !inspectableSchema(schema) || !inspectableName(table) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid schema or table."})
		return
	}
	details := tableDetails{Schema: schema, Table: table}
	err := a.readOnlyDatabase(r, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT column_name, data_type, is_nullable = 'YES', column_default FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, schema, table)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var column tableColumn
			if err := rows.Scan(&column.Name, &column.Type, &column.Nullable, &column.Default); err != nil {
				return err
			}
			details.Columns = append(details.Columns, column)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(details.Columns) == 0 {
			return errTableNotFound
		}
		pkRows, err := tx.Query(ctx, `SELECT a.attname FROM pg_index i JOIN pg_class c ON c.oid = i.indrelid JOIN pg_namespace n ON n.oid = c.relnamespace JOIN unnest(i.indkey) WITH ORDINALITY AS keys(attnum, position) ON true JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = keys.attnum WHERE i.indisprimary AND n.nspname = $1 AND c.relname = $2 ORDER BY keys.position`, schema, table)
		if err != nil {
			return err
		}
		defer pkRows.Close()
		for pkRows.Next() {
			var column string
			if err := pkRows.Scan(&column); err != nil {
				return err
			}
			details.PrimaryKey = append(details.PrimaryKey, column)
		}
		if err := pkRows.Err(); err != nil {
			return err
		}
		var identity string
		if err := tx.QueryRow(ctx, `SELECT CASE relreplident WHEN 'f' THEN 'full' WHEN 'i' THEN 'index' WHEN 'n' THEN 'nothing' ELSE 'default' END FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2`, schema, table).Scan(&identity); err != nil {
			return err
		}
		details.ReplicaIdentity = identity
		if identity == "full" {
			details.CaptureGuidance = "REPLICA IDENTITY FULL is enabled; UPDATE and DELETE events can include complete before-row data."
		} else if len(details.PrimaryKey) == 0 {
			details.CaptureGuidance = "No primary key. Configure REPLICA IDENTITY FULL before capturing UPDATE or DELETE events."
		} else {
			details.CaptureGuidance = "A primary key exists, but REPLICA IDENTITY FULL is recommended when complete before-row data is required for UPDATE and DELETE events."
		}
		return nil
	})
	if err != nil {
		inspectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (a *app) tablePreview(w http.ResponseWriter, r *http.Request) {
	schema, table := r.PathValue("schema"), r.PathValue("table")
	if !inspectableSchema(schema) || !inspectableName(table) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid schema or table."})
		return
	}
	response := map[string]any{"columns": []string{}, "rows": []map[string]any{}, "limit": 25}
	err := a.readOnlyDatabase(r, func(ctx context.Context, tx pgx.Tx) error {
		identifier := pgx.Identifier{schema, table}.Sanitize()
		rows, err := tx.Query(ctx, "SELECT * FROM "+identifier+" LIMIT $1", 25)
		if err != nil {
			return err
		}
		defer rows.Close()
		fields := rows.FieldDescriptions()
		columns := make([]string, len(fields))
		for index, field := range fields {
			columns[index] = field.Name
		}
		response["columns"] = columns
		values := []map[string]any{}
		for rows.Next() {
			rowValues, err := rows.Values()
			if err != nil {
				return err
			}
			row := map[string]any{}
			for index, value := range rowValues {
				row[columns[index]] = value
			}
			values = append(values, row)
		}
		response["rows"] = values
		return rows.Err()
	})
	if err != nil {
		inspectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

var errTableNotFound = errors.New("table was not found")

func (a *app) readOnlyDatabase(r *http.Request, work func(context.Context, pgx.Tx) error) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	connectionString, err := a.store.SourceConnectionURL(r.Context(), id)
	if err != nil {
		return err
	}
	config, err := pgx.ParseConfig(connectionString)
	if err != nil {
		return errors.New("saved database configuration is invalid")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return errors.New("could not connect to saved database")
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return errors.New("could not open read-only database session")
	}
	defer tx.Rollback(ctx)
	if err := work(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func inspectionError(w http.ResponseWriter, err error) {
	if errors.Is(err, errTableNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Table was not found or is not available to this role."})
		return
	}
	if strings.Contains(err.Error(), "not found") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Saved database was not found."})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Could not inspect the saved database. Check its availability and the role's read permissions."})
}

func inspectableSchema(schema string) bool {
	return inspectableName(schema) && schema != "information_schema" && !strings.HasPrefix(strings.ToLower(schema), "pg_")
}
func inspectableName(value string) bool { return value != "" && !strings.ContainsAny(value, "/\x00") }

func testPostgres(parent context.Context, connectionString string) (connectionTest, int) {
	config, err := pgx.ParseConfig(connectionString)
	if err != nil || config.Host == "" {
		return connectionTest{Message: "Enter a valid PostgreSQL connection string."}, http.StatusBadRequest
	}
	if strings.Contains(strings.ToLower(config.Host), "pooler") {
		return connectionTest{Message: "Use a direct, non-pooled PostgreSQL endpoint for logical replication."}, http.StatusBadRequest
	}
	if !isLocalHost(config.Host) && !hasSecureSSLMode(connectionString) {
		return connectionTest{Message: "Remote databases must use TLS. Add sslmode=require, verify-ca, or verify-full."}, http.StatusBadRequest
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return connectionTest{Message: "Could not connect to PostgreSQL. Check the host, port, credentials, network access, and TLS settings."}, http.StatusBadGateway
	}
	defer conn.Close(context.Background())
	if err := conn.Ping(ctx); err != nil {
		return connectionTest{Message: "PostgreSQL did not respond to a connection check."}, http.StatusBadGateway
	}
	result := connectionTest{SQLConnected: true}
	var walLevel string
	if err := conn.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel); err != nil || walLevel != "logical" {
		result.Message = "SQL connection works, but logical replication is not enabled. Set wal_level to logical on the source database."
		return result, http.StatusUnprocessableEntity
	}
	var canReplicate bool
	if err := conn.QueryRow(ctx, "SELECT rolreplication FROM pg_roles WHERE rolname = current_user").Scan(&canReplicate); err != nil || !canReplicate {
		result.Message = "SQL connection works, but this role does not have REPLICATION permission."
		return result, http.StatusUnprocessableEntity
	}
	result.LogicalReplication = true
	result.Message = "Connection and logical replication prerequisites are ready. Saving does not start CDC."
	return result, http.StatusOK
}

func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hasSecureSSLMode(connectionString string) bool {
	if parsed, err := url.Parse(connectionString); err == nil && parsed.Scheme != "" {
		mode := parsed.Query().Get("sslmode")
		return mode == "require" || mode == "verify-ca" || mode == "verify-full"
	}
	for _, field := range strings.Fields(connectionString) {
		if strings.HasPrefix(field, "sslmode=") {
			mode := strings.Trim(strings.TrimPrefix(field, "sslmode="), "'")
			return mode == "require" || mode == "verify-ca" || mode == "verify-full"
		}
	}
	return false
}
func decodeJSON[T any](r *http.Request) (T, error) {
	var value T
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&value)
	return value, err
}
func pathID(r *http.Request) (int64, error) {
	value := r.PathValue("id")
	if value == "" {
		return 0, errors.New("missing id")
	}
	var id int64
	_, err := fmt.Sscan(value, &id)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
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
