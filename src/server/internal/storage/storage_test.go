package storage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
)

func TestOpenPreservesExistingParentPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory permission semantics")
	}
	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0755); err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(parent, "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0755 {
		t.Fatalf("existing parent permissions = %o, want 755", got)
	}
}

func TestOpenSecuresNewStorageDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX directory permission semantics")
	}
	directory := filepath.Join(t.TempDir(), "new", "storage")
	store, err := Open(filepath.Join(directory, "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0700 {
		t.Fatalf("new storage directory permissions = %o, want 700", got)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "obs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrationWALUpsertAndBoundedSearch(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	started := time.Now().UTC().Add(-time.Minute)
	closed := started.Add(time.Second)
	c := model.Connection{ID: "c1", State: "active", Network: "tcp", Inbound: "tun", SourceIP: "192.0.2.1", DestinationPort: 443, Outbound: "direct", OutboundType: "direct", Chain: []string{"direct"}, StartedAt: started, Upload: 1, Download: 2}
	if err := s.UpsertConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	c.State = "closed"
	c.ClosedAt = &closed
	c = c.WithDuration(time.Now().UTC())
	c.SourceIP = ""
	if err := s.UpsertConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	data, total, err := s.ListRecent(ctx, started.Add(-time.Second), closed.Add(time.Second), "", "", "", true, 50, 0)
	if err != nil || total != 1 || len(data) != 1 || data[0].SourceIP != "192.0.2.1" {
		t.Fatalf("recent %#v total %d err %v", data, total, err)
	}
	data, total, err = s.ListRecent(ctx, started.Add(-time.Second), closed.Add(time.Second), "%' OR 1=1 --", "", "", true, 50, 0)
	if err != nil || total != 0 || len(data) != 0 {
		t.Fatalf("search injection %#v total %d err %v", data, total, err)
	}
	data, total, err = s.ListRecent(ctx, started.Add(-time.Second), closed.Add(time.Second), "192.0.2.1", "", "", false, 50, 0)
	if err != nil || total != 0 || len(data) != 0 {
		t.Fatalf("sensitive search leaked while disabled: %#v total %d err %v", data, total, err)
	}
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal mode %s", mode)
	}
}

func TestHistorySkipsResetAndKeepsNonNegativeTotals(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Add(-time.Minute)
	put := func(at time.Time, up int64) {
		err := s.InsertSnapshot(ctx, model.MetricSnapshot{ObservedAt: at, Version: "v", UptimeSeconds: at.Sub(base).Seconds(), MemoryBytes: 10, Goroutines: 2, ActiveConnections: 1, ConnectionsTotal: up, UploadBytesTotal: up, DownloadBytesTotal: up}, nil, model.StateOnline)
		if err != nil {
			t.Fatal(err)
		}
	}
	put(base, 0)
	put(base.Add(time.Second), 0)
	put(base.Add(2*time.Second), 100)
	put(base.Add(3*time.Second), 10)
	put(base.Add(4*time.Second), 10)
	h, err := s.History(ctx, base.Add(-time.Second), base.Add(5*time.Second), time.Second, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if h.Totals == nil || h.Totals.UploadBytes != 100 || h.Totals.Connections != 100 {
		t.Fatalf("history totals %#v", h.Totals)
	}
	if len(h.Series) == 0 {
		t.Fatal("expected series")
	}
}

func TestRetentionRemovesOldSamples(t *testing.T) {
	s := testStore(t)
	old := time.Now().UTC().Add(-2 * time.Hour)
	if err := s.InsertSnapshot(context.Background(), model.MetricSnapshot{ObservedAt: old, Version: "v", UptimeSeconds: 1, MemoryBytes: 1, Goroutines: 1, ActiveConnections: 0, ConnectionsTotal: 0, UploadBytesTotal: 0, DownloadBytesTotal: 0}, nil, model.StateOnline); err != nil {
		t.Fatal(err)
	}
	if err := s.Retention(context.Background(), time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.HistoryFrom(context.Background()); err == nil {
		t.Fatal("expected no history")
	}
}

func TestHistoryAggregatesWholeBucketAndEmitsGap(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	put := func(second int, upload int64) {
		at := base.Add(time.Duration(second) * time.Second)
		if err := s.InsertSnapshot(ctx, model.MetricSnapshot{ObservedAt: at, Version: "v", UptimeSeconds: float64(100 + second), MemoryBytes: 10, Goroutines: 2, ActiveConnections: 1, ConnectionsTotal: upload, UploadBytesTotal: upload, DownloadBytesTotal: upload * 2}, nil, model.StateOnline); err != nil {
			t.Fatal(err)
		}
	}
	put(0, 0)
	put(1, 10)
	put(2, 30)
	put(6, 50)

	history, err := s.History(ctx, base, base.Add(8*time.Second), 2*time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Series) != 4 {
		t.Fatalf("series length = %d, want 4", len(history.Series))
	}
	if history.Series[0].UploadBytesPerSecond == nil || *history.Series[0].UploadBytesPerSecond != 10 {
		t.Fatalf("first bucket rate = %v, want 10", history.Series[0].UploadBytesPerSecond)
	}
	if history.Series[1].UploadBytesPerSecond == nil || *history.Series[1].UploadBytesPerSecond != 20 {
		t.Fatalf("second bucket rate = %v, want 20", history.Series[1].UploadBytesPerSecond)
	}
	if history.Series[2].ActiveConnections != nil || history.Series[2].UploadBytesPerSecond != nil {
		t.Fatalf("gap bucket should contain null metrics: %#v", history.Series[2])
	}
}

func TestHistoryUsesPersistenceIntervalForDefaultConfiguration(t *testing.T) {
	s := testStore(t)
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	for index, total := range []int64{100, 250} {
		at := base.Add(time.Duration(index) * 15 * time.Second)
		if err := s.InsertSnapshot(context.Background(), model.MetricSnapshot{ObservedAt: at, Version: "v", UptimeSeconds: float64(100 + index*15), MemoryBytes: 1, Goroutines: 1, ConnectionsTotal: total, UploadBytesTotal: total, DownloadBytesTotal: total}, nil, model.StateOnline); err != nil {
			t.Fatal(err)
		}
	}
	history, err := s.History(context.Background(), base, base.Add(30*time.Second), 15*time.Second, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if history.Totals == nil || history.Totals.UploadBytes != 150 {
		t.Fatalf("default persistence delta was rejected: %#v", history.Totals)
	}
}

func TestDimensionHistoryCalculatesRatesAndPreservesDelay(t *testing.T) {
	s := testStore(t)
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	for index, total := range []int64{100, 250, 400} {
		at := base.Add(time.Duration(index) * 15 * time.Second)
		delay := int64(40 + index)
		dimensions := []model.DimensionSnapshot{{Kind: "outbound", Value: "direct", Active: int64(index + 1), Connections: total / 10, UploadTotal: total, DownloadTotal: total * 2, DelayMs: &delay}}
		snapshot := model.MetricSnapshot{ObservedAt: at, Version: "v", UptimeSeconds: float64(100 + index*15), MemoryBytes: 1, Goroutines: 1, ConnectionsTotal: total, UploadBytesTotal: total, DownloadBytesTotal: total * 2}
		if err := s.InsertSnapshot(context.Background(), snapshot, dimensions, model.StateOnline); err != nil {
			t.Fatal(err)
		}
	}
	series, err := s.DimensionHistory(context.Background(), "outbound", "direct", base, base.Add(45*time.Second), 15*time.Second, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 3 || series[1].UploadBytesPerSecond == nil || *series[1].UploadBytesPerSecond != 10 {
		t.Fatalf("dimension series %#v", series)
	}
	if series[2].DelayMs == nil || *series[2].DelayMs != 42 || series[2].ActiveConnections == nil || *series[2].ActiveConnections != 3 {
		t.Fatalf("dimension point %#v", series[2])
	}
}
