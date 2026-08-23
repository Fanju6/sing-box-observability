package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/auth"
	"github.com/Fanju6/sing-box-observability/src/server/internal/collector"
	"github.com/Fanju6/sing-box-observability/src/server/internal/config"
	"github.com/Fanju6/sing-box-observability/src/server/internal/events"
	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
	"github.com/Fanju6/sing-box-observability/src/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	collector *collector.Collector
	store     *storage.Store
	auth      *auth.Manager
	cfg       config.Config
	log       *slog.Logger
}

func New(c *collector.Collector, s *storage.Store, a *auth.Manager, cfg config.Config, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{collector: c, store: s, auth: a, cfg: cfg, log: log}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestID, s.recoverer, s.securityHeaders)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "INVALID_ARGUMENT", "Route not found / 路由不存在", false, nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "INVALID_ARGUMENT", "Method not allowed / 不允许的请求方法", false, nil)
	})
	r.Get("/healthz", s.health)
	r.Get("/readyz", s.ready)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/session", s.getSession)
		r.Post("/session", s.createSession)
		r.Delete("/session", s.deleteSession)
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/meta", s.meta)
			r.Get("/overview", s.overview)
			r.Get("/rankings", s.rankings)
			r.Get("/dimensions/series", s.dimensionSeries)
			r.Get("/connections/active", s.active)
			r.Get("/connections/recent", s.recent)
			r.Get("/events", s.events)
		})
	})
	return r
}

type requestIDKey struct{}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if len(id) < 1 || len(id) > 128 || strings.ContainsAny(id, "\r\n") {
			id = newRequestID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}
func requestID(r *http.Request) string {
	if v, ok := r.Context().Value(requestIDKey{}).(string); ok {
		return v
	}
	return "req_unknown"
}
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic recovered", "requestId", requestID(r))
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error / 服务器内部错误", true, nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Authenticated(r) {
			writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Console authentication is required / 需要控制台认证", false, nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Storage is unavailable / 存储不可用", true, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authEnabled": s.auth.Enabled(), "authenticated": s.auth.Authenticated(r)})
}
func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Token == "" || len(req.Token) > 4096 {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid login request / 登录请求无效", false, nil)
		return
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid login request / 登录请求无效", false, nil)
		return
	}
	ok, retry := s.auth.Login(w, r, req.Token)
	if !ok {
		if retry > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry/time.Second)))
			writeError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many login attempts / 登录尝试过于频繁", true, nil)
			return
		}
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid console token / 控制台令牌无效", false, nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authEnabled": s.auth.Enabled(), "authenticated": true})
}
func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	s.auth.Clear(w, r)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.collector.Meta())
}

type windowParams struct {
	from, to time.Time
	window   model.TimeWindow
}

func (s *Server) parseWindow(r *http.Request, withStep bool) (windowParams, error) {
	q := r.URL.Query()
	rangeValue := q.Get("range")
	fromS, toS := q.Get("from"), q.Get("to")
	now := time.Now().UTC()
	if rangeValue != "" && (fromS != "" || toS != "") {
		return windowParams{}, apiErr{"INVALID_TIME_RANGE", "range cannot be combined with from/to", false}
	}
	if (fromS == "") != (toS == "") {
		return windowParams{}, apiErr{"INVALID_TIME_RANGE", "from and to must be provided together", false}
	}
	var from, to time.Time
	if fromS != "" {
		var err error
		from, err = time.Parse(time.RFC3339Nano, fromS)
		if err != nil {
			return windowParams{}, apiErr{"INVALID_TIME_RANGE", "from is not RFC3339", false}
		}
		to, err = time.Parse(time.RFC3339Nano, toS)
		if err != nil {
			return windowParams{}, apiErr{"INVALID_TIME_RANGE", "to is not RFC3339", false}
		}
		from = from.UTC()
		to = to.UTC()
	} else {
		if rangeValue == "" {
			rangeValue = "1h"
		}
		d, ok := presetRange(rangeValue)
		if !ok {
			return windowParams{}, apiErr{"INVALID_TIME_RANGE", "unsupported range", false}
		}
		to = now
		from = to.Add(-d)
	}
	if !to.After(from) {
		return windowParams{}, apiErr{"INVALID_TIME_RANGE", "to must be after from", false}
	}
	cutoff := now.Add(-s.cfg.Storage.Retention)
	if to.Before(cutoff) || to.Equal(cutoff) {
		return windowParams{}, apiErr{"RANGE_OUTSIDE_RETENTION", "requested range is outside retention / 请求范围超出保留期", false}
	}
	if from.Before(cutoff) {
		from = cutoff
	}
	if to.After(now) {
		to = now
	}
	if !to.After(from) {
		return windowParams{}, apiErr{"RANGE_OUTSIDE_RETENTION", "requested range is outside retention / 请求范围超出保留期", false}
	}
	step := s.cfg.Collector.PersistInterval
	if withStep {
		parsedStep, err := parseStep(q.Get("step"), step)
		if err != nil {
			return windowParams{}, apiErr{"INVALID_ARGUMENT", "invalid step", false}
		}
		step = parsedStep
	}
	minimumSeconds := mathCeil(to.Sub(from).Seconds() / 720)
	minimum := time.Duration(minimumSeconds) * time.Second
	if minimum > step {
		step = minimum
	}
	if step < time.Second {
		step = time.Second
	}
	stepSeconds := mathCeil(step.Seconds())
	step = time.Duration(stepSeconds) * time.Second
	return windowParams{from: from, to: to, window: model.TimeWindow{From: from, To: to, StepSeconds: stepSeconds}}, nil
}
func presetRange(v string) (time.Duration, bool) {
	switch v {
	case "15m":
		return 15 * time.Minute, true
	case "1h":
		return time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	}
	return 0, false
}
func parseStep(v string, fallback time.Duration) (time.Duration, error) {
	if v == "" || v == "auto" {
		return fallback, nil
	}
	if len(v) < 2 {
		return 0, errors.New("bad step")
	}
	unit := v[len(v)-1]
	n, err := strconv.ParseInt(v[:len(v)-1], 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("bad step")
	}
	switch unit {
	case 's':
		return time.Duration(n) * time.Second, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	}
	return 0, errors.New("bad step")
}
func mathCeil(v float64) int64 {
	n := int64(v)
	if float64(n) < v {
		n++
	}
	return n
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	p, err := s.parseWindow(r, true)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	current, dims, _, state := s.collector.Current()
	history, err := s.store.History(r.Context(), p.from, p.to, time.Duration(p.window.StepSeconds)*time.Second, s.cfg.Collector.PersistInterval)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "History storage is unavailable / 历史存储不可用", true, nil)
		return
	}
	var currentCopy *model.StatusSnapshot
	if current != nil {
		currentCopy = current
	}
	out := model.OverviewResponse{GeneratedAt: time.Now().UTC(), SourceState: state, Window: p.window, Current: currentCopy, RangeTotals: history.Totals, Series: history.Series, TopOutbounds: s.collector.TopOutbounds(), URLTests: s.collector.URLTests(), APIHealth: s.collector.APIHealth()}
	_ = dims
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) rankings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dimension := q.Get("dimension")
	if !validDimension(dimension) {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid ranking dimension / 排行维度无效", false, nil)
		return
	}
	cap := s.collector.Capabilities()
	if !contains(cap.RankingDimensions, dimension) {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Ranking dimension is unavailable upstream / 上游不支持该排行维度", false, nil)
		return
	}
	if !cap.ExposeSensitive && contains(cap.SensitiveDimensions, dimension) {
		writeError(w, r, http.StatusForbidden, "SENSITIVE_DIMENSION_DISABLED", "Sensitive ranking dimension is disabled / 敏感排行维度已禁用", false, nil)
		return
	}
	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "traffic"
	}
	if !validSort(sortBy) {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid ranking sort / 排行排序无效", false, nil)
		return
	}
	limit := parseInt(q.Get("limit"), 10)
	if limit < 1 || limit > 100 {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "limit must be between 1 and 100 / limit 必须在 1 到 100 之间", false, nil)
		return
	}
	p, err := s.parseWindow(r, false)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	result, err := s.store.Rankings(r.Context(), dimension, p.from, p.to, sortBy, limit, s.collector.ActiveConnections())
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Ranking storage is unavailable / 排行存储不可用", true, nil)
		return
	}
	result.Window = p.window
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) dimensionSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dimension := q.Get("dimension")
	if dimension != "network" && dimension != "inbound" && dimension != "outbound" {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Dimension history is available for network, inbound and outbound / 维度历史仅支持网络、入站和出站", false, nil)
		return
	}
	value := strings.TrimSpace(q.Get("value"))
	if value == "" || len(value) > 512 {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "A valid dimension value is required / 必须提供有效的维度值", false, nil)
		return
	}
	p, err := s.parseWindow(r, true)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	series, err := s.store.DimensionHistory(r.Context(), dimension, value, p.from, p.to, time.Duration(p.window.StepSeconds)*time.Second, s.cfg.Collector.PersistInterval)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Dimension history storage is unavailable / 维度历史存储不可用", true, nil)
		return
	}
	writeJSON(w, http.StatusOK, model.DimensionSeriesResponse{GeneratedAt: time.Now().UTC(), Window: p.window, Dimension: dimension, Value: value, Series: series})
}

func validDimension(v string) bool {
	switch v {
	case "network", "inbound", "outbound", "rule", "domain", "destination_ip", "source", "process", "user":
		return true
	}
	return false
}
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func validSort(v string) bool {
	switch v {
	case "traffic", "connections", "download", "upload":
		return true
	}
	return false
}

func (s *Server) active(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := pageParams(r)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid pagination / 分页参数无效", false, nil)
		return
	}
	q := r.URL.Query()
	if !validConnectionFilters(q) {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid connection filter / 连接筛选参数无效", false, nil)
		return
	}
	connections := s.collector.ActiveConnections()
	if !s.collector.Capabilities().ExposeSensitive {
		redactConnections(connections)
	}
	filtered := filterConnections(connections, q.Get("q"), q.Get("network"), q.Get("outbound"))
	writeJSON(w, http.StatusOK, model.ConnectionPage{GeneratedAt: time.Now().UTC(), Total: len(filtered), Limit: limit, Offset: offset, Data: paginate(filtered, limit, offset)})
}
func (s *Server) recent(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := pageParams(r)
	if !ok {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid pagination / 分页参数无效", false, nil)
		return
	}
	if !validConnectionFilters(r.URL.Query()) {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid connection filter / 连接筛选参数无效", false, nil)
		return
	}
	p, err := s.parseWindow(r, false)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	q := r.URL.Query()
	exposeSensitive := s.collector.Capabilities().ExposeSensitive
	data, total, err := s.store.ListRecent(r.Context(), p.from, p.to, q.Get("q"), q.Get("network"), q.Get("outbound"), exposeSensitive, limit, offset)
	if err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "STORAGE_UNAVAILABLE", "Connection storage is unavailable / 连接存储不可用", true, nil)
		return
	}
	now := time.Now().UTC()
	if !exposeSensitive {
		redactConnections(data)
	}
	for i := range data {
		data[i] = data[i].WithDuration(now)
	}
	writeJSON(w, http.StatusOK, model.ConnectionPage{GeneratedAt: now, Total: total, Limit: limit, Offset: offset, Data: data})
}
func redactConnections(connections []model.Connection) {
	for i := range connections {
		connections[i].SourceIP = ""
		connections[i].SourcePort = 0
		connections[i].SourceMAC = ""
		connections[i].SourceHostname = ""
		connections[i].DestinationIP = ""
		connections[i].Domain = ""
		connections[i].Process = ""
		connections[i].User = ""
		connections[i].Rule = ""
	}
}
func pageParams(r *http.Request) (int, int, bool) {
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)
	return limit, offset, limit >= 1 && limit <= 500 && offset >= 0
}
func validConnectionFilters(q map[string][]string) bool {
	get := func(name string) string {
		values := q[name]
		if len(values) == 0 {
			return ""
		}
		return values[0]
	}
	return len(get("q")) <= 200 && len(get("network")) <= 32 && len(get("outbound")) <= 512
}
func paginate(in []model.Connection, limit, offset int) []model.Connection {
	if offset >= len(in) {
		return []model.Connection{}
	}
	end := offset + limit
	if end > len(in) {
		end = len(in)
	}
	return in[offset:end]
}
func filterConnections(in []model.Connection, q, network, outbound string) []model.Connection {
	q = strings.ToLower(q)
	out := make([]model.Connection, 0, len(in))
	for _, c := range in {
		if network != "" && c.Network != network {
			continue
		}
		if outbound != "" && c.Outbound != outbound {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(connectionSearch(c)), q) {
			continue
		}
		out = append(out, c)
	}
	return out
}
func connectionSearch(c model.Connection) string {
	return strings.Join([]string{c.ID, c.Network, c.Inbound, c.SourceIP, c.SourceHostname, c.DestinationIP, c.Domain, c.Process, c.User, c.Outbound, c.OutboundType, strings.Join(c.Chain, " "), c.Rule}, " ")
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	heartbeat, err := parseHeartbeat(r.URL.Query().Get("heartbeat"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid heartbeat / heartbeat 无效", false, nil)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Streaming is unavailable / 流式响应不可用", true, nil)
		return
	}
	lastID := uint64(0)
	rawLastID := r.Header.Get("Last-Event-ID")
	if rawLastID == "" {
		// EventSource does not allow setting request headers when a new instance is
		// created by application-level backoff. The query fallback preserves replay
		// semantics across those reconnects; native reconnects still use the header.
		rawLastID = r.URL.Query().Get("lastEventId")
	}
	if rawLastID != "" {
		lastID, err = strconv.ParseUint(rawLastID, 10, 64)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid Last-Event-ID", false, nil)
			return
		}
	}
	sub := s.collectorHub().Subscribe(lastID)
	defer sub.Close()
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-sub.C:
			if !open {
				return
			}
			if err := writeSSE(w, flusher, event); err != nil {
				return
			}
		case <-ticker.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
func (s *Server) collectorHub() *events.Hub { return s.collector.Hub() }
func parseHeartbeat(v string) (time.Duration, error) {
	if v == "" {
		return 15 * time.Second, nil
	}
	if len(v) < 2 {
		return 0, errors.New("bad heartbeat")
	}
	n, err := strconv.ParseInt(v[:len(v)-1], 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("bad heartbeat")
	}
	switch v[len(v)-1] {
	case 's':
		return time.Duration(n) * time.Second, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	}
	return 0, errors.New("bad heartbeat")
}
func writeSSE(w http.ResponseWriter, f http.Flusher, e model.EventEnvelope) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.Sequence, e.Type, data); err != nil {
		return err
	}
	f.Flush()
	return nil
}

type apiErr struct {
	code, message string
	retryable     bool
}

func (e apiErr) Error() string { return e.code }
func writeAPIError(w http.ResponseWriter, r *http.Request, e error) {
	var ae apiErr
	if errors.As(e, &ae) {
		status := http.StatusBadRequest
		if ae.code == "RANGE_OUTSIDE_RETENTION" {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, r, status, ae.code, ae.message, ae.retryable, nil)
		return
	}
	writeError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid request / 请求无效", false, nil)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "requestId": requestID(r), "retryable": retryable, "details": details}})
}
func parseInt(v string, defaultValue int) int {
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}
