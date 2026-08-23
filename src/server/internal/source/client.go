package source

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
	"github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	prometheusmodel "github.com/prometheus/common/model"
)

const (
	maxJSONBody    = 16 << 20
	maxMetricsBody = 16 << 20
	maxErrorBody   = 64 << 10
	maxSSEEvent    = 2 << 20
)

type Error struct {
	Code         string
	StatusCode   int
	UpstreamCode string
	Parameter    string
	Maximum      string
	Err          error
}

func (e *Error) Error() string { return e.Code }
func (e *Error) Unwrap() error { return e.Err }

func ErrorCode(err error) string {
	var upstream *Error
	if errors.As(err, &upstream) {
		return upstream.Code
	}
	return "UPSTREAM_UNAVAILABLE"
}

type Client struct {
	baseURL *url.URL
	token   string
	http    *http.Client
	sse     *http.Client
}

func NewClient(rawBaseURL, token string) (*Client, error) {
	u, err := url.Parse(rawBaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("invalid upstream URL")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	suffix := "/observability/v1"
	if strings.HasSuffix(u.Path, suffix) {
		u.Path = strings.TrimSuffix(u.Path, suffix)
	}
	return &Client{baseURL: u, token: token, http: &http.Client{}, sse: &http.Client{}}, nil
}

func (c *Client) endpoint(path string) string {
	u := *c.baseURL
	pathPart, rawQuery, _ := strings.Cut(path, "?")
	u.Path = strings.TrimRight(u.Path, "/") + "/observability/v1" + pathPart
	u.RawQuery = rawQuery
	u.RawPath = ""
	return u.String()
}

type Capabilities struct {
	APIVersion          int      `json:"apiVersion"`
	Endpoints           []string `json:"endpoints"`
	TopDimensions       []string `json:"topDimensions"`
	SensitiveDimensions []string `json:"sensitiveDimensions"`
	ExposeSensitive     bool     `json:"exposeSensitive"`
	RecentLimit         int      `json:"recentLimit"`
	RecentTTL           string   `json:"recentTTL"`
	TopKLimit           int      `json:"topKLimit"`
	ActivePageLimit     int      `json:"activePageLimit"`
	CursorPagination    bool     `json:"cursorPagination"`
	EventReplay         bool     `json:"eventReplay"`
}

type Connection struct {
	ID              string     `json:"id"`
	Network         string     `json:"network"`
	Inbound         string     `json:"inbound"`
	SourceIP        string     `json:"sourceIP,omitempty"`
	SourcePort      int        `json:"sourcePort,omitempty"`
	SourceMAC       string     `json:"sourceMAC,omitempty"`
	SourceHostname  string     `json:"sourceHostname,omitempty"`
	DestinationIP   string     `json:"destinationIP,omitempty"`
	DestinationPort int        `json:"destinationPort"`
	Domain          string     `json:"domain,omitempty"`
	Process         string     `json:"process,omitempty"`
	User            string     `json:"user,omitempty"`
	Outbound        string     `json:"outbound"`
	OutboundType    string     `json:"outboundType"`
	Chain           []string   `json:"chain"`
	Rule            string     `json:"rule,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	ClosedAt        *time.Time `json:"closedAt,omitempty"`
	Upload          int64      `json:"upload"`
	Download        int64      `json:"download"`
}

type ConnectionPage struct {
	Data       []Connection `json:"data"`
	Total      int          `json:"total"`
	NextCursor string       `json:"nextCursor,omitempty"`
	HasMore    bool         `json:"hasMore"`
}

func (c *Client) Capabilities(ctx context.Context) (Capabilities, error) {
	var capabilities Capabilities
	if err := c.getJSON(ctx, "/capabilities", &capabilities, 5*time.Second); err != nil {
		return capabilities, err
	}
	if err := validateCapabilities(capabilities); err != nil {
		return capabilities, invalid(err.Error())
	}
	return capabilities, nil
}

func validateCapabilities(capabilities Capabilities) error {
	if capabilities.APIVersion != 1 {
		return fmt.Errorf("unsupported observability API version %d", capabilities.APIVersion)
	}
	if capabilities.RecentLimit < 1 || capabilities.TopKLimit < 1 || capabilities.ActivePageLimit < 1 || capabilities.ActivePageLimit > 500 {
		return errors.New("invalid capability limits")
	}
	if ttl, err := time.ParseDuration(capabilities.RecentTTL); err != nil || ttl <= 0 {
		return errors.New("invalid recent TTL capability")
	}
	if !capabilities.CursorPagination {
		return errors.New("cursor pagination is required")
	}
	required := map[string]bool{"capabilities": false, "metrics": false, "status": false, "connections/active": false, "connections/recent": false, "events": false, "top": false}
	seen := make(map[string]bool, len(capabilities.Endpoints))
	for _, endpoint := range capabilities.Endpoints {
		if endpoint == "" || seen[endpoint] {
			return errors.New("invalid capabilities endpoint list")
		}
		seen[endpoint] = true
		if _, ok := required[endpoint]; ok {
			required[endpoint] = true
		}
	}
	for endpoint, present := range required {
		if !present {
			return fmt.Errorf("required endpoint %s is unavailable", endpoint)
		}
	}
	top := make(map[string]bool, len(capabilities.TopDimensions))
	for _, dimension := range capabilities.TopDimensions {
		if dimension == "" || top[dimension] {
			return errors.New("invalid top dimension list")
		}
		top[dimension] = true
	}
	seenSensitive := make(map[string]bool, len(capabilities.SensitiveDimensions))
	for _, dimension := range capabilities.SensitiveDimensions {
		if dimension == "" || !top[dimension] || seenSensitive[dimension] {
			return errors.New("invalid sensitive dimension list")
		}
		seenSensitive[dimension] = true
	}
	return nil
}

func (c *Client) Metrics(ctx context.Context) (model.MetricSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/metrics"), nil)
	if err != nil {
		return model.MetricSnapshot{}, &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	c.setAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return model.MetricSnapshot{}, &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.MetricSnapshot{}, upstreamHTTPError(resp)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && !strings.Contains(strings.ToLower(contentType), "text/plain") && !strings.Contains(strings.ToLower(contentType), "openmetrics") {
		return model.MetricSnapshot{}, invalid("invalid metrics content type")
	}
	body, err := readLimited(resp.Body, maxMetricsBody)
	if err != nil {
		return model.MetricSnapshot{}, invalid(err.Error())
	}
	snapshot, err := ParseMetrics(bytes.NewReader(body))
	if err != nil {
		return model.MetricSnapshot{}, invalid(err.Error())
	}
	snapshot.ObservedAt = time.Now().UTC()
	return snapshot, nil
}

func (c *Client) Active(ctx context.Context, pageLimit int) ([]Connection, error) {
	if pageLimit < 1 || pageLimit > 500 {
		return nil, invalid("invalid active page limit")
	}
	connections := make([]Connection, 0)
	seenConnections := make(map[string]bool)
	seenCursors := make(map[string]bool)
	cursor := ""
	for {
		path := "/connections/active?limit=" + strconv.Itoa(pageLimit)
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page ConnectionPage
		if err := c.getJSON(ctx, path, &page, 10*time.Second); err != nil {
			return nil, err
		}
		if err := validatePage(page, pageLimit); err != nil {
			return nil, invalid(err.Error())
		}
		for i := range page.Data {
			page.Data[i].ClosedAt = nil
			if err := validateConnection(page.Data[i]); err != nil {
				return nil, invalid(err.Error())
			}
			if seenConnections[page.Data[i].ID] {
				return nil, invalid("duplicate active connection across cursor pages")
			}
			seenConnections[page.Data[i].ID] = true
			connections = append(connections, page.Data[i])
		}
		if !page.HasMore {
			return connections, nil
		}
		if seenCursors[page.NextCursor] {
			return nil, invalid("active cursor cycle")
		}
		seenCursors[page.NextCursor] = true
		cursor = page.NextCursor
		if len(connections) > 100_000 {
			return nil, invalid("too many active connections")
		}
	}
}

func (c *Client) RecentPage(ctx context.Context, window time.Duration, limit int, cursor string) (ConnectionPage, error) {
	path := "/connections/recent?window=" + url.QueryEscape(shortDuration(window)) + "&limit=" + strconv.Itoa(limit)
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}
	var page ConnectionPage
	if err := c.getJSON(ctx, path, &page, 10*time.Second); err != nil {
		return page, err
	}
	if err := validatePage(page, limit); err != nil {
		return page, invalid(err.Error())
	}
	for i := range page.Data {
		if page.Data[i].ClosedAt != nil && page.Data[i].ClosedAt.IsZero() {
			page.Data[i].ClosedAt = nil
		}
		if err := validateConnection(page.Data[i]); err != nil {
			return page, invalid(err.Error())
		}
	}
	return page, nil
}

func validatePage(page ConnectionPage, limit int) error {
	if page.Total < 0 || len(page.Data) > limit {
		return errors.New("invalid connection page")
	}
	if page.HasMore && page.NextCursor == "" {
		return errors.New("missing next cursor")
	}
	if page.HasMore && len(page.Data) == 0 {
		return errors.New("empty connection page has more results")
	}
	if !page.HasMore && page.NextCursor != "" {
		return errors.New("unexpected next cursor")
	}
	return nil
}

func validateConnection(c Connection) error {
	if c.ID == "" || c.StartedAt.IsZero() || c.Network == "" || c.DestinationPort < 0 || c.DestinationPort > 65535 || c.SourcePort < 0 || c.SourcePort > 65535 || c.Upload < 0 || c.Download < 0 {
		return errors.New("invalid upstream connection")
	}
	if c.ClosedAt != nil && c.ClosedAt.Before(c.StartedAt) {
		return errors.New("connection closed before it started")
	}
	return nil
}

type Event struct {
	ID         uint64
	Type       string
	Connection Connection
}

func (c *Client) StreamEvents(ctx context.Context, onOpen func(), fn func(Event) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/events?heartbeat=15s"), nil)
	if err != nil {
		return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	c.setAuth(req)
	resp, err := c.sse.Do(req)
	if err != nil {
		return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamHTTPError(resp)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return invalid("invalid SSE content type")
	}
	if onOpen != nil {
		onOpen()
	}
	err = parseSSE(resp.Body, func(eventName string, eventID uint64, data []byte) error {
		if len(data) == 0 {
			return nil
		}
		var envelope struct {
			ID         uint64     `json:"id"`
			Type       string     `json:"type"`
			Connection Connection `json:"connection"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return &Error{Code: "UPSTREAM_INVALID_RESPONSE", Err: err}
		}
		kind := envelope.Type
		if kind != "open" && kind != "close" {
			return invalid("invalid upstream event type")
		}
		if eventName != kind || eventID == 0 || envelope.ID != eventID {
			return invalid("inconsistent upstream event identity")
		}
		if envelope.Connection.ClosedAt != nil && envelope.Connection.ClosedAt.IsZero() {
			envelope.Connection.ClosedAt = nil
		}
		if err := validateConnection(envelope.Connection); err != nil {
			return invalid(err.Error())
		}
		return fn(Event{ID: eventID, Type: kind, Connection: envelope.Connection})
	})
	return err
}

func (c *Client) getJSON(ctx context.Context, path string, out any, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	c.setAuth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return upstreamHTTPError(resp)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return invalid("invalid JSON content type")
	}
	body, err := readLimited(resp.Body, maxJSONBody)
	if err != nil {
		return invalid(err.Error())
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return invalid("empty upstream response")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return invalid("malformed upstream JSON")
	}
	return nil
}

func upstreamHTTPError(resp *http.Response) error {
	code := "UPSTREAM_UNAVAILABLE"
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		code = "UPSTREAM_UNAUTHORIZED"
	} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		code = "UPSTREAM_INVALID_RESPONSE"
	}
	body, readErr := readLimited(resp.Body, maxErrorBody)
	if readErr != nil {
		return &Error{Code: code, StatusCode: resp.StatusCode, Err: readErr}
	}
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Parameter string `json:"parameter,omitempty"`
			Maximum   string `json:"maximum,omitempty"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || payload.Error.Code == "" || payload.Error.Message == "" {
		return &Error{Code: code, StatusCode: resp.StatusCode, Err: errors.New("malformed upstream API error")}
	}
	return &Error{Code: code, StatusCode: resp.StatusCode, UpstreamCode: payload.Error.Code, Parameter: payload.Error.Parameter, Maximum: payload.Error.Maximum, Err: errors.New(payload.Error.Message)}
}

func (c *Client) setAuth(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func readLimited(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errors.New("upstream response is too large")
	}
	return b, nil
}

func invalid(message string) error {
	return &Error{Code: "UPSTREAM_INVALID_RESPONSE", Err: errors.New(message)}
}

func shortDuration(d time.Duration) string {
	if d%time.Hour == 0 {
		return strconv.FormatInt(int64(d/time.Hour), 10) + "h"
	}
	if d%time.Minute == 0 {
		return strconv.FormatInt(int64(d/time.Minute), 10) + "m"
	}
	return strconv.FormatInt(int64(d/time.Second), 10) + "s"
}

func ParseMetrics(r io.Reader) (model.MetricSnapshot, error) {
	parser := expfmt.NewTextParser(prometheusmodel.LegacyValidation)
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	get := func(name string) (*io_prometheus_client.MetricFamily, error) {
		family := families[name]
		if family == nil {
			return nil, fmt.Errorf("missing metric %s", name)
		}
		return family, nil
	}
	global := func(name string) (float64, error) {
		family, err := get(name)
		if err != nil {
			return 0, err
		}
		if len(family.Metric) != 1 || len(family.Metric[0].Label) != 0 {
			return 0, fmt.Errorf("invalid labels for %s", name)
		}
		return metricValue(family.Metric[0])
	}
	build, err := get("singbox_build_info")
	if err != nil || len(build.Metric) != 1 {
		if err == nil {
			err = errors.New("invalid build info")
		}
		return model.MetricSnapshot{}, err
	}
	if _, err := metricValue(build.Metric[0]); err != nil {
		return model.MetricSnapshot{}, err
	}
	labels := labelMap(build.Metric[0])
	if len(labels) != 4 {
		return model.MetricSnapshot{}, errors.New("invalid build info labels")
	}
	for _, key := range []string{"version", "go_version", "os", "arch"} {
		if labels[key] == "" {
			return model.MetricSnapshot{}, errors.New("missing build info label")
		}
	}
	uptime, err := global("singbox_uptime_seconds")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	memory, err := global("singbox_memory_bytes")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	goroutines, err := global("singbox_goroutines")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	active, err := global("singbox_connections_active")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	recent, err := global("singbox_recent_connections")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	recentCapacity, err := global("singbox_recent_connections_capacity")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	connections, err := global("singbox_connections_total")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	upload, err := global("singbox_traffic_upload_bytes_total")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	download, err := global("singbox_traffic_download_bytes_total")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	sseSubscribers, err := global("singbox_observability_sse_subscribers")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	sseEvents, err := global("singbox_observability_sse_events_total")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	values := []float64{uptime, memory, goroutines, active, recent, recentCapacity, connections, upload, download, sseSubscribers, sseEvents}
	for _, v := range values {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return model.MetricSnapshot{}, errors.New("invalid negative or non-finite metric")
		}
	}
	toInt := func(v float64) (int64, error) {
		if math.Trunc(v) != v || v >= float64(math.MaxInt64) {
			return 0, errors.New("metric integer overflow")
		}
		return int64(v), nil
	}
	memoryI, err := toInt(memory)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	goroutinesI, err := toInt(goroutines)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	activeI, err := toInt(active)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	recentI, err := toInt(recent)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	recentCapacityI, err := toInt(recentCapacity)
	if err != nil || recentCapacityI < 1 {
		return model.MetricSnapshot{}, errors.New("invalid recent connections capacity")
	}
	if recentI > recentCapacityI {
		return model.MetricSnapshot{}, errors.New("recent connections exceed capacity")
	}
	connectionsI, err := toInt(connections)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	uploadI, err := toInt(upload)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	downloadI, err := toInt(download)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	sseSubscribersI, err := toInt(sseSubscribers)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	sseEventsI, err := toInt(sseEvents)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	s := model.MetricSnapshot{Version: labels["version"], UptimeSeconds: uptime, MemoryBytes: memoryI, Goroutines: goroutinesI, ActiveConnections: activeI, RecentConnections: recentI, RecentConnectionsCapacity: recentCapacityI, ConnectionsTotal: connectionsI, UploadBytesTotal: uploadI, DownloadBytesTotal: downloadI, SSESubscribers: sseSubscribersI, SSEEventsTotal: sseEventsI}
	api, err := parseAPIMetrics(families, toInt)
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	s.API = api
	for _, spec := range []struct{ kind, active, total, up, down string }{{"outbound", "singbox_outbound_connections_active", "singbox_outbound_connections_total", "singbox_outbound_upload_bytes_total", "singbox_outbound_download_bytes_total"}, {"inbound", "singbox_inbound_connections_active", "singbox_inbound_connections_total", "", ""}, {"network", "singbox_network_connections_active", "singbox_network_connections_total", "", ""}} {
		activeMap, err := labeledMetrics(families[spec.active], spec.kind)
		if err != nil {
			return model.MetricSnapshot{}, err
		}
		totalMap, err := labeledMetrics(families[spec.total], spec.kind)
		if err != nil {
			return model.MetricSnapshot{}, err
		}
		upMap, err := labeledMetrics(families[spec.up], "outbound")
		if err != nil {
			return model.MetricSnapshot{}, err
		}
		downMap, err := labeledMetrics(families[spec.down], "outbound")
		if err != nil {
			return model.MetricSnapshot{}, err
		}
		keys := map[string]bool{}
		for k := range activeMap {
			keys[k] = true
		}
		for k := range totalMap {
			keys[k] = true
		}
		for k := range upMap {
			keys[k] = true
		}
		for k := range downMap {
			keys[k] = true
		}
		for key := range keys {
			d := model.DimensionSnapshot{Kind: spec.kind, Value: key}
			if v, ok := activeMap[key]; ok {
				d.Active, err = toInt(v)
			}
			if err != nil {
				return model.MetricSnapshot{}, err
			}
			if v, ok := totalMap[key]; ok {
				d.Connections, err = toInt(v)
			}
			if err != nil {
				return model.MetricSnapshot{}, err
			}
			if v, ok := upMap[key]; ok {
				d.UploadTotal, err = toInt(v)
			}
			if err != nil {
				return model.MetricSnapshot{}, err
			}
			if v, ok := downMap[key]; ok {
				d.DownloadTotal, err = toInt(v)
			}
			if err != nil {
				return model.MetricSnapshot{}, err
			}
			s.Dimensions = append(s.Dimensions, d)
		}
	}
	delayMap, err := labeledMetrics(families["singbox_outbound_urltest_delay_milliseconds"], "outbound")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	timestampMap, err := labeledMetrics(families["singbox_outbound_urltest_timestamp_seconds"], "outbound")
	if err != nil {
		return model.MetricSnapshot{}, err
	}
	for key, delay := range delayMap {
		d, err := toInt(delay)
		if err != nil || d < 0 {
			return model.MetricSnapshot{}, errors.New("invalid urltest delay")
		}
		ts, ok := timestampMap[key]
		if !ok {
			continue
		}
		t, err := toInt(ts)
		if err != nil || t < 0 {
			return model.MetricSnapshot{}, errors.New("invalid urltest timestamp")
		}
		measured := time.Unix(t, 0).UTC()
		s.URLTests = append(s.URLTests, model.URLTestResult{Outbound: key, DelayMs: d, MeasuredAt: measured})
	}
	return s, nil
}

type apiMetricKey struct {
	endpoint string
	status   int
}

func parseAPIMetrics(families map[string]*io_prometheus_client.MetricFamily, toInt func(float64) (int64, error)) ([]model.APIMetricCounter, error) {
	read := func(name string) (map[apiMetricKey]float64, error) {
		family := families[name]
		if family == nil {
			return nil, fmt.Errorf("missing metric %s", name)
		}
		values := make(map[apiMetricKey]float64, len(family.Metric))
		for _, metric := range family.Metric {
			labels := labelMap(metric)
			if len(labels) != 2 || labels["endpoint"] == "" || len(labels["endpoint"]) > 128 {
				return nil, fmt.Errorf("invalid labels for %s", name)
			}
			status, err := strconv.Atoi(labels["status"])
			if err != nil || status < 100 || status > 599 {
				return nil, fmt.Errorf("invalid status label for %s", name)
			}
			value, err := metricValue(metric)
			if err != nil || value < 0 {
				return nil, fmt.Errorf("invalid value for %s", name)
			}
			key := apiMetricKey{endpoint: labels["endpoint"], status: status}
			if _, exists := values[key]; exists {
				return nil, fmt.Errorf("duplicate labels for %s", name)
			}
			values[key] = value
		}
		return values, nil
	}
	requests, err := read("singbox_observability_http_requests_total")
	if err != nil {
		return nil, err
	}
	bytes, err := read("singbox_observability_http_response_bytes_total")
	if err != nil {
		return nil, err
	}
	durations, err := read("singbox_observability_http_request_duration_seconds_total")
	if err != nil {
		return nil, err
	}
	if len(requests) != len(bytes) || len(requests) != len(durations) {
		return nil, errors.New("inconsistent observability API metric labels")
	}
	if len(requests) > 128 {
		return nil, errors.New("too many observability API metric series")
	}
	result := make([]model.APIMetricCounter, 0, len(requests))
	for key, requestValue := range requests {
		byteValue, hasBytes := bytes[key]
		durationValue, hasDuration := durations[key]
		if !hasBytes || !hasDuration {
			return nil, errors.New("inconsistent observability API metric labels")
		}
		requestTotal, err := toInt(requestValue)
		if err != nil {
			return nil, err
		}
		responseBytesTotal, err := toInt(byteValue)
		if err != nil {
			return nil, err
		}
		result = append(result, model.APIMetricCounter{Endpoint: key.endpoint, Status: key.status, RequestsTotal: requestTotal, ResponseBytesTotal: responseBytesTotal, DurationSecondsTotal: durationValue})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Endpoint == result[j].Endpoint {
			return result[i].Status < result[j].Status
		}
		return result[i].Endpoint < result[j].Endpoint
	})
	return result, nil
}

func metricValue(m *io_prometheus_client.Metric) (float64, error) {
	var v float64
	switch {
	case m.Gauge != nil:
		v = m.GetGauge().GetValue()
	case m.Counter != nil:
		v = m.GetCounter().GetValue()
	case m.Untyped != nil:
		v = m.GetUntyped().GetValue()
	default:
		return 0, errors.New("metric has no numeric value")
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("non-finite metric")
	}
	return v, nil
}
func labelMap(m *io_prometheus_client.Metric) map[string]string {
	out := make(map[string]string, len(m.Label))
	for _, l := range m.Label {
		out[l.GetName()] = l.GetValue()
	}
	return out
}
func labeledMetrics(f *io_prometheus_client.MetricFamily, label string) (map[string]float64, error) {
	out := make(map[string]float64)
	if f == nil {
		return out, nil
	}
	for _, m := range f.Metric {
		labels := labelMap(m)
		if len(labels) != 1 || labels[label] == "" {
			continue
		}
		v, err := metricValue(m)
		if err != nil || v < 0 {
			if err == nil {
				err = errors.New("negative metric")
			}
			return nil, err
		}
		if _, exists := out[labels[label]]; exists {
			return nil, errors.New("duplicate metric labels")
		}
		out[labels[label]] = v
	}
	return out, nil
}

func parseSSE(r io.Reader, fn func(eventName string, eventID uint64, data []byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), maxSSEEvent)
	var eventName string
	var eventID uint64
	var hasEventID bool
	var data []string
	size := 0
	flush := func() error {
		if len(data) == 0 {
			eventName = ""
			eventID = 0
			hasEventID = false
			return nil
		}
		if !hasEventID || eventID == 0 {
			return invalid("missing SSE event ID")
		}
		payload := []byte(strings.Join(data, "\n"))
		if len(payload) > maxSSEEvent {
			return invalid("SSE event is too large")
		}
		err := fn(eventName, eventID, payload)
		eventName = ""
		eventID = 0
		hasEventID = false
		data = data[:0]
		size = 0
		return err
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "id:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil || parsed == 0 {
				return invalid("invalid SSE event ID")
			}
			eventID = parsed
			hasEventID = true
			continue
		}
		if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			size += len(value)
			if size > maxSSEEvent {
				return invalid("SSE event is too large")
			}
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: err}
	}
	if err := flush(); err != nil {
		return err
	}
	return &Error{Code: "UPSTREAM_UNAVAILABLE", Err: io.EOF}
}
