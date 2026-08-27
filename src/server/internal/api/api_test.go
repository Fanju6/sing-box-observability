package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/auth"
	"github.com/Fanju6/sing-box-observability/src/server/internal/collector"
	"github.com/Fanju6/sing-box-observability/src/server/internal/config"
	"github.com/Fanju6/sing-box-observability/src/server/internal/events"
	"github.com/Fanju6/sing-box-observability/src/server/internal/fakeupstream"
	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
	"github.com/Fanju6/sing-box-observability/src/server/internal/source"
	"github.com/Fanju6/sing-box-observability/src/server/internal/storage"
)

func TestRedactConnectionsRemovesEverySensitiveField(t *testing.T) {
	connections := []model.Connection{{SourceIP: "192.0.2.1", SourcePort: 1234, SourceMAC: "00:11:22:33:44:55", SourceHostname: "device", DestinationIP: "203.0.113.1", Domain: "example.com", Process: "browser", User: "user", Rule: "final"}}
	redactConnections(connections)
	if connections[0].SourceIP != "" || connections[0].SourcePort != 0 || connections[0].SourceMAC != "" || connections[0].SourceHostname != "" || connections[0].DestinationIP != "" || connections[0].Domain != "" || connections[0].Process != "" || connections[0].User != "" || connections[0].Rule != "" {
		t.Fatalf("sensitive fields remain after redaction: %#v", connections[0])
	}
}

func TestPresetRangeIncludesLongTermWindows(t *testing.T) {
	testCases := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
		"90d": 90 * 24 * time.Hour,
	}
	for value, expected := range testCases {
		actual, ok := presetRange(value)
		if !ok || actual != expected {
			t.Fatalf("presetRange(%q) = %v, %v; want %v, true", value, actual, ok, expected)
		}
	}
}

func TestProtectedAPIAndSensitiveDimension(t *testing.T) {
	upstream := httptest.NewServer(fakeupstream.New("online").Handler())
	defer upstream.Close()
	cfg := config.Default()
	cfg.Singbox.BaseURL = upstream.URL
	cfg.Console.AuthToken = "console-secret"
	cfg.Storage.Path = filepath.Join(t.TempDir(), "obs.db")
	cfg.Collector.ScrapeInterval = 20 * time.Millisecond
	cfg.Collector.PersistInterval = 40 * time.Millisecond
	cfg.Collector.ReconcileInterval = 20 * time.Millisecond
	cfg.Collector.StaleAfter = 200 * time.Millisecond
	store, err := storage.Open(cfg.Storage.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	client, _ := source.NewClient(cfg.Singbox.BaseURL, "")
	c := collector.New(client, store, events.NewHub(), cfg, nil)
	ctx := context.Background()
	defer c.Stop()
	c.Run(ctx)
	time.Sleep(120 * time.Millisecond)
	handler := New(c, store, auth.New(cfg.Console.AuthToken, time.Hour, false), cfg, nil).Handler()
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest("GET", "/api/v1/meta", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status %d body %s", unauth.Code, unauth.Body.String())
	}
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/session", strings.NewReader(`{"token":"console-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login status %d body %s", login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	meta := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/meta", nil)
	req.AddCookie(cookie)
	handler.ServeHTTP(meta, req)
	if meta.Code != http.StatusOK {
		t.Fatalf("meta status %d body %s", meta.Code, meta.Body.String())
	}
	rank := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/rankings?dimension=domain", nil)
	req.AddCookie(cookie)
	handler.ServeHTTP(rank, req)
	if rank.Code != http.StatusForbidden {
		t.Fatalf("rank status %d body %s", rank.Code, rank.Body.String())
	}
	var metaBody map[string]any
	if err := json.Unmarshal(meta.Body.Bytes(), &metaBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := metaBody["token"]; ok {
		t.Fatal("token in response")
	}
	if !strings.Contains(meta.Body.String(), `"exposeSensitive"`) || strings.Contains(meta.Body.String(), `"ExposeSensitive"`) {
		t.Fatalf("capability fields are not camelCase: %s", meta.Body.String())
	}
	if !strings.Contains(meta.Body.String(), `"upstreamApiVersion":1`) || !strings.Contains(meta.Body.String(), `"cursorPagination":true`) {
		t.Fatalf("latest upstream capability contract missing: %s", meta.Body.String())
	}
	overview := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/overview?range=15m", nil)
	req.AddCookie(cookie)
	handler.ServeHTTP(overview, req)
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), `"urlTests":[]`) || !strings.Contains(overview.Body.String(), `"series":[`) || !strings.Contains(overview.Body.String(), `"apiHealth":{"recentConnectionsCapacity":1000`) {
		t.Fatalf("overview required arrays must not be null: status=%d body=%s", overview.Code, overview.Body.String())
	}
	dimensionSeries := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/dimensions/series?dimension=outbound&value=direct&range=15m", nil)
	req.AddCookie(cookie)
	handler.ServeHTTP(dimensionSeries, req)
	if dimensionSeries.Code != http.StatusOK || !strings.Contains(dimensionSeries.Body.String(), `"dimension":"outbound"`) || !strings.Contains(dimensionSeries.Body.String(), `"series":[`) {
		t.Fatalf("dimension series status=%d body=%s", dimensionSeries.Code, dimensionSeries.Body.String())
	}
}
