CREATE TABLE IF NOT EXISTS administrators (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS source_connections (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  connection_url_ciphertext BLOB NOT NULL,
  status TEXT NOT NULL DEFAULT 'unknown' CHECK (status IN ('unknown', 'healthy', 'failed')),
  last_tested_at TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pipelines (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  source_connection_id INTEGER NOT NULL REFERENCES source_connections(id) ON DELETE RESTRICT,
  publication_name TEXT NOT NULL,
  slot_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'stopped' CHECK (status IN ('stopped', 'starting', 'running', 'failed')),
  last_commit_lsn TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (source_connection_id, publication_name),
  UNIQUE (source_connection_id, slot_name)
);

CREATE TABLE IF NOT EXISTS pipeline_tables (
  pipeline_id INTEGER NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  schema_name TEXT NOT NULL,
  table_name TEXT NOT NULL,
  PRIMARY KEY (pipeline_id, schema_name, table_name)
);

CREATE TABLE IF NOT EXISTS sinks (
  id INTEGER PRIMARY KEY,
  pipeline_id INTEGER NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type = 'webhook'),
  webhook_url TEXT NOT NULL,
  signing_secret_ciphertext BLOB NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
  last_delivery_status TEXT CHECK (last_delivery_status IN ('success', 'failed')),
  last_delivery_at TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (pipeline_id, name)
);

CREATE TABLE IF NOT EXISTS delivery_attempts (
  id INTEGER PRIMARY KEY,
  pipeline_id INTEGER NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
  sink_id INTEGER NOT NULL REFERENCES sinks(id) ON DELETE CASCADE,
  event_id TEXT NOT NULL,
  redis_stream_id TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
  http_status INTEGER,
  error_message TEXT,
  attempted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipelines_source_connection ON pipelines(source_connection_id);
CREATE INDEX IF NOT EXISTS idx_sinks_pipeline ON sinks(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_sink_time ON delivery_attempts(sink_id, attempted_at DESC);
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_pipeline_time ON delivery_attempts(pipeline_id, attempted_at DESC);
