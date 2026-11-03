package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/render"
)

// terminal builds the screens a person sees on a terminal: marks, colors, human
// labels and relative times. The piped form never goes through it.
type terminal struct {
	style render.Style
	now   time.Time
}

// labeled is one `Label  value` line of a detail screen.
type labeled struct {
	label, value string
}

var wordColors = map[string]render.Color{
	"up":       render.Green,
	"resolved": render.Green,
	"down":     render.Red,
	"stale":    render.Yellow,
	"paused":   render.Gray,
	"new":      render.Gray,
}

var locationColors = map[string]render.Color{
	"up":      render.Green,
	"silent":  render.Yellow,
	"failing": render.Red,
}

// statusKey is a status word as the compact form and --status write it: "new", "up".
func statusKey(word string) string {
	return strings.ToLower(word)
}

// titleWord writes a status word the way the web does: "Up", "Down", "Resolved".
func titleWord(word string) string {
	key := statusKey(word)
	if key == "" {
		return noValue
	}

	return strings.ToUpper(key[:1]) + key[1:]
}

func (t terminal) wordCell(word string) render.Cell {
	return render.Cell{Text: titleWord(word), Color: wordColors[statusKey(word)]}
}

// dot is the status mark: filled for up, down and stale, hollow for paused, dotted for new.
func (t terminal) dot(word string) string {
	mark := "●"
	switch statusKey(word) {
	case "paused":
		mark = "○"
	case "new":
		mark = "◌"
	}

	return t.style.Paint(wordColors[statusKey(word)], mark)
}

func (t terminal) ago(timestamp string) string {
	when, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}

	return relativeTime(t.now, when)
}

func (t terminal) agoOrDash(timestamp *string) string {
	if timestamp == nil {
		return noValue
	}

	return t.ago(*timestamp)
}

// withMarks puts a status dot before each data row of aligned lines. The first line
// is the header and gets a blank where the dot would be.
func (t terminal) withMarks(lines, words []string) string {
	var out strings.Builder

	for i, line := range lines {
		mark := " "
		if i > 0 {
			mark = t.dot(words[i-1])
		}
		fmt.Fprintf(&out, " %s %s\n", mark, line)
	}

	return out.String()
}

// details writes labeled lines in two columns, indented under a heading.
func (t terminal) details(lines []labeled) string {
	rows := make([][]render.Cell, len(lines))
	for i, line := range lines {
		rows[i] = []render.Cell{{Text: line.label}, {Text: line.value}}
	}

	return t.indented(rows)
}

// indented aligns rows in columns and indents each line two spaces.
func (t terminal) indented(rows [][]render.Cell) string {
	var out strings.Builder
	for _, line := range t.style.Align(rows) {
		fmt.Fprintf(&out, "  %s\n", line)
	}

	return out.String()
}

func (t terminal) heading(word, title string) string {
	return fmt.Sprintf("%s %s  %s\n", t.dot(word), title, t.style.Paint(wordColors[statusKey(word)], titleWord(word)))
}

func (t terminal) gray(text string) string {
	return t.style.Paint(render.Gray, text)
}

func (t terminal) statusScreen(status api.Status) string {
	counts := status.Counts
	line := fmt.Sprintf("%d up · %d down · %d stale · %d paused · %d total",
		counts.Up, counts.Down, counts.Stale, counts.Paused, counts.Total)

	var out strings.Builder
	fmt.Fprintf(&out, "%s %s     %s\n", t.style.Paint(statusColor(counts), "●"), status.Summary.Text, t.gray(line))

	if status.CzarNotice != nil {
		fmt.Fprintf(&out, "\n%s\n", *status.CzarNotice)
	}

	return out.String()
}

// statusColor never shows green for an account with no monitors.
func statusColor(counts api.Counts) render.Color {
	switch {
	case counts.Down > 0:
		return render.Red
	case counts.Stale > 0:
		return render.Yellow
	case counts.Total == 0:
		return render.Gray
	default:
		return render.Green
	}
}

func (t terminal) meScreen(me api.Me, serverURL, tokenSource string) string {
	done := t.style.Paint(render.Green, render.MarkDone)

	return fmt.Sprintf("%s Signed in as %s\n", done, me.Email) + t.details([]labeled{
		{"Name", me.Name},
		{"Plan", me.PlanWord},
		{"Monitors", fmt.Sprintf("%d of %d", me.MonitorCount, me.MonitorLimit)},
		{"Server", serverURL},
		{"Token from", tokenSource},
	})
}

func (t terminal) monitorListScreen(monitors []api.MonitorSummary) string {
	if len(monitors) == 0 {
		return "No monitors.\n"
	}

	rows := [][]render.Cell{{
		{Text: "NAME", Color: render.Gray}, {Text: "STATUS", Color: render.Gray},
		{Text: "UPTIME 24H", Color: render.Gray}, {Text: "ID", Color: render.Gray},
	}}
	words := make([]string, len(monitors))

	for i, monitor := range monitors {
		words[i] = monitor.Word
		rows = append(rows, []render.Cell{
			{Text: monitor.Name},
			t.wordCell(monitor.Word),
			{Text: uptimeText(monitor.Uptime24)},
			{Text: monitor.ID, Color: render.Gray},
		})
	}

	return t.withMarks(t.style.Align(rows), words)
}

func (t terminal) monitorScreen(monitor api.MonitorDetail, webURL string) string {
	var out strings.Builder

	out.WriteString(t.heading(monitor.Word, monitor.Name))
	fmt.Fprintf(&out, "  %s\n\n", t.gray(t.monitorSummary(monitor)))
	out.WriteString(t.details(t.monitorDetails(monitor)))

	if len(monitor.RecentIncidents) > 0 {
		out.WriteString("\n  Recent incidents\n")
		out.WriteString(t.recentIncidents(monitor.RecentIncidents))
	}

	fmt.Fprintf(&out, "\n  On the web: %s/monitors/%s\n", strings.TrimRight(webURL, "/"), monitor.ID)

	return out.String()
}

// monitorSummary is the gray line under the name: the address, the type and the schedule.
func (t terminal) monitorSummary(monitor api.MonitorDetail) string {
	parts := monitorFacts(monitor)
	if monitor.URL != nil {
		parts = append([]string{*monitor.URL}, parts...)
	}

	return strings.Join(parts, " · ")
}

// monitorFacts are the type, the check length and, for a heartbeat, the grace.
func monitorFacts(monitor api.MonitorDetail) []string {
	if monitor.Type != "heartbeat" {
		return []string{"HTTP", "every " + formatSeconds(monitor.IntervalSeconds)}
	}

	facts := []string{"Heartbeat", "every " + formatSeconds(monitor.IntervalSeconds)}
	if monitor.GraceSeconds != nil {
		facts = append(facts, "grace "+formatSeconds(*monitor.GraceSeconds))
	}

	return facts
}

func (t terminal) monitorDetails(monitor api.MonitorDetail) []labeled {
	lines := []labeled{{"Uptime 24h", uptimeText(monitor.Uptime24)}}

	if monitor.Type == "heartbeat" {
		lines = append(lines, labeled{"Last heard", t.agoOrDash(monitor.LastHeardAt)})
	} else {
		lines = append(lines, labeled{"Last checked", t.agoOrDash(monitor.LastCheckedAt)})
	}

	if monitor.MuteMode != nil {
		lines = append(lines, labeled{"Snoozed", t.snoozeSentence(monitor)})
	}

	return append(lines, labeled{"Incidents", strconv.Itoa(monitor.IncidentsTotal) + " in total"})
}

// snoozeSentence says when alerts come back: "until 18:00 (in 1 hour)".
func (t terminal) snoozeSentence(monitor api.MonitorDetail) string {
	switch *monitor.MuteMode {
	case "next_incident":
		return "until the next incident ends"
	case "forever":
		return "until you run monitors unsnooze"
	}

	until, err := time.Parse(time.RFC3339, textOrDash(monitor.MutedUntil))
	if err != nil {
		return snoozeText(monitor)
	}

	return fmt.Sprintf("until %s (%s)", absoluteTime(t.now, until), relativeTime(t.now, until))
}

func (t terminal) recentIncidents(incidents []api.IncidentSummary) string {
	rows := make([][]render.Cell, len(incidents))
	for i, incident := range incidents {
		rows[i] = []render.Cell{
			{Text: incident.ID, Color: render.Gray},
			t.wordCell(incident.StatusWord),
			{Text: t.incidentWhen(incident)},
			{Text: textOrDash(incident.DurationWord)},
		}
	}

	return t.indented(rows)
}

// incidentWhen says "started 48 minutes ago" for an open incident, and when it ended otherwise.
func (t terminal) incidentWhen(incident api.IncidentSummary) string {
	if incident.EndedAt == nil {
		return "started " + t.ago(incident.StartedAt)
	}

	return t.ago(*incident.EndedAt)
}

// createdScreen confirms a new monitor. A heartbeat monitor also gets its secret
// ping address, which is the next thing the person needs.
func (t terminal) createdScreen(monitor api.MonitorDetail) string {
	done := t.style.Paint(render.Green, render.MarkDone)
	facts := strings.Join(monitorFacts(monitor), ", ")

	out := fmt.Sprintf("%s Created %s (%s)\n", done, monitor.Name, facts)
	if monitor.PingURL != nil {
		out += fmt.Sprintf("\n  Ping this address at the end of each run:\n  curl -fsS %s\n", *monitor.PingURL)
	}

	return out
}

func (t terminal) incidentListScreen(page api.IncidentList, filter api.IncidentFilter) string {
	if len(page.Incidents) == 0 {
		return "No incidents.\n"
	}

	rows := [][]render.Cell{{
		{Text: "MONITOR", Color: render.Gray}, {Text: "STATUS", Color: render.Gray},
		{Text: "STARTED", Color: render.Gray}, {Text: "DURATION", Color: render.Gray},
		{Text: "ID", Color: render.Gray},
	}}
	words := make([]string, len(page.Incidents))

	for i, incident := range page.Incidents {
		words[i] = incident.StatusWord
		rows = append(rows, []render.Cell{
			{Text: incident.MonitorName},
			t.wordCell(incident.StatusWord),
			{Text: t.ago(incident.StartedAt)},
			{Text: textOrDash(incident.DurationWord)},
			{Text: incident.ID, Color: render.Gray},
		})
	}

	out := t.withMarks(t.style.Align(rows), words)
	if page.HasMore && page.NextCursor != nil {
		out += fmt.Sprintf("\n  Next page: %s\n", nextPageCommand(filter, *page.NextCursor))
	}

	return out
}

func (t terminal) incidentScreen(incident api.IncidentDetail) string {
	var out strings.Builder

	out.WriteString(t.heading(incident.StatusWord, incident.MonitorName))
	fmt.Fprintf(&out, "  %s\n\n", t.gray("Incident "+incident.ID))
	out.WriteString(t.details([]labeled{
		{"Started", t.ago(incident.StartedAt)},
		{"Failing since", t.agoOrDash(incident.FailingSince)},
		{"Ended", t.agoOrDash(incident.EndedAt)},
		{"Duration", textOrDash(incident.DurationWord)},
		{"Scope", strings.TrimSuffix(textOrDash(incident.Scope), ":")},
		{"Quorum", fmt.Sprintf("%d of %d", incident.QuorumUsed, incident.ActiveNodeCount)},
	}))

	if len(incident.Locations) > 0 {
		out.WriteString("\n  Locations\n")
		out.WriteString(t.locations(incident.Locations))
	}

	return out.String()
}

func (t terminal) locations(locations []api.IncidentLocation) string {
	rows := make([][]render.Cell, len(locations))
	for i, location := range locations {
		rows[i] = []render.Cell{{Text: location.Location}, t.locationCell(location)}
	}

	return t.indented(rows)
}

func (t terminal) locationCell(location api.IncidentLocation) render.Cell {
	state := locationState(location)

	return render.Cell{Text: state, Color: locationColors[state]}
}

// nextPageCommand is the full command that shows the page after this one.
func nextPageCommand(filter api.IncidentFilter, cursor string) string {
	words := []string{"pulseczar", "incidents"}
	if filter.Open {
		words = append(words, "--open")
	}
	if filter.Range != "" {
		words = append(words, "--since", filter.Range)
	}
	if filter.MonitorID != "" {
		words = append(words, "--monitor", filter.MonitorID)
	}

	return strings.Join(append(words, "--cursor", shellWord(cursor)), " ")
}

// shellWord quotes a word only when a shell would split or change it.
func shellWord(word string) string {
	if strings.Trim(word, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_=.,:/@") == "" {
		return word
	}

	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
