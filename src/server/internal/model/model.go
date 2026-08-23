package model

import "time"

type SourceState string

const (
	StateConnecting   SourceState = "connecting"
	StateOnline       SourceState = "online"
	StateStale        SourceState = "stale"
	StateOffline      SourceState = "offline"
	StateUnauthorized SourceState = "unauthorized"
)

type Connection struct {
	ID              string     `json:"id"`
	State           string     `json:"state"`
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
	ClosedAt        *time.Time `json:"closedAt"`
	DurationSeconds float64    `json:"durationSeconds"`
	Upload          int64      `json:"upload"`
	Download        int64      `json:"download"`
}

func (c Connection) WithDuration(now time.Time) Connection {
	end := now
	if c.ClosedAt != nil {
		end = *c.ClosedAt
	}
	d := end.Sub(c.StartedAt).Seconds()
	if d < 0 {
		d = 0
	}
	c.DurationSeconds = d
	return c
}

type ConnectionPage struct {
	GeneratedAt time.Time    `json:"generatedAt"`
	Total       int          `json:"total"`
	Limit       int          `json:"limit"`
	Offset      int          `json:"offset"`
	Data        []Connection `json:"data"`
}

type StatusSnapshot struct {
	ObservedAt             time.Time `json:"observedAt"`
	Version                string    `json:"version"`
	UptimeSeconds          float64   `json:"uptimeSeconds"`
	MemoryBytes            int64     `json:"memoryBytes"`
	Goroutines             int64     `json:"goroutines"`
	ActiveConnections      int64     `json:"activeConnections"`
	RecentConnections      int64     `json:"recentConnections"`
	ConnectionsTotal       int64     `json:"connectionsTotal"`
	UploadBytesTotal       int64     `json:"uploadBytesTotal"`
	DownloadBytesTotal     int64     `json:"downloadBytesTotal"`
	UploadBytesPerSecond   *float64  `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond *float64  `json:"downloadBytesPerSecond"`
}

type DimensionSnapshot struct {
	Kind          string
	Value         string
	Active        int64
	Connections   int64
	UploadTotal   int64
	DownloadTotal int64
	DelayMs       *int64
	MeasuredAtMs  *int64
	UploadRate    *float64
	DownloadRate  *float64
}

type URLTestResult struct {
	Outbound   string    `json:"outbound"`
	DelayMs    int64     `json:"delayMs"`
	MeasuredAt time.Time `json:"measuredAt"`
	AgeSeconds float64   `json:"ageSeconds"`
}

type MetricSnapshot struct {
	ObservedAt                time.Time
	Version                   string
	UptimeSeconds             float64
	MemoryBytes               int64
	Goroutines                int64
	ActiveConnections         int64
	RecentConnections         int64
	RecentConnectionsCapacity int64
	ConnectionsTotal          int64
	UploadBytesTotal          int64
	DownloadBytesTotal        int64
	SSESubscribers            int64
	SSEEventsTotal            int64
	API                       []APIMetricCounter
	Dimensions                []DimensionSnapshot
	URLTests                  []URLTestResult
}

type APIMetricCounter struct {
	Endpoint             string
	Status               int
	RequestsTotal        int64
	ResponseBytesTotal   int64
	DurationSecondsTotal float64
}

type Capabilities struct {
	UpstreamAPIVersion    int      `json:"upstreamApiVersion"`
	Endpoints             []string `json:"endpoints"`
	ExposeSensitive       bool     `json:"exposeSensitive"`
	RankingDimensions     []string `json:"rankingDimensions"`
	SensitiveDimensions   []string `json:"sensitiveDimensions"`
	RecentConnectionLimit int      `json:"recentConnectionLimit"`
	RecentTTLSeconds      int64    `json:"recentTtlSeconds"`
	TopKSize              int      `json:"topKSize"`
	ActivePageLimit       int      `json:"activePageLimit"`
	CursorPagination      bool     `json:"cursorPagination"`
	EventReplay           bool     `json:"eventReplay"`
}

type SourceInfo struct {
	State                SourceState `json:"state"`
	DisplayName          string      `json:"displayName"`
	LastAttemptAt        *time.Time  `json:"lastAttemptAt"`
	LastSuccessAt        *time.Time  `json:"lastSuccessAt"`
	LastErrorCode        *string     `json:"lastErrorCode"`
	HistoryAvailableFrom *time.Time  `json:"historyAvailableFrom"`
}

type CollectorInfo struct {
	ScrapeIntervalSeconds  float64           `json:"scrapeIntervalSeconds"`
	PersistIntervalSeconds float64           `json:"persistIntervalSeconds"`
	RetentionSeconds       int64             `json:"retentionSeconds"`
	MaxSeriesPoints        int               `json:"maxSeriesPoints"`
	Channels               CollectorChannels `json:"channels"`
}

type CollectorChannel struct {
	State         SourceState `json:"state"`
	LastAttemptAt *time.Time  `json:"lastAttemptAt"`
	LastSuccessAt *time.Time  `json:"lastSuccessAt"`
	LastErrorCode *string     `json:"lastErrorCode"`
}

type EventChannel struct {
	CollectorChannel
	Reconnects   int64      `json:"reconnects"`
	LastEventAt  *time.Time `json:"lastEventAt"`
	LastSequence uint64     `json:"lastSequence"`
}

type CollectorChannels struct {
	Capabilities CollectorChannel `json:"capabilities"`
	Metrics      CollectorChannel `json:"metrics"`
	Connections  CollectorChannel `json:"connections"`
	Events       EventChannel     `json:"events"`
}

type MetaResponse struct {
	APIVersion   string        `json:"apiVersion"`
	AppVersion   string        `json:"appVersion"`
	GeneratedAt  time.Time     `json:"generatedAt"`
	Source       SourceInfo    `json:"source"`
	Capabilities *Capabilities `json:"capabilities"`
	Collector    CollectorInfo `json:"collector"`
}

type TimeWindow struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	StepSeconds int64     `json:"stepSeconds"`
}

type RangeTotals struct {
	UploadBytes   int64 `json:"uploadBytes"`
	DownloadBytes int64 `json:"downloadBytes"`
	Connections   int64 `json:"connections"`
}

type TimePoint struct {
	Timestamp              time.Time `json:"timestamp"`
	UploadBytesPerSecond   *float64  `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond *float64  `json:"downloadBytesPerSecond"`
	ActiveConnections      *int64    `json:"activeConnections"`
	MemoryBytes            *int64    `json:"memoryBytes"`
	Goroutines             *int64    `json:"goroutines"`
}

type DimensionTimePoint struct {
	Timestamp              time.Time `json:"timestamp"`
	UploadBytesPerSecond   *float64  `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond *float64  `json:"downloadBytesPerSecond"`
	ConnectionsPerSecond   *float64  `json:"connectionsPerSecond"`
	ActiveConnections      *int64    `json:"activeConnections"`
	DelayMs                *int64    `json:"delayMs"`
}

type DimensionSeriesResponse struct {
	GeneratedAt time.Time            `json:"generatedAt"`
	Window      TimeWindow           `json:"window"`
	Dimension   string               `json:"dimension"`
	Value       string               `json:"value"`
	Series      []DimensionTimePoint `json:"series"`
}

type CompactRanking struct {
	Value                  string   `json:"value"`
	UploadBytesPerSecond   *float64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond *float64 `json:"downloadBytesPerSecond"`
	ActiveConnections      int64    `json:"activeConnections"`
}

type APIEndpointHealth struct {
	Endpoint               string   `json:"endpoint"`
	Status                 int      `json:"status"`
	RequestsTotal          int64    `json:"requestsTotal"`
	RequestsPerSecond      *float64 `json:"requestsPerSecond"`
	AverageDurationMs      *float64 `json:"averageDurationMs"`
	ResponseBytesPerSecond *float64 `json:"responseBytesPerSecond"`
}

type APIHealthSnapshot struct {
	RecentConnectionsCapacity    int64               `json:"recentConnectionsCapacity"`
	RecentConnectionsUtilization *float64            `json:"recentConnectionsUtilization"`
	SSESubscribers               int64               `json:"sseSubscribers"`
	SSEEventsTotal               int64               `json:"sseEventsTotal"`
	SSEEventsPerSecond           *float64            `json:"sseEventsPerSecond"`
	ErrorRate                    *float64            `json:"errorRate"`
	Endpoints                    []APIEndpointHealth `json:"endpoints"`
}

type OverviewResponse struct {
	GeneratedAt  time.Time          `json:"generatedAt"`
	SourceState  SourceState        `json:"sourceState"`
	Window       TimeWindow         `json:"window"`
	Current      *StatusSnapshot    `json:"current"`
	RangeTotals  *RangeTotals       `json:"rangeTotals"`
	Series       []TimePoint        `json:"series"`
	TopOutbounds []CompactRanking   `json:"topOutbounds"`
	URLTests     []URLTestResult    `json:"urlTests"`
	APIHealth    *APIHealthSnapshot `json:"apiHealth"`
}

type RankingItem struct {
	Value             string  `json:"value"`
	UploadBytes       int64   `json:"uploadBytes"`
	DownloadBytes     int64   `json:"downloadBytes"`
	Connections       int64   `json:"connections"`
	ActiveConnections int64   `json:"activeConnections"`
	Percentage        float64 `json:"percentage"`
}

type RankingsResponse struct {
	GeneratedAt time.Time     `json:"generatedAt"`
	Window      TimeWindow    `json:"window"`
	Dimension   string        `json:"dimension"`
	Sort        string        `json:"sort"`
	Total       int           `json:"total"`
	Data        []RankingItem `json:"data"`
}

type EventEnvelope struct {
	Sequence    uint64    `json:"sequence"`
	GeneratedAt time.Time `json:"generatedAt"`
	Type        string    `json:"type"`
	Data        any       `json:"data"`
}

type SourceStateEventData struct {
	State         SourceState `json:"state"`
	LastSuccessAt *time.Time  `json:"lastSuccessAt"`
	ErrorCode     *string     `json:"errorCode"`
}

type ResyncEventData struct {
	Reason string `json:"reason"`
}
