package collector

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/buildinfo"
	"github.com/Fanju6/sing-box-observability/src/server/internal/config"
	"github.com/Fanju6/sing-box-observability/src/server/internal/events"
	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
	"github.com/Fanju6/sing-box-observability/src/server/internal/source"
	"github.com/Fanju6/sing-box-observability/src/server/internal/storage"
)

type Collector struct {
	client              *source.Client
	store               *storage.Store
	hub                 *events.Hub
	cfg                 config.Config
	log                 *slog.Logger
	mu                  sync.RWMutex
	current             *model.StatusSnapshot
	metric              *model.MetricSnapshot
	dimensions          []model.DimensionSnapshot
	active              map[string]model.Connection
	misses              map[string]int
	state               model.SourceState
	lastAttempt         *time.Time
	lastSuccess         *time.Time
	lastError           *string
	capabilities        model.Capabilities
	capabilitiesReady   bool
	previous            *model.MetricSnapshot
	apiHealth           *model.APIHealthSnapshot
	channels            model.CollectorChannels
	lastUpstreamEventID uint64
	eventConnectedOnce  bool
	backfillRequests    chan string
	started             bool
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
}

func New(client *source.Client, store *storage.Store, hub *events.Hub, cfg config.Config, log *slog.Logger) *Collector {
	if log == nil {
		log = slog.Default()
	}
	connecting := model.CollectorChannel{State: model.StateConnecting}
	return &Collector{
		client: client, store: store, hub: hub, cfg: cfg, log: log,
		state: model.StateConnecting, active: make(map[string]model.Connection), misses: make(map[string]int),
		backfillRequests: make(chan string, 1),
		channels: model.CollectorChannels{
			Capabilities: connecting,
			Metrics:      connecting,
			Connections:  connecting,
			Events:       model.EventChannel{CollectorChannel: connecting},
		},
	}
}

func (c *Collector) Run(ctx context.Context) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	child, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()
	capabilitiesReady := true
	if err := c.refreshCapabilities(child); err != nil {
		capabilitiesReady = false
	}
	if capabilitiesReady {
		if err := c.scrape(child); err != nil {
		}
		if err := c.reconcile(child); err != nil {
			c.log.Warn("initial active reconciliation failed", "code", source.ErrorCode(err))
		}
	}
	for _, worker := range []func(context.Context){c.backfillLoop, c.scrapeLoop, c.reconcileLoop, c.persistLoop, c.capabilitiesLoop, c.retentionLoop, c.sseLoop} {
		c.wg.Add(1)
		go func(run func(context.Context)) { defer c.wg.Done(); run(child) }(worker)
	}
}

func (c *Collector) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	c.persist(context.Background())
}

func (c *Collector) scrapeLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Collector.ScrapeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.retryAllowed() || !c.hasCapabilities() {
				continue
			}
			_ = c.scrape(ctx)
		}
	}
}
func (c *Collector) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Collector.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.retryAllowed() || !c.hasCapabilities() {
				continue
			}
			if err := c.reconcile(ctx); err != nil {
				c.log.Warn("active reconciliation failed", "code", source.ErrorCode(err))
			}
		}
	}
}
func (c *Collector) persistLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Collector.PersistInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.persist(ctx)
		}
	}
}
func (c *Collector) capabilitiesLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.retryAllowed() {
				continue
			}
			wasReady := c.hasCapabilities()
			if err := c.refreshCapabilities(ctx); err != nil {
				c.log.Warn("capabilities refresh failed", "code", source.ErrorCode(err))
				continue
			}
			if !wasReady {
				if err := c.scrape(ctx); err != nil {
				}
				if err := c.reconcile(ctx); err != nil {
					c.log.Warn("active reconciliation after capabilities recovery failed", "code", source.ErrorCode(err))
				}
				c.backfill(ctx)
			}
		}
	}
}
func (c *Collector) retentionLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.store.Retention(ctx, time.Now().UTC().Add(-c.cfg.Storage.Retention)); err != nil {
				c.log.Warn("retention cleanup failed")
			}
		}
	}
}

func (c *Collector) refreshCapabilities(ctx context.Context) error {
	c.markChannelAttempt("capabilities")
	capabilities, err := c.client.Capabilities(ctx)
	if err != nil {
		c.markChannelFailure("capabilities", err)
		return err
	}
	ttl, _ := time.ParseDuration(capabilities.RecentTTL)
	cap := model.Capabilities{
		UpstreamAPIVersion:    capabilities.APIVersion,
		Endpoints:             append([]string(nil), capabilities.Endpoints...),
		ExposeSensitive:       capabilities.ExposeSensitive,
		RankingDimensions:     append([]string(nil), capabilities.TopDimensions...),
		SensitiveDimensions:   append([]string(nil), capabilities.SensitiveDimensions...),
		RecentConnectionLimit: capabilities.RecentLimit,
		RecentTTLSeconds:      int64(ttl / time.Second),
		TopKSize:              capabilities.TopKLimit,
		ActivePageLimit:       capabilities.ActivePageLimit,
		CursorPagination:      capabilities.CursorPagination,
		EventReplay:           capabilities.EventReplay,
	}
	c.mu.Lock()
	c.capabilities = cap
	c.capabilitiesReady = true
	c.mu.Unlock()
	c.markChannelSuccess("capabilities")
	return nil
}

func (c *Collector) hasCapabilities() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilitiesReady
}

func (c *Collector) scrape(ctx context.Context) error {
	c.markChannelAttempt("metrics")
	snapshot, err := c.client.Metrics(ctx)
	now := snapshot.ObservedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err != nil {
		c.markChannelFailure("metrics", err)
		return err
	}
	c.mu.Lock()
	previous := c.previous
	status, dims, apiHealth := withRates(snapshot, previous, c.cfg.Collector.ScrapeInterval)
	c.previous = &snapshot
	c.metric = &snapshot
	c.current = &status
	c.dimensions = dims
	c.apiHealth = &apiHealth
	c.mu.Unlock()
	c.markChannelSuccessAt("metrics", now)
	return nil
}

func (c *Collector) channelLocked(name string) *model.CollectorChannel {
	switch name {
	case "capabilities":
		return &c.channels.Capabilities
	case "metrics":
		return &c.channels.Metrics
	case "connections":
		return &c.channels.Connections
	case "events":
		return &c.channels.Events.CollectorChannel
	default:
		panic("unknown collector channel: " + name)
	}
}

func (c *Collector) markChannelAttempt(name string) {
	now := time.Now().UTC()
	c.mu.Lock()
	channel := c.channelLocked(name)
	channel.LastAttemptAt = &now
	c.lastAttempt = &now
	c.mu.Unlock()
}

func (c *Collector) markChannelSuccess(name string) {
	c.markChannelSuccessAt(name, time.Now().UTC())
}

func (c *Collector) markChannelSuccessAt(name string, now time.Time) {
	c.mu.Lock()
	channel := c.channelLocked(name)
	channel.State = model.StateOnline
	channel.LastAttemptAt = &now
	channel.LastSuccessAt = &now
	channel.LastErrorCode = nil
	c.lastAttempt = &now
	c.lastSuccess = &now
	old := c.state
	c.recomputeStateLocked()
	changed := old != c.state
	c.mu.Unlock()
	if changed {
		c.publishState()
	}
}

func (c *Collector) markChannelFailure(name string, err error) {
	code := source.ErrorCode(err)
	now := time.Now().UTC()
	c.mu.Lock()
	channel := c.channelLocked(name)
	channel.LastAttemptAt = &now
	channel.LastErrorCode = &code
	if code == "UPSTREAM_UNAUTHORIZED" {
		channel.State = model.StateUnauthorized
	} else if channel.LastSuccessAt != nil && now.Sub(*channel.LastSuccessAt) <= c.cfg.Collector.StaleAfter {
		channel.State = model.StateStale
	} else {
		channel.State = model.StateOffline
	}
	c.lastAttempt = &now
	c.lastError = &code
	old := c.state
	c.recomputeStateLocked()
	changed := old != c.state
	c.mu.Unlock()
	if changed {
		c.publishState()
	}
}

func (c *Collector) recomputeStateLocked() {
	states := []model.SourceState{
		c.channels.Capabilities.State,
		c.channels.Metrics.State,
		c.channels.Connections.State,
		c.channels.Events.State,
	}
	for _, state := range states {
		if state == model.StateUnauthorized {
			c.state = model.StateUnauthorized
			return
		}
	}
	if c.channels.Metrics.State == model.StateOffline || c.channels.Capabilities.State == model.StateOffline {
		c.state = model.StateOffline
		return
	}
	for _, state := range states {
		if state == model.StateConnecting {
			c.state = model.StateConnecting
			return
		}
	}
	for _, state := range states {
		if state != model.StateOnline {
			c.state = model.StateStale
			return
		}
	}
	c.state = model.StateOnline
	c.lastError = nil
}

func (c *Collector) recordFailure(err error) { c.markChannelFailure("metrics", err) }

func (c *Collector) retryAllowed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state != model.StateUnauthorized || c.lastAttempt == nil || time.Since(*c.lastAttempt) >= 30*time.Second
}

func (c *Collector) reconcile(ctx context.Context) error {
	c.markChannelAttempt("connections")
	c.mu.RLock()
	cap := c.capabilities
	ready := c.capabilitiesReady
	c.mu.RUnlock()
	if !ready {
		err := errors.New("upstream capabilities unavailable")
		c.markChannelFailure("connections", err)
		return err
	}
	connections, err := c.client.Active(ctx, cap.ActivePageLimit)
	if err != nil {
		c.markChannelFailure("connections", err)
		return err
	}
	now := time.Now().UTC()
	incoming := make(map[string]model.Connection, len(connections))
	for _, raw := range connections {
		conn := toModelConnection(raw, "active", cap.ExposeSensitive).WithDuration(now)
		if conn.ID != "" {
			incoming[conn.ID] = conn
		}
	}
	type lifecycleEvent struct {
		kind       string
		connection model.Connection
	}
	eventsToPublish := make([]lifecycleEvent, 0)
	connectionsToPersist := make([]model.Connection, 0, len(incoming))
	c.mu.Lock()
	for _, conn := range incoming {
		c.misses[conn.ID] = 0
		_, exists := c.active[conn.ID]
		c.active[conn.ID] = conn
		if !exists {
			eventsToPublish = append(eventsToPublish, lifecycleEvent{kind: "connection.open", connection: conn})
		}
		connectionsToPersist = append(connectionsToPersist, conn)
	}
	for id, old := range c.active {
		if _, ok := incoming[id]; ok {
			continue
		}
		c.misses[id]++
		if c.misses[id] < 2 {
			continue
		}
		closed := old
		t := now
		closed.State = "closed"
		closed.ClosedAt = &t
		closed = closed.WithDuration(now)
		delete(c.active, id)
		delete(c.misses, id)
		eventsToPublish = append(eventsToPublish, lifecycleEvent{kind: "connection.close", connection: closed})
		connectionsToPersist = append(connectionsToPersist, closed)
	}
	c.mu.Unlock()
	for _, conn := range connectionsToPersist {
		if err := c.store.UpsertConnection(ctx, conn); err != nil {
			c.log.Warn("persist reconciled connection failed")
		}
	}
	for _, event := range eventsToPublish {
		c.hub.Publish(event.kind, event.connection)
	}
	c.markChannelSuccessAt("connections", now)
	return nil
}

func (c *Collector) backfillLoop(ctx context.Context) {
	c.backfill(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case reason := <-c.backfillRequests:
			c.backfill(ctx)
			c.hub.Publish("resync", model.ResyncEventData{Reason: reason})
		}
	}
}

func (c *Collector) requestBackfill(reason string) {
	select {
	case c.backfillRequests <- reason:
	default:
	}
}

func (c *Collector) backfill(ctx context.Context) {
	c.mu.RLock()
	cap := c.capabilities
	ready := c.capabilitiesReady
	c.mu.RUnlock()
	if !ready {
		return
	}
	limit := cap.RecentConnectionLimit
	window := time.Duration(cap.RecentTTLSeconds) * time.Second
	cursor := ""
	seenCursors := make(map[string]bool)
	loaded := 0
	for loaded < limit {
		page, err := c.client.RecentPage(ctx, window, min(500, limit-loaded), cursor)
		if err != nil {
			c.log.Warn("recent connection backfill failed", "code", source.ErrorCode(err))
			return
		}
		for _, raw := range page.Data {
			conn := toModelConnection(raw, "closed", cap.ExposeSensitive).WithDuration(time.Now().UTC())
			_ = c.store.UpsertConnection(ctx, conn)
		}
		loaded += len(page.Data)
		if !page.HasMore {
			break
		}
		if seenCursors[page.NextCursor] {
			c.log.Warn("recent connection backfill stopped on cursor cycle")
			return
		}
		seenCursors[page.NextCursor] = true
		cursor = page.NextCursor
	}
}

func (c *Collector) persist(ctx context.Context) {
	c.mu.RLock()
	metric := c.metric
	dims := append([]model.DimensionSnapshot(nil), c.dimensions...)
	state := c.state
	c.mu.RUnlock()
	if metric == nil {
		return
	}
	if err := c.store.InsertSnapshot(ctx, *metric, dims, state); err != nil {
		c.log.Warn("persist snapshot failed")
	}
}

func (c *Collector) sseLoop(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if !c.hasCapabilities() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				continue
			}
		}
		c.markChannelAttempt("events")
		err := c.client.StreamEvents(ctx, func() {
			c.mu.Lock()
			c.lastUpstreamEventID = 0
			reconnected := c.eventConnectedOnce
			if reconnected {
				c.channels.Events.Reconnects++
			}
			c.eventConnectedOnce = true
			c.mu.Unlock()
			c.markChannelSuccess("events")
			if reconnected {
				if err := c.reconcile(ctx); err != nil {
					c.log.Warn("active reconciliation after upstream reconnect failed", "code", source.ErrorCode(err))
				}
				c.requestBackfill("upstream_reconnected")
			}
			backoff = time.Second
		}, func(e source.Event) error {
			now := time.Now().UTC()
			c.mu.Lock()
			previousID := c.lastUpstreamEventID
			c.lastUpstreamEventID = e.ID
			c.channels.Events.LastEventAt = &now
			c.channels.Events.LastSequence = e.ID
			c.mu.Unlock()
			if previousID != 0 && e.ID != previousID+1 {
				if err := c.reconcile(ctx); err != nil {
					c.log.Warn("active reconciliation after upstream event gap failed", "code", source.ErrorCode(err))
				}
				c.requestBackfill("upstream_event_gap")
			}
			c.handleEvent(ctx, e)
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		c.markChannelFailure("events", err)
		if source.ErrorCode(err) == "UPSTREAM_UNAUTHORIZED" {
			backoff = 30 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(backoff)):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}
func jitter(d time.Duration) time.Duration {
	return time.Duration(float64(d) * (0.8 + 0.4*float64(time.Now().UnixNano()%1000)/1000))
}

func (c *Collector) handleEvent(ctx context.Context, event source.Event) {
	now := time.Now().UTC()
	if event.Connection.ID == "" || event.Connection.StartedAt.IsZero() || event.Connection.Upload < 0 || event.Connection.Download < 0 {
		return
	}
	c.mu.RLock()
	sensitive := c.capabilities.ExposeSensitive
	c.mu.RUnlock()
	conn := toModelConnection(event.Connection, "active", sensitive).WithDuration(now)
	if conn.ID == "" {
		return
	}
	if event.Type == "open" {
		c.mu.Lock()
		_, exists := c.active[conn.ID]
		c.active[conn.ID] = conn
		c.misses[conn.ID] = 0
		c.mu.Unlock()
		if !exists {
			c.hub.Publish("connection.open", conn)
		}
		if err := c.store.UpsertConnection(ctx, conn); err != nil {
			c.log.Warn("persist opened connection failed")
		}
		return
	}
	if event.Type == "close" {
		conn.State = "closed"
		if conn.ClosedAt == nil {
			conn.ClosedAt = &now
		}
		conn = conn.WithDuration(now)
		c.mu.Lock()
		delete(c.active, conn.ID)
		delete(c.misses, conn.ID)
		c.mu.Unlock()
		c.hub.Publish("connection.close", conn)
		if err := c.store.UpsertConnection(ctx, conn); err != nil {
			c.log.Warn("persist closed connection failed")
		}
	}
}

func toModelConnection(raw source.Connection, state string, sensitive bool) model.Connection {
	c := model.Connection{ID: raw.ID, State: state, Network: raw.Network, Inbound: raw.Inbound, DestinationPort: raw.DestinationPort, Outbound: raw.Outbound, OutboundType: raw.OutboundType, Chain: append([]string(nil), raw.Chain...), StartedAt: raw.StartedAt.UTC(), ClosedAt: raw.ClosedAt, Upload: raw.Upload, Download: raw.Download}
	if c.Chain == nil {
		c.Chain = []string{}
	}
	if sensitive {
		c.SourceIP = raw.SourceIP
		c.SourcePort = raw.SourcePort
		c.SourceMAC = raw.SourceMAC
		c.SourceHostname = raw.SourceHostname
		c.DestinationIP = raw.DestinationIP
		c.Domain = raw.Domain
		c.Process = raw.Process
		c.User = raw.User
		c.Rule = raw.Rule
	}
	return c
}

func (c *Collector) publishState() {
	c.mu.RLock()
	data := model.SourceStateEventData{State: c.state, LastSuccessAt: c.lastSuccess, ErrorCode: c.lastError}
	c.mu.RUnlock()
	c.hub.Publish("source.state", data)
}

func (c *Collector) Meta() model.MetaResponse {
	c.mu.RLock()
	state := c.state
	lastAttempt := c.lastAttempt
	lastSuccess := c.lastSuccess
	lastError := c.lastError
	channels := c.channels
	var capabilities *model.Capabilities
	if c.capabilitiesReady {
		copy := c.capabilities
		copy.Endpoints = append([]string(nil), c.capabilities.Endpoints...)
		copy.RankingDimensions = append([]string(nil), c.capabilities.RankingDimensions...)
		copy.SensitiveDimensions = append([]string(nil), c.capabilities.SensitiveDimensions...)
		capabilities = &copy
	}
	c.mu.RUnlock()
	var from *time.Time
	if c.store != nil {
		if t, err := c.store.HistoryFrom(context.Background()); err == nil && !t.IsZero() {
			from = &t
		}
	}
	return model.MetaResponse{APIVersion: "v1", AppVersion: buildinfo.Version, GeneratedAt: time.Now().UTC(), Source: model.SourceInfo{State: state, DisplayName: c.cfg.Singbox.Name, LastAttemptAt: lastAttempt, LastSuccessAt: lastSuccess, LastErrorCode: lastError, HistoryAvailableFrom: from}, Capabilities: capabilities, Collector: model.CollectorInfo{ScrapeIntervalSeconds: c.cfg.Collector.ScrapeInterval.Seconds(), PersistIntervalSeconds: c.cfg.Collector.PersistInterval.Seconds(), RetentionSeconds: int64(c.cfg.Storage.Retention / time.Second), MaxSeriesPoints: 720, Channels: channels}}
}
func (c *Collector) Current() (*model.StatusSnapshot, []model.DimensionSnapshot, []model.URLTestResult, model.SourceState) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var current *model.StatusSnapshot
	if c.current != nil {
		copy := *c.current
		current = &copy
	}
	return current, append([]model.DimensionSnapshot(nil), c.dimensions...), append([]model.URLTestResult(nil), func() []model.URLTestResult {
		if c.metric == nil {
			return nil
		}
		return c.metric.URLTests
	}()...), c.state
}
func (c *Collector) ActiveConnections() []model.Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.Connection, 0, len(c.active))
	now := time.Now().UTC()
	for _, conn := range c.active {
		out = append(out, conn.WithDuration(now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}
func (c *Collector) Capabilities() model.Capabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cap := c.capabilities
	cap.Endpoints = append([]string(nil), cap.Endpoints...)
	cap.RankingDimensions = append([]string(nil), cap.RankingDimensions...)
	cap.SensitiveDimensions = append([]string(nil), cap.SensitiveDimensions...)
	return cap
}

func (c *Collector) APIHealth() *model.APIHealthSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.apiHealth == nil {
		return nil
	}
	copy := *c.apiHealth
	copy.Endpoints = append([]model.APIEndpointHealth(nil), c.apiHealth.Endpoints...)
	return &copy
}

func (c *Collector) Check() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.current == nil && c.state == model.StateConnecting {
		return errors.New("collector is starting")
	}
	return nil
}
func RuntimeGoroutines() int { return runtime.NumGoroutine() }
func safeRate(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 {
		return nil
	}
	return v
}
func (c *Collector) TopOutbounds() []model.CompactRanking {
	_, dims, _, _ := c.Current()
	out := make([]model.CompactRanking, 0)
	for _, d := range dims {
		if d.Kind != "outbound" {
			continue
		}
		out = append(out, model.CompactRanking{Value: d.Value, UploadBytesPerSecond: safeRate(d.UploadRate), DownloadBytesPerSecond: safeRate(d.DownloadRate), ActiveConnections: d.Active})
	}
	sort.Slice(out, func(i, j int) bool {
		li := rateValue(out[i].UploadBytesPerSecond) + rateValue(out[i].DownloadBytesPerSecond)
		lj := rateValue(out[j].UploadBytesPerSecond) + rateValue(out[j].DownloadBytesPerSecond)
		if li == lj {
			return out[i].Value < out[j].Value
		}
		return li > lj
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
func rateValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
func (c *Collector) URLTests() []model.URLTestResult {
	_, _, tests, _ := c.Current()
	if tests == nil {
		tests = []model.URLTestResult{}
	}
	now := time.Now().UTC()
	for i := range tests {
		tests[i].AgeSeconds = math.Max(0, now.Sub(tests[i].MeasuredAt).Seconds())
	}
	return tests
}
func (c *Collector) RefreshFailureForTest(err error) { c.recordFailure(err) }
func (c *Collector) Logger() *slog.Logger            { return c.log }
func normalizeErrorCode(code string) string          { return strings.TrimSpace(code) }
func (c *Collector) Hub() *events.Hub                { return c.hub }
