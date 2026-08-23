package collector

import (
	"math"
	"time"

	"github.com/Fanju6/sing-box-observability/src/server/internal/model"
)

type rateResult struct{ Upload, Download *float64 }
type endpointStatus struct {
	endpoint string
	status   int
}

func counterRate(now, previous time.Time, uptime, previousUptime float64, version, previousVersion string, upload, previousUpload, download, previousDownload int64, scrapeInterval time.Duration) rateResult {
	if previous.IsZero() || !sameGeneration(uptime, previousUptime, version, previousVersion) {
		return rateResult{}
	}
	seconds := now.Sub(previous).Seconds()
	if seconds <= 0 || seconds > scrapeInterval.Seconds()*3 {
		return rateResult{}
	}
	result := rateResult{}
	if upload >= previousUpload {
		value := float64(upload-previousUpload) / seconds
		if !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 {
			result.Upload = &value
		}
	}
	if download >= previousDownload {
		value := float64(download-previousDownload) / seconds
		if !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 {
			result.Download = &value
		}
	}
	return result
}

func sameGeneration(uptime, previousUptime float64, version, previousVersion string) bool {
	return uptime >= previousUptime && version == previousVersion
}

func withRates(current model.MetricSnapshot, previous *model.MetricSnapshot, scrapeInterval time.Duration) (model.StatusSnapshot, []model.DimensionSnapshot, model.APIHealthSnapshot) {
	status := model.StatusSnapshot{ObservedAt: current.ObservedAt, Version: current.Version, UptimeSeconds: current.UptimeSeconds, MemoryBytes: current.MemoryBytes, Goroutines: current.Goroutines, ActiveConnections: current.ActiveConnections, RecentConnections: current.RecentConnections, ConnectionsTotal: current.ConnectionsTotal, UploadBytesTotal: current.UploadBytesTotal, DownloadBytesTotal: current.DownloadBytesTotal}
	apiHealth := model.APIHealthSnapshot{RecentConnectionsCapacity: current.RecentConnectionsCapacity, SSESubscribers: current.SSESubscribers, SSEEventsTotal: current.SSEEventsTotal, Endpoints: make([]model.APIEndpointHealth, 0, len(current.API))}
	if current.RecentConnectionsCapacity > 0 {
		value := math.Min(1, float64(current.RecentConnections)/float64(current.RecentConnectionsCapacity))
		apiHealth.RecentConnectionsUtilization = &value
	}
	var global rateResult
	if previous != nil {
		global = counterRate(current.ObservedAt, previous.ObservedAt, current.UptimeSeconds, previous.UptimeSeconds, current.Version, previous.Version, current.UploadBytesTotal, previous.UploadBytesTotal, current.DownloadBytesTotal, previous.DownloadBytesTotal, scrapeInterval)
	}
	status.UploadBytesPerSecond = global.Upload
	status.DownloadBytesPerSecond = global.Download
	byKey := make(map[string]model.DimensionSnapshot)
	if previous != nil {
		for _, d := range previous.Dimensions {
			byKey[d.Kind+"\x00"+d.Value] = d
		}
	}
	dimensions := make([]model.DimensionSnapshot, 0, len(current.Dimensions))
	for _, d := range current.Dimensions {
		if old, ok := byKey[d.Kind+"\x00"+d.Value]; ok {
			rates := counterRate(current.ObservedAt, previous.ObservedAt, current.UptimeSeconds, previous.UptimeSeconds, current.Version, previous.Version, d.UploadTotal, old.UploadTotal, d.DownloadTotal, old.DownloadTotal, scrapeInterval)
			d.UploadRate = rates.Upload
			d.DownloadRate = rates.Download
		}
		dimensions = append(dimensions, d)
	}
	previousAPI := make(map[endpointStatus]model.APIMetricCounter)
	validInterval := false
	seconds := 0.0
	if previous != nil && sameGeneration(current.UptimeSeconds, previous.UptimeSeconds, current.Version, previous.Version) {
		seconds = current.ObservedAt.Sub(previous.ObservedAt).Seconds()
		validInterval = seconds > 0 && seconds <= scrapeInterval.Seconds()*3
		for _, metric := range previous.API {
			previousAPI[endpointStatus{endpoint: metric.Endpoint, status: metric.Status}] = metric
		}
		if validInterval && current.SSEEventsTotal >= previous.SSEEventsTotal {
			value := float64(current.SSEEventsTotal-previous.SSEEventsTotal) / seconds
			apiHealth.SSEEventsPerSecond = &value
		}
	}
	var requestDelta, errorDelta int64
	for _, metric := range current.API {
		row := model.APIEndpointHealth{Endpoint: metric.Endpoint, Status: metric.Status, RequestsTotal: metric.RequestsTotal}
		if validInterval {
			if old, ok := previousAPI[endpointStatus{endpoint: metric.Endpoint, status: metric.Status}]; ok && metric.RequestsTotal >= old.RequestsTotal && metric.ResponseBytesTotal >= old.ResponseBytesTotal && metric.DurationSecondsTotal >= old.DurationSecondsTotal {
				requests := metric.RequestsTotal - old.RequestsTotal
				requestRate := float64(requests) / seconds
				responseRate := float64(metric.ResponseBytesTotal-old.ResponseBytesTotal) / seconds
				row.RequestsPerSecond = &requestRate
				row.ResponseBytesPerSecond = &responseRate
				if requests > 0 {
					averageDuration := (metric.DurationSecondsTotal - old.DurationSecondsTotal) / float64(requests) * 1000
					row.AverageDurationMs = &averageDuration
				}
				requestDelta += requests
				if metric.Status >= 400 {
					errorDelta += requests
				}
			}
		}
		apiHealth.Endpoints = append(apiHealth.Endpoints, row)
	}
	if requestDelta > 0 {
		value := float64(errorDelta) / float64(requestDelta)
		apiHealth.ErrorRate = &value
	}
	return status, dimensions, apiHealth
}
