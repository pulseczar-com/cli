package cli

import (
	"net/http"
	"reflect"
	"testing"
)

func TestCreateSendsAnHTTPMonitor(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors": {{http.StatusCreated, "monitor_created"}}})

	run := runCLI(t, czar, options{}, "monitors", "create", "https://example.com", "--name", "Example", "--every", "1m")

	want := map[string]any{
		"type": "http", "url": "https://example.com", "name": "Example",
		"interval_seconds": float64(60), "timeout_seconds": float64(10),
	}
	if got := czar.only().Body; !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
	assertGolden(t, "monitors_create.compact", run.stdout)
}

func TestCreateDefaultsToCheckingEveryFiveMinutes(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors": {{http.StatusCreated, "monitor_created"}}})

	runCLI(t, czar, options{}, "monitors", "create", "https://example.com")

	body := czar.only().Body
	if body["interval_seconds"] != float64(300) {
		t.Errorf("interval_seconds = %v, want 300", body["interval_seconds"])
	}
	if _, named := body["name"]; named {
		t.Errorf("name was sent without --name")
	}
}

func TestCreateSendsAHeartbeatMonitor(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors": {{http.StatusCreated, "monitor_heartbeat"}}})

	runCLI(t, czar, options{}, "monitors", "create", "--heartbeat", "--name", "Nightly backup", "--every", "1d", "--grace", "1h")

	want := map[string]any{
		"type": "heartbeat", "name": "Nightly backup",
		"interval_seconds": float64(86400), "grace_seconds": float64(3600),
	}
	if got := czar.only().Body; !reflect.DeepEqual(got, want) {
		t.Errorf("body = %v, want %v", got, want)
	}
}

func TestCreateRejectsBadArgumentsBeforeCallingTheServer(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"no address", []string{"monitors", "create"}, "give the address to check, or use --heartbeat"},
		{"grace on an address", []string{"monitors", "create", "https://example.com", "--grace", "1h"}, "--grace is for heartbeat monitors only"},
		{"bad length", []string{"monitors", "create", "https://example.com", "--every", "soon"}, `--every: "soon" is not a length; use a number and s, m, h or d, such as 5m`},
		{"heartbeat without a name", []string{"monitors", "create", "--heartbeat", "--every", "1d"}, "--heartbeat needs --name and --every"},
		{"heartbeat without an interval", []string{"monitors", "create", "--heartbeat", "--name", "Job"}, "--heartbeat needs --name and --every"},
		{"heartbeat with an address", []string{"monitors", "create", "https://example.com", "--heartbeat", "--name", "Job", "--every", "1d"}, "a heartbeat monitor has no address to check"},
		{"bad grace", []string{"monitors", "create", "--heartbeat", "--name", "Job", "--every", "1d", "--grace", "late"}, `--grace: "late" is not a length; use a number and s, m, h or d, such as 5m`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, nil)

			run := runCLI(t, czar, options{}, tc.args...)

			assertUsageError(t, run, tc.message)
			if len(czar.requests) != 0 {
				t.Errorf("the server was called")
			}
		})
	}
}

func TestPauseAndResumeSetTheState(t *testing.T) {
	cases := []struct {
		verb   string
		paused bool
	}{
		{"pause", true},
		{"resume", false},
	}

	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors/7K3M9XQ2T4VB/pause": ok("monitor_paused")})

			run := runCLI(t, czar, options{}, "monitors", tc.verb, "7K3M9XQ2T4VB")

			if run.code != 0 {
				t.Fatalf("exit %d: %s", run.code, run.stderr)
			}
			if got := czar.only().Body; !reflect.DeepEqual(got, map[string]any{"paused": tc.paused}) {
				t.Errorf("body = %v", got)
			}
		})
	}
}

func TestSnoozeAndUnsnoozeSendTheMuteMode(t *testing.T) {
	cases := []struct {
		args []string
		mode string
	}{
		{[]string{"snooze", "--for", "1h"}, "1h"},
		{[]string{"snooze", "--for", "24h"}, "24h"},
		{[]string{"snooze", "--for", "next"}, "next_incident"},
		{[]string{"snooze", "--for", "forever"}, "forever"},
		{[]string{"unsnooze"}, "off"},
	}

	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors/7K3M9XQ2T4VB/mute": ok("monitor_snoozed")})
			args := append([]string{"monitors", tc.args[0], "7K3M9XQ2T4VB"}, tc.args[1:]...)

			run := runCLI(t, czar, options{}, args...)

			if run.code != 0 {
				t.Fatalf("exit %d: %s", run.code, run.stderr)
			}
			if got := czar.only().Body; !reflect.DeepEqual(got, map[string]any{"mode": tc.mode}) {
				t.Errorf("body = %v", got)
			}
		})
	}
}

func TestSnoozeNeedsAKnownLength(t *testing.T) {
	czar := newFakeCzar(t, nil)

	missing := runCLI(t, czar, options{}, "monitors", "snooze", "7K3M9XQ2T4VB")
	unknown := runCLI(t, czar, options{}, "monitors", "snooze", "7K3M9XQ2T4VB", "--for", "2h")

	if missing.code != exitUsage {
		t.Errorf("missing --for: exit %d, stderr %q", missing.code, missing.stderr)
	}
	assertUsageError(t, unknown, `--for must be one of 1h, 24h, next, forever; got "2h"`)
}

func TestAWriteWithJSONPrintsTheBodyUnchanged(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors/7K3M9XQ2T4VB/pause": ok("monitor_paused")})

	run := runCLI(t, czar, options{}, "monitors", "pause", "7K3M9XQ2T4VB", "--json")

	if run.stdout != string(readFixture(t, "monitor_paused")) {
		t.Errorf("--json changed the body:\n%s", run.stdout)
	}
}

func TestA422PrintsTheFieldMessages(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		golden  string
	}{
		{"plan limit", "invalid_plan", "create_plan_limit.stderr"},
		{"field error", "invalid_duplicate", "create_field_error.stderr"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors": {{http.StatusUnprocessableEntity, tc.fixture}}})

			run := runCLI(t, czar, options{}, "monitors", "create", "https://example.com")

			if run.code != exitError {
				t.Errorf("exit = %d, want %d", run.code, exitError)
			}
			assertGolden(t, tc.golden, run.stderr)
		})
	}
}
