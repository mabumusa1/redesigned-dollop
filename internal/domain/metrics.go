package domain

import "time"

// MatchMetrics represents the aggregated metrics for a match.
// Used as the response for GET /api/matches/{matchId}/metrics.
type MatchMetrics struct {
	MatchID                  string                    `json:"matchId"`
	TotalEvents              int64                     `json:"totalEvents"`
	EventsByType             map[string]int64          `json:"eventsByType"`
	Goals                    int64                     `json:"goals"`
	YellowCards              int64                     `json:"yellowCards"`
	RedCards                 int64                     `json:"redCards"`
	FirstEventAt             *time.Time                `json:"firstEventAt,omitempty"`
	LastEventAt              *time.Time                `json:"lastEventAt,omitempty"`
	PeakMinute               *PeakEngagement           `json:"peakMinute,omitempty"`
	ResponseTimePercentiles  *ResponseTimePercentiles  `json:"responseTimePercentiles,omitempty"`
}

// ResponseTimePercentiles represents response time latency percentiles in milliseconds.
type ResponseTimePercentiles struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// PeakEngagement represents the minute with the highest event count.
type PeakEngagement struct {
	Minute     time.Time `json:"minute"`
	EventCount int64     `json:"eventCount"`
}

// EventsPerMinute represents the count of events per minute and type.
type EventsPerMinute struct {
	Minute     time.Time `json:"minute"`
	EventType  string    `json:"eventType"`
	EventCount int64     `json:"eventCount"`
}

// NewMatchMetrics creates a new MatchMetrics with initialized maps.
func NewMatchMetrics(matchID string) *MatchMetrics {
	return &MatchMetrics{
		MatchID:      matchID,
		EventsByType: make(map[string]int64),
	}
}
