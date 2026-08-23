CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at_ms INTEGER NOT NULL);

CREATE TABLE IF NOT EXISTS global_samples (
  ts_ms INTEGER PRIMARY KEY,
  version TEXT NOT NULL,
  uptime_seconds REAL NOT NULL,
  memory_bytes INTEGER NOT NULL,
  goroutines INTEGER NOT NULL,
  active_connections INTEGER NOT NULL,
  recent_connections INTEGER NOT NULL,
  connections_total INTEGER NOT NULL,
  upload_bytes_total INTEGER NOT NULL,
  download_bytes_total INTEGER NOT NULL,
  source_state TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dimension_samples (
  ts_ms INTEGER NOT NULL,
  dimension TEXT NOT NULL,
  value TEXT NOT NULL,
  active_connections INTEGER,
  connections_total INTEGER,
  upload_bytes_total INTEGER,
  download_bytes_total INTEGER,
  delay_ms INTEGER,
  measured_at_ms INTEGER,
  PRIMARY KEY (ts_ms, dimension, value)
);

CREATE TABLE IF NOT EXISTS connections (
  id TEXT PRIMARY KEY,
  state TEXT NOT NULL,
  network TEXT NOT NULL,
  inbound TEXT NOT NULL,
  source_ip TEXT,
  source_port INTEGER,
  source_mac TEXT,
  source_hostname TEXT,
  destination_ip TEXT,
  destination_port INTEGER NOT NULL,
  domain TEXT,
  process TEXT,
  user_name TEXT,
  outbound TEXT NOT NULL,
  outbound_type TEXT NOT NULL,
  chain_json TEXT NOT NULL,
  rule_text TEXT,
  started_at_ms INTEGER NOT NULL,
  closed_at_ms INTEGER,
  duration_seconds REAL NOT NULL,
  upload INTEGER NOT NULL,
  download INTEGER NOT NULL,
  last_seen_at_ms INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_connections_state_started ON connections(state, started_at_ms DESC);
CREATE INDEX IF NOT EXISTS idx_connections_closed ON connections(closed_at_ms DESC);
CREATE INDEX IF NOT EXISTS idx_connections_network_closed ON connections(network, closed_at_ms DESC);
CREATE INDEX IF NOT EXISTS idx_connections_outbound_closed ON connections(outbound, closed_at_ms DESC);
CREATE INDEX IF NOT EXISTS idx_global_samples_ts ON global_samples(ts_ms);
