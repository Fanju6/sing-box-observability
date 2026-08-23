package fakeupstream

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	scenario string
	mu       sync.Mutex
	scrape   int
}

func New(scenario string) *Server { return &Server{scenario: scenario} }
func (f *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/observability/v1/capabilities", f.capabilities)
	mux.HandleFunc("/observability/v1/status", f.status)
	mux.HandleFunc("/observability/v1/metrics", f.metrics)
	mux.HandleFunc("/observability/v1/connections/active", f.active)
	mux.HandleFunc("/observability/v1/connections/recent", f.recent)
	mux.HandleFunc("/observability/v1/events", f.events)
	return mux
}
func (f *Server) unauthorized(w http.ResponseWriter) bool {
	if f.scenario == "401" || f.scenario == "unauthorized" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"unauthorized"},"message":"unauthorized"}`))
		return true
	}
	return false
}
func (f *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	if f.scenario == "malformed-json" {
		w.Write([]byte("{"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	expose := f.scenario == "sensitive" || f.scenario == "sensitive-on"
	fmt.Fprintf(w, `{"apiVersion":1,"endpoints":["capabilities","metrics","status","connections/active","connections/recent","events","top"],"topDimensions":["network","inbound","outbound","rule","domain","destination_ip","source","process","user"],"sensitiveDimensions":["rule","domain","destination_ip","source","process","user"],"exposeSensitive":%t,"recentLimit":1000,"recentTTL":"1h0m0s","topKLimit":100,"activePageLimit":500,"cursorPagination":true,"eventReplay":false}`, expose)
}
func (f *Server) status(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"version":"fake-1.0","uptimeSeconds":100,"memoryBytes":1000,"goroutines":10,"activeConnections":1,"recentConnections":2,"connectionsTotal":10,"uploadBytesTotal":1000,"downloadBytesTotal":2000,"recentConnectionLimit":1000,"recentTTL":"1h0m0s","topKSize":100,"exposeSensitive":false}`)
}
func (f *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	f.mu.Lock()
	f.scrape++
	n := f.scrape
	f.mu.Unlock()
	if f.scenario == "stale" {
		http.Error(w, "temporary failure", http.StatusInternalServerError)
		return
	}
	if f.scenario == "malformed" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("not prometheus"))
		return
	}
	reset := f.scenario == "reset" && n == 5
	base := 1000 + n*100
	if reset {
		base = 10
	}
	connectionsTotal := base
	uploadTotal := base * 2
	downloadTotal := base + 10
	if f.scenario == "zero" {
		connectionsTotal = 0
		uploadTotal = 0
		downloadTotal = 0
		base = 0
	}
	fmt.Fprintf(w, "# TYPE singbox_build_info gauge\nsingbox_build_info{version=\"fake-1.0\",go_version=\"go1.26\",os=\"windows\",arch=\"amd64\"} 1\n# TYPE singbox_uptime_seconds gauge\nsingbox_uptime_seconds %d\n# TYPE singbox_memory_bytes gauge\nsingbox_memory_bytes 1000\n# TYPE singbox_goroutines gauge\nsingbox_goroutines 10\n# TYPE singbox_connections_active gauge\nsingbox_connections_active 1\n# TYPE singbox_recent_connections gauge\nsingbox_recent_connections 2\n# TYPE singbox_recent_connections_capacity gauge\nsingbox_recent_connections_capacity 1000\n# TYPE singbox_connections_total counter\nsingbox_connections_total %d\n# TYPE singbox_traffic_upload_bytes_total counter\nsingbox_traffic_upload_bytes_total %d\n# TYPE singbox_traffic_download_bytes_total counter\nsingbox_traffic_download_bytes_total %d\n# TYPE singbox_outbound_connections_active gauge\nsingbox_outbound_connections_active{outbound=\"direct\"} 1\n# TYPE singbox_outbound_connections_total counter\nsingbox_outbound_connections_total{outbound=\"direct\"} %d\n# TYPE singbox_outbound_upload_bytes_total counter\nsingbox_outbound_upload_bytes_total{outbound=\"direct\"} %d\n# TYPE singbox_outbound_download_bytes_total counter\nsingbox_outbound_download_bytes_total{outbound=\"direct\"} %d\n# TYPE singbox_inbound_connections_active gauge\nsingbox_inbound_connections_active{inbound=\"tun\"} 1\n# TYPE singbox_inbound_connections_total counter\nsingbox_inbound_connections_total{inbound=\"tun\"} %d\n# TYPE singbox_network_connections_active gauge\nsingbox_network_connections_active{network=\"tcp\"} 1\n# TYPE singbox_network_connections_total counter\nsingbox_network_connections_total{network=\"tcp\"} %d\n# TYPE singbox_observability_http_requests_total counter\nsingbox_observability_http_requests_total{endpoint=\"metrics\",status=\"200\"} %d\n# TYPE singbox_observability_http_response_bytes_total counter\nsingbox_observability_http_response_bytes_total{endpoint=\"metrics\",status=\"200\"} %d\n# TYPE singbox_observability_http_request_duration_seconds_total counter\nsingbox_observability_http_request_duration_seconds_total{endpoint=\"metrics\",status=\"200\"} %.6f\n# TYPE singbox_observability_sse_subscribers gauge\nsingbox_observability_sse_subscribers 1\n# TYPE singbox_observability_sse_events_total counter\nsingbox_observability_sse_events_total %d\n", 100+n, connectionsTotal, uploadTotal, downloadTotal, base, base, base, base, base, n, n*4096, float64(n)/100, n*2)
}
func (f *Server) active(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if f.scenario == "sensitive" || f.scenario == "sensitive-on" {
		fmt.Fprintf(w, `{"total":1,"data":[{"id":"fake-1","network":"tcp","inbound":"tun","sourceIP":"192.0.2.1","sourcePort":1234,"destinationIP":"203.0.113.1","destinationPort":443,"domain":"example.com","process":"browser","outbound":"direct","outboundType":"direct","chain":["direct"],"rule":"final","startedAt":%q,"upload":1,"download":2}],"hasMore":false}`, now)
		return
	}
	fmt.Fprintf(w, `{"total":1,"data":[{"id":"fake-1","network":"tcp","inbound":"tun","destinationPort":443,"outbound":"direct","outboundType":"direct","chain":["direct"],"startedAt":%q,"upload":1,"download":2}],"hasMore":false}`, now)
}
func (f *Server) recent(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Query().Has("offset") {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_query_parameter","message":"offset is unsupported","parameter":"offset"},"message":"offset is unsupported"}`))
		return
	}
	now := time.Now().UTC()
	start := now.Add(-time.Minute)
	fmt.Fprintf(w, `{"total":1,"data":[{"id":"fake-closed","network":"tcp","inbound":"tun","destinationPort":443,"outbound":"direct","outboundType":"direct","chain":["direct"],"startedAt":%q,"closedAt":%q,"upload":3,"download":4}],"hasMore":false}`, start.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
}
func (f *Server) events(w http.ResponseWriter, r *http.Request) {
	if f.unauthorized(w) {
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}
	fmt.Fprint(w, ": fake keepalive\n\n")
	flusher.Flush()
	if f.scenario == "reconnect" {
		return
	}
	<-r.Context().Done()
}
func (f *Server) Scrapes() int { f.mu.Lock(); defer f.mu.Unlock(); return f.scrape }
func ParseScenario(value string) string {
	if value == "" {
		return "online"
	}
	return value
}
