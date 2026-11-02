package api

import (
	"net/http"
	"net/url"
)

// Each method returns the decoded body and the raw body. The CLI asks for no
// bars on any route: no command shows them, and the server sends none unless asked.

// GetMe returns the signed-in account.
func (c *Client) GetMe() (Me, []byte, error) {
	return call[Me](c, http.MethodGet, "me", nil, nil)
}

// GetStatus returns the status chip and the counts.
func (c *Client) GetStatus() (Status, []byte, error) {
	return call[Status](c, http.MethodGet, "status", nil, nil)
}

// GetBoard returns one row for every monitor.
func (c *Client) GetBoard() (Board, []byte, error) {
	return call[Board](c, http.MethodGet, "board", nil, nil)
}

// GetMonitor returns one monitor.
func (c *Client) GetMonitor(id string) (MonitorDetail, []byte, error) {
	return call[MonitorDetail](c, http.MethodGet, "monitors/"+url.PathEscape(id), nil, nil)
}

// IncidentFilter narrows GET /incidents. An empty field sends nothing.
type IncidentFilter struct {
	Open      bool
	Range     string
	MonitorID string
	Cursor    string
}

// ListIncidents returns one page of incidents, newest first.
func (c *Client) ListIncidents(filter IncidentFilter) (IncidentList, []byte, error) {
	query := url.Values{}
	if filter.Open {
		query.Set("status", "open")
	}
	if filter.Range != "" {
		query.Set("range", filter.Range)
	}
	if filter.MonitorID != "" {
		query.Set("monitor", filter.MonitorID)
	}
	if filter.Cursor != "" {
		query.Set("cursor", filter.Cursor)
	}

	return call[IncidentList](c, http.MethodGet, "incidents", query, nil)
}

// GetIncident returns one incident.
func (c *Client) GetIncident(id string) (IncidentDetail, []byte, error) {
	return call[IncidentDetail](c, http.MethodGet, "incidents/"+url.PathEscape(id), nil, nil)
}

// MonitorCreate is the body of POST /monitors. Zero fields are left out.
type MonitorCreate struct {
	Type            string `json:"type"`
	Name            string `json:"name,omitempty"`
	URL             string `json:"url,omitempty"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	GraceSeconds    *int   `json:"grace_seconds,omitempty"`
}

// CreateMonitor adds an http or heartbeat monitor.
func (c *Client) CreateMonitor(body MonitorCreate) (MonitorDetail, []byte, error) {
	return call[MonitorDetail](c, http.MethodPost, "monitors", nil, body)
}

// SetPaused pauses or resumes a monitor. It sets the state and never toggles.
func (c *Client) SetPaused(id string, paused bool) (MonitorDetail, []byte, error) {
	return call[MonitorDetail](c, http.MethodPost, "monitors/"+url.PathEscape(id)+"/pause", nil,
		map[string]bool{"paused": paused})
}

// SetSnooze snoozes alerts for a monitor. Czar calls this "mute"; mode "off" unmutes.
func (c *Client) SetSnooze(id, mode string) (MonitorDetail, []byte, error) {
	return call[MonitorDetail](c, http.MethodPost, "monitors/"+url.PathEscape(id)+"/mute", nil,
		map[string]string{"mode": mode})
}
