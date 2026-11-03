package api

// These types hold only the fields the CLI shows. --json prints the body as the
// server sent it, so a field added to the API needs no change here.

// Me is GET /me.
type Me struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	PlanWord     string `json:"plan_word"`
	MonitorCount int    `json:"monitor_count"`
	MonitorLimit int    `json:"monitor_limit"`
}

// Counts are the monitor totals by state.
type Counts struct {
	Up     int `json:"up"`
	Down   int `json:"down"`
	Stale  int `json:"stale"`
	Paused int `json:"paused"`
	Total  int `json:"total"`
}

// Status is GET /status.
type Status struct {
	Summary struct {
		Text string `json:"text"`
	} `json:"summary"`
	Counts     Counts  `json:"counts"`
	CzarNotice *string `json:"czar_notice"`
}

// Board is GET /board.
type Board struct {
	Summary struct {
		Text string `json:"text"`
	} `json:"summary"`
	Counts   Counts           `json:"counts"`
	Monitors []MonitorSummary `json:"monitors"`
}

// MonitorSummary is one monitor on the board.
type MonitorSummary struct {
	ID       string   `json:"public_id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	URL      *string  `json:"url"`
	Status   string   `json:"status"`
	Word     string   `json:"word"`
	Uptime24 *float64 `json:"uptime_24h"`
	// FootLeft is the text on the web card's footer, such as the last error.
	FootLeft string `json:"foot_left"`
}

// MonitorDetail is GET /monitors/{id}, and the answer to every write.
type MonitorDetail struct {
	MonitorSummary
	MuteMode        *string           `json:"mute_mode"`
	MutedUntil      *string           `json:"muted_until"`
	LastCheckedAt   *string           `json:"last_checked_at"`
	Schedule        string            `json:"schedule"`
	IntervalSeconds int               `json:"interval_seconds"`
	PingURL         *string           `json:"ping_url"`
	GraceSeconds    *int              `json:"grace_seconds"`
	LastHeardAt     *string           `json:"last_heard_at"`
	RecentIncidents []IncidentSummary `json:"recent_incidents"`
	IncidentsTotal  int               `json:"incidents_total"`
}

// IncidentList is GET /incidents.
type IncidentList struct {
	Incidents  []IncidentSummary `json:"incidents"`
	HasMore    bool              `json:"has_more"`
	NextCursor *string           `json:"next_cursor"`
}

// IncidentSummary is one incident in a list.
type IncidentSummary struct {
	ID           string             `json:"public_id"`
	MonitorID    string             `json:"monitor_public_id"`
	MonitorName  string             `json:"monitor_name"`
	StartedAt    string             `json:"started_at"`
	FailingSince *string            `json:"failing_since"`
	EndedAt      *string            `json:"ended_at"`
	StatusWord   string             `json:"status_word"`
	DurationWord *string            `json:"duration_word"`
	Scope        *string            `json:"scope"`
	Locations    []IncidentLocation `json:"locations"`
}

// IncidentLocation is one location's vote in an incident.
type IncidentLocation struct {
	Location string `json:"location"`
	OK       bool   `json:"ok"`
	Silent   bool   `json:"silent"`
}

// IncidentDetail is GET /incidents/{id}.
type IncidentDetail struct {
	IncidentSummary
	QuorumUsed      int `json:"quorum_used"`
	ActiveNodeCount int `json:"active_node_count"`
}
