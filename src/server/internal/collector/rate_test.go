package collector

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/config"
	"github.com/Fanju6/sing-box-observability/src/server/internal/events"
	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
)

func TestMetaCapabilitiesAreNullUntilUpstreamProtocolIsValidated(t *testing.T) {
	collector := New(nil, nil, events.NewHub(), config.Default(), nil)
	meta := collector.Meta()
	if meta.Capabilities != nil {
		t.Fatalf("capabilities must be unknown before validation: %#v", meta.Capabilities)
	}
	body, err := json.Marshal(meta)
	if err != nil || !strings.Contains(string(body), `"capabilities":null`) {
		t.Fatalf("meta JSON %s err=%v", body, err)
	}
}

func TestCounterRateResetGapAndUptimeRollback(t *testing.T) {
	t0 := time.Unix(100, 0)
	got := counterRate(t0.Add(2*time.Second), t0, 10, 9, "v", "v", 120, 100, 220, 200, 2*time.Second)
	if got.Upload == nil || *got.Upload != 10 || got.Download == nil || *got.Download != 10 {
		t.Fatalf("rate %#v", got)
	}
	reset := counterRate(t0.Add(4*time.Second), t0.Add(2*time.Second), 12, 11, "v", "v", 20, 120, 30, 220, 2*time.Second)
	if reset.Upload != nil || reset.Download != nil {
		t.Fatalf("reset produced rate %#v", reset)
	}
	uptimeReset := counterRate(t0.Add(6*time.Second), t0.Add(4*time.Second), 1, 12, "v", "v", 40, 20, 50, 30, 2*time.Second)
	if uptimeReset.Upload != nil {
		t.Fatalf("uptime reset produced rate %#v", uptimeReset)
	}
	gap := counterRate(t0.Add(20*time.Second), t0.Add(6*time.Second), 20, 1, "v", "v", 100, 40, 100, 50, 2*time.Second)
	if gap.Upload != nil {
		t.Fatalf("gap produced rate %#v", gap)
	}
}

func TestWithRatesDoesNotProduceNegativeDimensionRate(t *testing.T) {
	t0 := time.Unix(100, 0)
	old := model.MetricSnapshot{ObservedAt: t0, Version: "v", UptimeSeconds: 1, UploadBytesTotal: 100, DownloadBytesTotal: 100, Dimensions: []model.DimensionSnapshot{{Kind: "outbound", Value: "direct", UploadTotal: 100, DownloadTotal: 100}}}
	newer := old
	newer.ObservedAt = t0.Add(time.Second)
	newer.UptimeSeconds = 2
	newer.UploadBytesTotal = 101
	newer.DownloadBytesTotal = 101
	newer.Dimensions = []model.DimensionSnapshot{{Kind: "outbound", Value: "direct", UploadTotal: 90, DownloadTotal: 110}}
	status, dims, _ := withRates(newer, &old, time.Second)
	if status.UploadBytesPerSecond == nil || *status.UploadBytesPerSecond < 0 {
		t.Fatalf("status %#v", status)
	}
	if dims[0].UploadRate != nil || dims[0].DownloadRate == nil || *dims[0].DownloadRate < 0 {
		t.Fatalf("dims %#v", dims)
	}
}

func TestWithRatesBuildsAPIHealthFromCounterDeltas(t *testing.T) {
	t0 := time.Unix(100, 0)
	old := model.MetricSnapshot{ObservedAt: t0, Version: "v", UptimeSeconds: 10, RecentConnections: 25, RecentConnectionsCapacity: 100, SSEEventsTotal: 10, API: []model.APIMetricCounter{{Endpoint: "metrics", Status: 200, RequestsTotal: 10, ResponseBytesTotal: 1000, DurationSecondsTotal: 0.5}, {Endpoint: "events", Status: 500, RequestsTotal: 1, ResponseBytesTotal: 50, DurationSecondsTotal: 0.1}}}
	current := model.MetricSnapshot{ObservedAt: t0.Add(2 * time.Second), Version: "v", UptimeSeconds: 12, RecentConnections: 50, RecentConnectionsCapacity: 100, SSESubscribers: 2, SSEEventsTotal: 16, API: []model.APIMetricCounter{{Endpoint: "metrics", Status: 200, RequestsTotal: 14, ResponseBytesTotal: 1800, DurationSecondsTotal: 0.9}, {Endpoint: "events", Status: 500, RequestsTotal: 2, ResponseBytesTotal: 100, DurationSecondsTotal: 0.3}}}
	_, _, health := withRates(current, &old, 2*time.Second)
	if health.RecentConnectionsUtilization == nil || *health.RecentConnectionsUtilization != 0.5 || health.SSEEventsPerSecond == nil || *health.SSEEventsPerSecond != 3 {
		t.Fatalf("health %#v", health)
	}
	if health.ErrorRate == nil || *health.ErrorRate != 0.2 || len(health.Endpoints) != 2 || health.Endpoints[0].RequestsPerSecond == nil {
		t.Fatalf("endpoint health %#v", health)
	}
}

func TestJitterStaysWithinTwentyPercent(t *testing.T) {
	base := 10 * time.Second
	for range 100 {
		got := jitter(base)
		if got < 8*time.Second || got > 12*time.Second {
			t.Fatalf("jitter(%v) = %v", base, got)
		}
	}
}

func TestCollectorReportsIndependentChannelHealth(t *testing.T) {
	collector := New(nil, nil, events.NewHub(), config.Default(), nil)
	for _, channel := range []string{"capabilities", "metrics", "connections", "events"} {
		collector.markChannelSuccess(channel)
	}
	meta := collector.Meta()
	if meta.Source.State != model.StateOnline || meta.Collector.Channels.Events.State != model.StateOnline {
		t.Fatalf("healthy channels did not produce online state: %#v", meta)
	}
	collector.markChannelFailure("events", errors.New("stream closed"))
	meta = collector.Meta()
	if meta.Source.State != model.StateStale || meta.Collector.Channels.Metrics.State != model.StateOnline || meta.Collector.Channels.Events.State != model.StateStale {
		t.Fatalf("event failure was not isolated: %#v", meta.Collector.Channels)
	}
}
