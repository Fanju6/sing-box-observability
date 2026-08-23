package storage

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db      *sql.DB
	writeMu sync.Mutex
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		directory := filepath.Clean(filepath.Dir(path))
		_, statErr := os.Stat(directory)
		directoryCreated := errors.Is(statErr, os.ErrNotExist)
		if statErr != nil && !directoryCreated {
			return nil, fmt.Errorf("inspect storage directory: %w", statErr)
		}
		if err := os.MkdirAll(directory, 0700); err != nil {
			return nil, fmt.Errorf("create storage directory: %w", err)
		}
		if directoryCreated && directory != "." && filepath.Dir(directory) != directory {
			if err := os.Chmod(directory, 0700); err != nil {
				return nil, fmt.Errorf("secure storage directory: %w", err)
			}
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(4)
	}
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
		if _, err = db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if path != ":memory:" {
		for _, file := range []string{path, path + "-wal", path + "-shm"} {
			if _, statErr := os.Stat(file); errors.Is(statErr, os.ErrNotExist) {
				continue
			} else if statErr != nil {
				db.Close()
				return nil, fmt.Errorf("inspect storage file: %w", statErr)
			}
			if chmodErr := os.Chmod(file, 0600); chmodErr != nil {
				db.Close()
				return nil, fmt.Errorf("secure storage file: %w", chmodErr)
			}
		}
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at_ms INTEGER NOT NULL)`); err != nil {
		return err
	}
	var applied int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&applied)
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var version int
		_, _ = fmt.Sscanf(entry.Name(), "%d_", &version)
		if version <= applied {
			continue
		}
		data, readErr := migrationFiles.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			return readErr
		}
		if _, err = tx.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at_ms) VALUES(?,?)", version, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) InsertSnapshot(ctx context.Context, snap model.MetricSnapshot, dims []model.DimensionSnapshot, state model.SourceState) error {
	if snap.ObservedAt.IsZero() {
		return errors.New("snapshot timestamp is missing")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	ts := snap.ObservedAt.UnixMilli()
	_, err = tx.ExecContext(ctx, `INSERT INTO global_samples(ts_ms,version,uptime_seconds,memory_bytes,goroutines,active_connections,recent_connections,connections_total,upload_bytes_total,download_bytes_total,source_state) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(ts_ms) DO UPDATE SET version=excluded.version,uptime_seconds=excluded.uptime_seconds,memory_bytes=excluded.memory_bytes,goroutines=excluded.goroutines,active_connections=excluded.active_connections,recent_connections=excluded.recent_connections,connections_total=excluded.connections_total,upload_bytes_total=excluded.upload_bytes_total,download_bytes_total=excluded.download_bytes_total,source_state=excluded.source_state`, ts, snap.Version, snap.UptimeSeconds, snap.MemoryBytes, snap.Goroutines, snap.ActiveConnections, snap.RecentConnections, snap.ConnectionsTotal, snap.UploadBytesTotal, snap.DownloadBytesTotal, string(state))
	if err != nil {
		return err
	}
	for _, d := range dims {
		_, err = tx.ExecContext(ctx, `INSERT INTO dimension_samples(ts_ms,dimension,value,active_connections,connections_total,upload_bytes_total,download_bytes_total,delay_ms,measured_at_ms) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(ts_ms,dimension,value) DO UPDATE SET active_connections=excluded.active_connections,connections_total=excluded.connections_total,upload_bytes_total=excluded.upload_bytes_total,download_bytes_total=excluded.download_bytes_total,delay_ms=excluded.delay_ms,measured_at_ms=excluded.measured_at_ms`, ts, d.Kind, d.Value, d.Active, d.Connections, d.UploadTotal, d.DownloadTotal, d.DelayMs, d.MeasuredAtMs)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (s *Store) UpsertConnection(ctx context.Context, c model.Connection) error {
	if c.ID == "" || c.StartedAt.IsZero() {
		return errors.New("connection id and startedAt are required")
	}
	if c.Chain == nil {
		c.Chain = []string{}
	}
	chain, _ := json.Marshal(c.Chain)
	closed := any(nil)
	if c.ClosedAt != nil {
		closed = c.ClosedAt.UnixMilli()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT INTO connections(id,state,network,inbound,source_ip,source_port,source_mac,source_hostname,destination_ip,destination_port,domain,process,user_name,outbound,outbound_type,chain_json,rule_text,started_at_ms,closed_at_ms,duration_seconds,upload,download,last_seen_at_ms) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET state=excluded.state,network=excluded.network,inbound=excluded.inbound,source_ip=COALESCE(excluded.source_ip,connections.source_ip),source_port=COALESCE(excluded.source_port,connections.source_port),source_mac=COALESCE(excluded.source_mac,connections.source_mac),source_hostname=COALESCE(excluded.source_hostname,connections.source_hostname),destination_ip=COALESCE(excluded.destination_ip,connections.destination_ip),destination_port=excluded.destination_port,domain=COALESCE(excluded.domain,connections.domain),process=COALESCE(excluded.process,connections.process),user_name=COALESCE(excluded.user_name,connections.user_name),outbound=excluded.outbound,outbound_type=excluded.outbound_type,chain_json=excluded.chain_json,rule_text=COALESCE(excluded.rule_text,connections.rule_text),started_at_ms=excluded.started_at_ms,closed_at_ms=COALESCE(excluded.closed_at_ms,connections.closed_at_ms),duration_seconds=excluded.duration_seconds,upload=excluded.upload,download=excluded.download,last_seen_at_ms=excluded.last_seen_at_ms`, c.ID, c.State, c.Network, c.Inbound, nullString(c.SourceIP), nullInt(c.SourcePort), nullString(c.SourceMAC), nullString(c.SourceHostname), nullString(c.DestinationIP), c.DestinationPort, nullString(c.Domain), nullString(c.Process), nullString(c.User), c.Outbound, c.OutboundType, string(chain), nullString(c.Rule), c.StartedAt.UnixMilli(), closed, c.DurationSeconds, c.Upload, c.Download, time.Now().UTC().UnixMilli())
	return err
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

type connectionRow struct {
	id, state, network, inbound                                                         string
	sourceIP, sourceMAC, sourceHostname, destinationIP, domain, process, userName, rule sql.NullString
	sourcePort, destinationPort                                                         sql.NullInt64
	outbound, outboundType, chainJSON                                                   string
	started, closed, lastSeen                                                           sql.NullInt64
	duration                                                                            sql.NullFloat64
	upload, download                                                                    sql.NullInt64
}

func scanConnection(row interface{ Scan(...any) error }) (model.Connection, error) {
	var r connectionRow
	err := row.Scan(&r.id, &r.state, &r.network, &r.inbound, &r.sourceIP, &r.sourcePort, &r.sourceMAC, &r.sourceHostname, &r.destinationIP, &r.destinationPort, &r.domain, &r.process, &r.userName, &r.outbound, &r.outboundType, &r.chainJSON, &r.rule, &r.started, &r.closed, &r.duration, &r.upload, &r.download, &r.lastSeen)
	if err != nil {
		return model.Connection{}, err
	}
	c := model.Connection{ID: r.id, State: r.state, Network: r.network, Inbound: r.inbound, DestinationPort: int(r.destinationPort.Int64), Outbound: r.outbound, OutboundType: r.outboundType, StartedAt: time.UnixMilli(r.started.Int64).UTC(), Upload: r.upload.Int64, Download: r.download.Int64, DurationSeconds: r.duration.Float64}
	_ = json.Unmarshal([]byte(r.chainJSON), &c.Chain)
	if c.Chain == nil {
		c.Chain = []string{}
	}
	if r.closed.Valid {
		t := time.UnixMilli(r.closed.Int64).UTC()
		c.ClosedAt = &t
	}
	for _, x := range []struct {
		n string
		v sql.NullString
		d *string
	}{{"sourceIP", r.sourceIP, &c.SourceIP}, {"sourceMAC", r.sourceMAC, &c.SourceMAC}, {"sourceHostname", r.sourceHostname, &c.SourceHostname}, {"destinationIP", r.destinationIP, &c.DestinationIP}, {"domain", r.domain, &c.Domain}, {"process", r.process, &c.Process}, {"user", r.userName, &c.User}, {"rule", r.rule, &c.Rule}} {
		if x.v.Valid {
			*x.d = x.v.String
		}
	}
	if r.sourcePort.Valid {
		c.SourcePort = int(r.sourcePort.Int64)
	}
	return c, nil
}

const connectionSelect = `SELECT id,state,network,inbound,source_ip,source_port,source_mac,source_hostname,destination_ip,destination_port,domain,process,user_name,outbound,outbound_type,chain_json,rule_text,started_at_ms,closed_at_ms,duration_seconds,upload,download,last_seen_at_ms FROM connections`

func (s *Store) ListActive(ctx context.Context, q, network, outbound string, limit, offset int) ([]model.Connection, int, error) {
	return s.listConnections(ctx, "active", time.Time{}, time.Time{}, q, network, outbound, true, limit, offset)
}
func (s *Store) ListRecent(ctx context.Context, from, to time.Time, q, network, outbound string, includeSensitive bool, limit, offset int) ([]model.Connection, int, error) {
	return s.listConnections(ctx, "closed", from, to, q, network, outbound, includeSensitive, limit, offset)
}
func (s *Store) listConnections(ctx context.Context, state string, from, to time.Time, q, network, outbound string, includeSensitive bool, limit, offset int) ([]model.Connection, int, error) {
	where := []string{"state=?"}
	args := []any{state}
	if !from.IsZero() {
		where = append(where, "closed_at_ms>=?")
		args = append(args, from.UnixMilli())
	}
	if !to.IsZero() {
		where = append(where, "closed_at_ms<?")
		args = append(args, to.UnixMilli())
	}
	if network != "" {
		where = append(where, "network=?")
		args = append(args, network)
	}
	if outbound != "" {
		where = append(where, "outbound=?")
		args = append(args, outbound)
	}
	if q != "" {
		pattern := "%" + escapeLike(strings.ToLower(q)) + "%"
		cols := []string{"id", "network", "inbound", "outbound", "outbound_type"}
		if includeSensitive {
			cols = append(cols, "source_ip", "source_hostname", "destination_ip", "domain", "process", "user_name", "rule_text")
		}
		parts := make([]string, len(cols))
		for i, col := range cols {
			parts[i] = "LOWER(COALESCE(" + col + ",'')) LIKE ? ESCAPE '\\'"
			args = append(args, pattern)
		}
		where = append(where, "("+strings.Join(parts, " OR ")+")")
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM connections"+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, connectionSelect+whereSQL+" ORDER BY CASE WHEN state='active' THEN started_at_ms ELSE closed_at_ms END DESC, id ASC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]model.Connection, 0)
	for rows.Next() {
		c, e := scanConnection(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}
func escapeLike(v string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(v)
}

type History struct {
	Totals *model.RangeTotals
	Series []model.TimePoint
}
type sample struct {
	ts                                                        time.Time
	version                                                   string
	uptime                                                    float64
	active, memory, goroutines, connections, upload, download int64
}

func (s *Store) History(ctx context.Context, from, to time.Time, step, sampleInterval time.Duration) (History, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ts_ms,version,uptime_seconds,active_connections,memory_bytes,goroutines,connections_total,upload_bytes_total,download_bytes_total FROM global_samples WHERE ts_ms<=? AND ts_ms>=? ORDER BY ts_ms`, to.UnixMilli(), from.Add(-3*sampleInterval).UnixMilli())
	if err != nil {
		return History{}, err
	}
	defer rows.Close()
	var samples []sample
	for rows.Next() {
		var x sample
		var ms int64
		if err := rows.Scan(&ms, &x.version, &x.uptime, &x.active, &x.memory, &x.goroutines, &x.connections, &x.upload, &x.download); err != nil {
			return History{}, err
		}
		x.ts = time.UnixMilli(ms).UTC()
		samples = append(samples, x)
	}
	if err := rows.Err(); err != nil {
		return History{}, err
	}
	if len(samples) == 0 {
		return History{Series: []model.TimePoint{}}, nil
	}
	type bucketAggregate struct {
		point                  model.TimePoint
		uploadDelta, downDelta int64
		validSeconds           float64
		hasRate                bool
	}
	buckets := map[int64]*bucketAggregate{}
	var totals model.RangeTotals
	hasDelta := false
	hasInRange := false
	for i := 1; i < len(samples); i++ {
		prev, curr := samples[i-1], samples[i]
		if curr.ts.Before(from) || !curr.ts.Before(to) {
			continue
		}
		hasInRange = true
		dt := curr.ts.Sub(prev.ts).Seconds()
		valid := dt > 0 && dt <= 3*sampleInterval.Seconds() && curr.uptime >= prev.uptime && curr.version == prev.version && curr.upload >= prev.upload && curr.download >= prev.download && curr.connections >= prev.connections
		if valid {
			totals.UploadBytes += curr.upload - prev.upload
			totals.DownloadBytes += curr.download - prev.download
			totals.Connections += curr.connections - prev.connections
			hasDelta = true
		}
		bucket := from.Add(time.Duration(curr.ts.Sub(from)/step) * step)
		key := bucket.UnixMilli()
		aggregate := buckets[key]
		if aggregate == nil {
			aggregate = &bucketAggregate{point: model.TimePoint{Timestamp: bucket}}
			buckets[key] = aggregate
		}
		if valid {
			aggregate.uploadDelta += curr.upload - prev.upload
			aggregate.downDelta += curr.download - prev.download
			aggregate.validSeconds += dt
			aggregate.hasRate = true
		}
		active, memory, goroutines := curr.active, curr.memory, curr.goroutines
		aggregate.point.ActiveConnections = &active
		aggregate.point.MemoryBytes = &memory
		aggregate.point.Goroutines = &goroutines
	}
	if !hasInRange {
		return History{Series: []model.TimePoint{}}, nil
	}
	series := make([]model.TimePoint, 0, int(math.Ceil(float64(to.Sub(from))/float64(step))))
	for bucket := from; bucket.Before(to); bucket = bucket.Add(step) {
		aggregate := buckets[bucket.UnixMilli()]
		if aggregate == nil {
			series = append(series, model.TimePoint{Timestamp: bucket})
			continue
		}
		if aggregate.hasRate && aggregate.validSeconds > 0 {
			u := float64(aggregate.uploadDelta) / aggregate.validSeconds
			d := float64(aggregate.downDelta) / aggregate.validSeconds
			if finiteRate(u) {
				aggregate.point.UploadBytesPerSecond = &u
			}
			if finiteRate(d) {
				aggregate.point.DownloadBytesPerSecond = &d
			}
		}
		series = append(series, aggregate.point)
	}
	var totalPtr *model.RangeTotals
	if hasDelta {
		totalPtr = &totals
	}
	return History{Totals: totalPtr, Series: series}, nil
}
func finiteRate(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

type dimensionSample struct {
	ts                                           time.Time
	version                                      string
	uptime                                       float64
	active, connections, upload, download, delay sql.NullInt64
}

func (s *Store) DimensionHistory(ctx context.Context, dimension, value string, from, to time.Time, step, sampleInterval time.Duration) ([]model.DimensionTimePoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.ts_ms,g.version,g.uptime_seconds,d.active_connections,d.connections_total,
		       d.upload_bytes_total,d.download_bytes_total,d.delay_ms
		FROM dimension_samples d
		JOIN global_samples g ON g.ts_ms=d.ts_ms
		WHERE d.dimension=? AND d.value=? AND d.ts_ms<=? AND d.ts_ms>=?
		ORDER BY d.ts_ms`, dimension, value, to.UnixMilli(), from.Add(-3*sampleInterval).UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var samples []dimensionSample
	for rows.Next() {
		var item dimensionSample
		var ms int64
		if err = rows.Scan(&ms, &item.version, &item.uptime, &item.active, &item.connections, &item.upload, &item.download, &item.delay); err != nil {
			return nil, err
		}
		item.ts = time.UnixMilli(ms).UTC()
		samples = append(samples, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	type bucketAggregate struct {
		point                                       model.DimensionTimePoint
		uploadDelta, downloadDelta, connectionDelta int64
		validSeconds                                float64
		hasRate                                     bool
	}
	buckets := make(map[int64]*bucketAggregate)
	for i := 1; i < len(samples); i++ {
		previous, current := samples[i-1], samples[i]
		if current.ts.Before(from) || !current.ts.Before(to) {
			continue
		}
		bucket := from.Add(time.Duration(current.ts.Sub(from)/step) * step)
		aggregate := buckets[bucket.UnixMilli()]
		if aggregate == nil {
			aggregate = &bucketAggregate{point: model.DimensionTimePoint{Timestamp: bucket}}
			buckets[bucket.UnixMilli()] = aggregate
		}
		if current.active.Valid {
			active := current.active.Int64
			aggregate.point.ActiveConnections = &active
		}
		if current.delay.Valid {
			delay := current.delay.Int64
			aggregate.point.DelayMs = &delay
		}
		dt := current.ts.Sub(previous.ts).Seconds()
		validGeneration := current.version == previous.version && current.uptime >= previous.uptime
		validCounters := current.upload.Valid && previous.upload.Valid && current.upload.Int64 >= previous.upload.Int64 &&
			current.download.Valid && previous.download.Valid && current.download.Int64 >= previous.download.Int64 &&
			current.connections.Valid && previous.connections.Valid && current.connections.Int64 >= previous.connections.Int64
		if dt > 0 && dt <= 3*sampleInterval.Seconds() && validGeneration && validCounters {
			aggregate.uploadDelta += current.upload.Int64 - previous.upload.Int64
			aggregate.downloadDelta += current.download.Int64 - previous.download.Int64
			aggregate.connectionDelta += current.connections.Int64 - previous.connections.Int64
			aggregate.validSeconds += dt
			aggregate.hasRate = true
		}
	}
	result := make([]model.DimensionTimePoint, 0, int(math.Ceil(float64(to.Sub(from))/float64(step))))
	for bucket := from; bucket.Before(to); bucket = bucket.Add(step) {
		aggregate := buckets[bucket.UnixMilli()]
		if aggregate == nil {
			result = append(result, model.DimensionTimePoint{Timestamp: bucket})
			continue
		}
		if aggregate.hasRate && aggregate.validSeconds > 0 {
			upload := float64(aggregate.uploadDelta) / aggregate.validSeconds
			download := float64(aggregate.downloadDelta) / aggregate.validSeconds
			connections := float64(aggregate.connectionDelta) / aggregate.validSeconds
			if finiteRate(upload) {
				aggregate.point.UploadBytesPerSecond = &upload
			}
			if finiteRate(download) {
				aggregate.point.DownloadBytesPerSecond = &download
			}
			if finiteRate(connections) {
				aggregate.point.ConnectionsPerSecond = &connections
			}
		}
		result = append(result, aggregate.point)
	}
	return result, nil
}

func (s *Store) HistoryFrom(ctx context.Context) (time.Time, error) {
	var ms sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MIN(ts_ms) FROM global_samples").Scan(&ms); err != nil || !ms.Valid {
		if err != nil {
			return time.Time{}, err
		}
		return time.Time{}, sql.ErrNoRows
	}
	return time.UnixMilli(ms.Int64).UTC(), nil
}
func (s *Store) Retention(ctx context.Context, cutoff time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM global_samples WHERE ts_ms<?", cutoff.UnixMilli()); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM dimension_samples WHERE ts_ms<?", cutoff.UnixMilli()); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM connections WHERE state='closed' AND COALESCE(closed_at_ms,last_seen_at_ms)<?", cutoff.UnixMilli()); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type rankAggregate struct {
	Value                                 string
	Upload, Download, Connections, Active int64
}

func (s *Store) Rankings(ctx context.Context, dimension string, from, to time.Time, sortBy string, limit int, active []model.Connection) (model.RankingsResponse, error) {
	if !allowedDimension(dimension) {
		return model.RankingsResponse{}, errors.New("invalid dimension")
	}
	values := map[string]*rankAggregate{}
	rows, err := s.db.QueryContext(ctx, connectionSelect+" WHERE state='closed' AND closed_at_ms>=? AND closed_at_ms<?", from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return model.RankingsResponse{}, err
	}
	for rows.Next() {
		c, e := scanConnection(rows)
		if e != nil {
			rows.Close()
			return model.RankingsResponse{}, e
		}
		addRank(values, dimension, c, false)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return model.RankingsResponse{}, err
	}
	rows.Close()
	for _, c := range active {
		addRank(values, dimension, c, true)
	}
	all := make([]rankAggregate, 0, len(values))
	for _, v := range values {
		all = append(all, *v)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		av, bv := rankMetric(a, sortBy), rankMetric(b, sortBy)
		if av == bv {
			return a.Value < b.Value
		}
		return av > bv
	})
	total := len(all)
	denom := int64(0)
	for _, v := range all {
		denom += rankMetric(v, sortBy)
	}
	data := make([]model.RankingItem, 0, min(limit, len(all)))
	for _, v := range all[:min(limit, len(all))] {
		pct := float64(0)
		if denom > 0 {
			pct = float64(rankMetric(v, sortBy)) * 100 / float64(denom)
		}
		data = append(data, model.RankingItem{Value: v.Value, UploadBytes: v.Upload, DownloadBytes: v.Download, Connections: v.Connections, ActiveConnections: v.Active, Percentage: pct})
	}
	return model.RankingsResponse{GeneratedAt: time.Now().UTC(), Dimension: dimension, Sort: sortBy, Total: total, Data: data}, nil
}
func addRank(values map[string]*rankAggregate, dimension string, c model.Connection, active bool) {
	value := dimensionValue(dimension, c)
	if value == "" {
		return
	}
	v := values[value]
	if v == nil {
		v = &rankAggregate{Value: value}
		values[value] = v
	}
	v.Upload += max0(c.Upload)
	v.Download += max0(c.Download)
	v.Connections++
	if active {
		v.Active++
	}
}
func dimensionValue(d string, c model.Connection) string {
	switch d {
	case "network":
		return c.Network
	case "inbound":
		return c.Inbound
	case "outbound":
		if len(c.Chain) > 0 {
			return strings.Join(c.Chain, " > ")
		}
		if c.Outbound != "" {
			return c.Outbound
		}
		return "unknown"
	case "rule":
		return c.Rule
	case "domain":
		return c.Domain
	case "destination_ip":
		return c.DestinationIP
	case "source":
		if c.SourceHostname != "" {
			return c.SourceHostname
		}
		return c.SourceIP
	case "process":
		return c.Process
	case "user":
		return c.User
	}
	return ""
}
func rankMetric(v rankAggregate, sortBy string) int64 {
	switch sortBy {
	case "connections":
		return v.Connections
	case "download":
		return v.Download
	case "upload":
		return v.Upload
	default:
		return v.Upload + v.Download
	}
}
func allowedDimension(d string) bool {
	switch d {
	case "network", "inbound", "outbound", "rule", "domain", "destination_ip", "source", "process", "user":
		return true
	}
	return false
}
func max0(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
