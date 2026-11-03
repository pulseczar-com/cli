package cli

import (
	"fmt"
	"strconv"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/render"
)

// Every function here turns one API result into a render.View. A null value shows as "-".

const noValue = "-"

func meView(me api.Me, serverURL, tokenSource string) render.View {
	return render.View{Fields: []render.Field{
		{Key: "account", Value: me.Email},
		{Key: "name", Value: me.Name},
		{Key: "plan", Value: me.PlanWord},
		{Key: "monitors", Value: fmt.Sprintf("%d of %d", me.MonitorCount, me.MonitorLimit)},
		{Key: "server", Value: serverURL},
		{Key: "token_from", Value: tokenSource},
	}}
}

func statusView(status api.Status) render.View {
	fields := []render.Field{
		{Key: "status", Value: status.Summary.Text},
		{Key: "up", Value: strconv.Itoa(status.Counts.Up)},
		{Key: "down", Value: strconv.Itoa(status.Counts.Down)},
		{Key: "stale", Value: strconv.Itoa(status.Counts.Stale)},
		{Key: "paused", Value: strconv.Itoa(status.Counts.Paused)},
		{Key: "total", Value: strconv.Itoa(status.Counts.Total)},
	}

	if status.CzarNotice != nil {
		fields = append(fields, render.Field{Key: "notice", Value: *status.CzarNotice})
	}

	return render.View{Fields: fields}
}

func monitorListView(monitors []api.MonitorSummary) render.View {
	rows := make([][]string, len(monitors))
	for i, monitor := range monitors {
		rows[i] = []string{monitor.ID, monitor.Name, statusKey(monitor.Word), uptimeText(monitor.Uptime24)}
	}

	return render.View{
		ShowCount: true,
		Lists:     []render.List{{Name: "monitors", Columns: []string{"id", "name", "status", "uptime_24h"}, Rows: rows}},
	}
}

func monitorView(monitor api.MonitorDetail) render.View {
	fields := []render.Field{
		{Key: "id", Value: monitor.ID},
		{Key: "name", Value: monitor.Name},
		{Key: "type", Value: monitor.Type},
		{Key: "status", Value: statusKey(monitor.Word)},
		{Key: "uptime_24h", Value: uptimeText(monitor.Uptime24)},
		{Key: "url", Value: textOrDash(monitor.URL)},
		{Key: "every", Value: formatSeconds(monitor.IntervalSeconds)},
		{Key: "snoozed", Value: snoozeText(monitor)},
		{Key: "last_checked_at", Value: textOrDash(monitor.LastCheckedAt)},
		{Key: "incidents_total", Value: strconv.Itoa(monitor.IncidentsTotal)},
	}

	if monitor.Type == "heartbeat" {
		fields = append(fields,
			render.Field{Key: "ping_url", Value: textOrDash(monitor.PingURL)},
			render.Field{Key: "grace", Value: graceText(monitor.GraceSeconds)},
			render.Field{Key: "last_heard_at", Value: textOrDash(monitor.LastHeardAt)},
		)
	}

	return render.View{
		Fields: fields,
		Lists:  []render.List{recentIncidentsList(monitor.RecentIncidents)},
	}
}

func recentIncidentsList(incidents []api.IncidentSummary) render.List {
	rows := make([][]string, len(incidents))
	for i, incident := range incidents {
		rows[i] = []string{incident.ID, incident.StatusWord, incident.StartedAt, textOrDash(incident.DurationWord)}
	}

	return render.List{Name: "recent_incidents", Columns: []string{"id", "status", "started_at", "duration"}, Rows: rows}
}

func incidentListView(page api.IncidentList) render.View {
	rows := make([][]string, len(page.Incidents))
	for i, incident := range page.Incidents {
		rows[i] = []string{
			incident.ID, incident.MonitorID, incident.MonitorName,
			incident.StatusWord, incident.StartedAt, textOrDash(incident.DurationWord),
		}
	}

	view := render.View{
		ShowCount: true,
		Lists: []render.List{{
			Name:    "incidents",
			Columns: []string{"id", "monitor_id", "monitor", "status", "started_at", "duration"},
			Rows:    rows,
		}},
	}

	if page.HasMore && page.NextCursor != nil {
		view.Fields = []render.Field{{Key: "next_cursor", Value: *page.NextCursor}}
	}

	return view
}

func incidentView(incident api.IncidentDetail) render.View {
	locations := make([][]string, len(incident.Locations))
	for i, location := range incident.Locations {
		locations[i] = []string{location.Location, locationState(location)}
	}

	return render.View{
		Fields: []render.Field{
			{Key: "id", Value: incident.ID},
			{Key: "monitor_id", Value: incident.MonitorID},
			{Key: "monitor", Value: incident.MonitorName},
			{Key: "status", Value: incident.StatusWord},
			{Key: "started_at", Value: incident.StartedAt},
			{Key: "failing_since", Value: textOrDash(incident.FailingSince)},
			{Key: "ended_at", Value: textOrDash(incident.EndedAt)},
			{Key: "duration", Value: textOrDash(incident.DurationWord)},
			{Key: "scope", Value: textOrDash(incident.Scope)},
			{Key: "quorum", Value: fmt.Sprintf("%d of %d", incident.QuorumUsed, incident.ActiveNodeCount)},
		},
		Lists: []render.List{{Name: "locations", Columns: []string{"location", "state"}, Rows: locations}},
	}
}

func locationState(location api.IncidentLocation) string {
	switch {
	case location.Silent:
		return "silent"
	case location.OK:
		return "up"
	default:
		return "failing"
	}
}

func uptimeText(uptime *float64) string {
	if uptime == nil {
		return noValue
	}

	return strconv.FormatFloat(*uptime, 'f', -1, 64) + "%"
}

func textOrDash(text *string) string {
	if text == nil {
		return noValue
	}

	return *text
}

func graceText(seconds *int) string {
	if seconds == nil {
		return noValue
	}

	return formatSeconds(*seconds)
}

// snoozeText says whether alerts are snoozed and until when.
func snoozeText(monitor api.MonitorDetail) string {
	if monitor.MuteMode == nil {
		return "no"
	}

	switch *monitor.MuteMode {
	case "next_incident":
		return "until the next incident ends"
	case "until":
		return "until " + textOrDash(monitor.MutedUntil)
	default:
		return *monitor.MuteMode
	}
}
