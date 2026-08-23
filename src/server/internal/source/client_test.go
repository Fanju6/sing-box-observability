package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/fakeupstream"
)

const minimalMetrics = `# TYPE singbox_build_info gauge
singbox_build_info{version="v",go_version="go1.26",os="windows",arch="amd64"} 1
# TYPE singbox_uptime_seconds gauge
singbox_uptime_seconds 10
# TYPE singbox_memory_bytes gauge
singbox_memory_bytes 100
# TYPE singbox_goroutines gauge
singbox_goroutines 2
# TYPE singbox_connections_active gauge
singbox_connections_active 0
# TYPE singbox_recent_connections gauge
singbox_recent_connections 0
# TYPE singbox_recent_connections_capacity gauge
singbox_recent_connections_capacity 1000
# TYPE singbox_connections_total counter
singbox_connections_total 0
# TYPE singbox_traffic_upload_bytes_total counter
singbox_traffic_upload_bytes_total 0
# TYPE singbox_traffic_download_bytes_total counter
singbox_traffic_download_bytes_total 0
# TYPE singbox_observability_http_requests_total counter
singbox_observability_http_requests_total{endpoint="metrics",status="200"} 1
# TYPE singbox_observability_http_response_bytes_total counter
singbox_observability_http_response_bytes_total{endpoint="metrics",status="200"} 100
# TYPE singbox_observability_http_request_duration_seconds_total counter
singbox_observability_http_request_duration_seconds_total{endpoint="metrics",status="200"} 0.01
# TYPE singbox_observability_sse_subscribers gauge
singbox_observability_sse_subscribers 0
# TYPE singbox_observability_sse_events_total counter
singbox_observability_sse_events_total 0
`

func TestParseMetricsAllowsZeroAndRejectsKnownInvalid(t *testing.T) {
	s, err := ParseMetrics(strings.NewReader(minimalMetrics))
	if err != nil || s.UploadBytesTotal != 0 {
		t.Fatalf("zero metrics: %#v %v", s, err)
	}
	bad := strings.Replace(minimalMetrics, "singbox_traffic_upload_bytes_total 0", "singbox_traffic_upload_bytes_total NaN", 1)
	if _, err := ParseMetrics(strings.NewReader(bad)); err == nil {
		t.Fatal("expected NaN rejection")
	}
	bad = strings.Replace(minimalMetrics, "singbox_connections_total 0", "singbox_connections_total -1", 1)
	if _, err := ParseMetrics(strings.NewReader(bad)); err == nil {
		t.Fatal("expected negative counter rejection")
	}
}

func TestParseMetricsSSEMultiLineAndKeepalive(t *testing.T) {
	var got string
	var gotID uint64
	err := parseSSE(strings.NewReader(": ping\r\n\r\nid: 7\r\nevent: open\r\ndata: {\r\ndata: \"type\":\"open\"}\r\n\r\n"), func(name string, id uint64, data []byte) error {
		got = name + ":" + string(data)
		gotID = id
		return nil
	})
	if err == nil || gotID != 7 || got != "open:{\n\"type\":\"open\"}" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestClientURLAndBearerHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/observability/v1/capabilities" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization leaked/missing: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":1,"endpoints":["capabilities","metrics","status","connections/active","connections/recent","events","top"],"topDimensions":["network","inbound","outbound"],"sensitiveDimensions":[],"exposeSensitive":false,"recentLimit":1,"recentTTL":"1h","topKLimit":1,"activePageLimit":500,"cursorPagination":true,"eventReplay":false}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL+"/prefix/observability/v1/", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Capabilities(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClientUnauthorizedClassification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret")
	_, err := client.Capabilities(context.Background())
	if ErrorCode(err) != "UPSTREAM_UNAUTHORIZED" {
		t.Fatalf("error %v", err)
	}
}

func TestClientParsesStableUpstreamErrorDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_query_parameter","message":"limit is too large","parameter":"limit","maximum":"500"},"message":"limit is too large"}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "")
	_, err := client.RecentPage(context.Background(), time.Hour, 501, "")
	var upstream *Error
	if !errors.As(err, &upstream) || upstream.Code != "UPSTREAM_INVALID_RESPONSE" || upstream.UpstreamCode != "invalid_query_parameter" || upstream.Parameter != "limit" || upstream.Maximum != "500" {
		t.Fatalf("structured upstream error was not preserved safely: %#v", upstream)
	}
}

func TestFakeUpstreamFailureClassificationsAndZeroCounters(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scenario string
		call     func(*Client) error
		wantCode string
	}{
		{name: "unauthorized", scenario: "401", call: func(c *Client) error { _, err := c.Capabilities(context.Background()); return err }, wantCode: "UPSTREAM_UNAUTHORIZED"},
		{name: "metrics unavailable", scenario: "stale", call: func(c *Client) error { _, err := c.Metrics(context.Background()); return err }, wantCode: "UPSTREAM_UNAVAILABLE"},
		{name: "malformed metrics", scenario: "malformed", call: func(c *Client) error { _, err := c.Metrics(context.Background()); return err }, wantCode: "UPSTREAM_INVALID_RESPONSE"},
		{name: "malformed json", scenario: "malformed-json", call: func(c *Client) error { _, err := c.Capabilities(context.Background()); return err }, wantCode: "UPSTREAM_INVALID_RESPONSE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(fakeupstream.New(tc.scenario).Handler())
			defer server.Close()
			client, err := NewClient(server.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			if code := ErrorCode(tc.call(client)); code != tc.wantCode {
				t.Fatalf("error code %q, want %q", code, tc.wantCode)
			}
		})
	}

	server := httptest.NewServer(fakeupstream.New("zero").Handler())
	defer server.Close()
	client, err := NewClient(server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := client.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.ConnectionsTotal != 0 || metrics.UploadBytesTotal != 0 || metrics.DownloadBytesTotal != 0 {
		t.Fatalf("zero counters were not preserved: %#v", metrics)
	}
	if metrics.RecentConnectionsCapacity != 1000 || metrics.SSESubscribers != 1 || len(metrics.API) != 1 || metrics.API[0].Endpoint != "metrics" || metrics.API[0].Status != 200 {
		t.Fatalf("new observability health metrics were not parsed: %#v", metrics)
	}
}

func TestClientRecentKeepsQueryInURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/observability/v1/connections/recent" || r.URL.Query().Get("window") != "1h" {
			t.Errorf("request path/query %s %s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Has("offset") {
			t.Errorf("legacy offset query is forbidden: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"total":0,"data":[],"hasMore":false}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "")
	if _, err := client.RecentPage(context.Background(), time.Hour, 10, ""); err != nil {
		t.Fatal(err)
	}
}

func TestClientActiveConsumesEveryCursorPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "2" || r.URL.Query().Has("offset") {
			t.Errorf("unexpected active query %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		connection := func(id string) string {
			return `{"id":"` + id + `","network":"tcp","destinationPort":443,"startedAt":"2026-08-21T00:00:00Z","upload":1,"download":2}`
		}
		if r.URL.Query().Get("cursor") == "" {
			_, _ = w.Write([]byte(`{"total":3,"data":[` + connection("c1") + `,` + connection("c2") + `],"nextCursor":"next-2","hasMore":true}`))
			return
		}
		if r.URL.Query().Get("cursor") != "next-2" {
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
		_, _ = w.Write([]byte(`{"total":3,"data":[` + connection("c3") + `],"hasMore":false}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "")
	connections, err := client.Active(context.Background(), 2)
	if err != nil || len(connections) != 3 || connections[2].ID != "c3" {
		t.Fatalf("connections=%#v err=%v", connections, err)
	}
}

func TestCapabilitiesRequiresCursorProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":1,"endpoints":["capabilities","metrics","connections/active","connections/recent","events"],"topDimensions":["network"],"sensitiveDimensions":[],"recentLimit":1,"recentTTL":"1h","topKLimit":1,"activePageLimit":500,"cursorPagination":false,"eventReplay":false}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "")
	if _, err := client.Capabilities(context.Background()); ErrorCode(err) != "UPSTREAM_INVALID_RESPONSE" {
		t.Fatalf("expected latest-only protocol rejection, got %v", err)
	}
}

func TestStreamEventsSignalsOpenBeforeDeliveringNamedEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("id: 9\nevent: open\ndata: {\"id\":9,\"type\":\"open\",\"connection\":{\"id\":\"c1\",\"network\":\"tcp\",\"inbound\":\"tun\",\"destinationPort\":443,\"outbound\":\"direct\",\"outboundType\":\"direct\",\"chain\":[\"direct\"],\"startedAt\":\"2026-08-21T00:00:00Z\",\"upload\":1,\"download\":2}}\n\n"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "")
	opened := false
	delivered := false
	err := client.StreamEvents(context.Background(), func() { opened = true }, func(event Event) error {
		if !opened {
			t.Fatal("event delivered before stream-open callback")
		}
		delivered = event.ID == 9 && event.Type == "open" && event.Connection.ID == "c1"
		return nil
	})
	if !opened || !delivered || ErrorCode(err) != "UPSTREAM_UNAVAILABLE" {
		t.Fatalf("opened=%v delivered=%v err=%v", opened, delivered, err)
	}
}
