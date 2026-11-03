package cli

import (
	"net/http"
	"testing"
)

// Each read command prints three forms: a table on a terminal, the compact form when
// piped, and the API body unchanged with --json. Golden files hold all three.
func TestReadCommandsPrintTheThreeForms(t *testing.T) {
	cases := []struct {
		name    string
		route   string
		fixture string
		args    []string
	}{
		{"status", "GET /api/v1/status", "status", []string{"status"}},
		{"monitors_list", "GET /api/v1/board", "board", []string{"monitors", "list"}},
		{"monitors_show_http", "GET /api/v1/monitors/B2C4D6F8G1H3", "monitor_http", []string{"monitors", "show", "B2C4D6F8G1H3"}},
		{"monitors_show_heartbeat", "GET /api/v1/monitors/M5N7P9Q1R3S5", "monitor_heartbeat", []string{"monitors", "show", "M5N7P9Q1R3S5"}},
		{"incidents_list", "GET /api/v1/incidents", "incidents", []string{"incidents", "list"}},
		{"incidents_list_empty", "GET /api/v1/incidents", "incidents_empty", []string{"incidents", "list", "--open"}},
		{"incidents_show", "GET /api/v1/incidents/4T9WQ8ZN2HJ5", "incident", []string{"incidents", "show", "4T9WQ8ZN2HJ5"}},
		{"auth_status", "GET /api/v1/me", "me", []string{"auth", "status"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{tc.route: ok(tc.fixture)})

			table := runCLI(t, czar, options{terminal: true}, tc.args...)
			compact := runCLI(t, czar, options{}, tc.args...)
			asJSON := runCLI(t, czar, options{terminal: true}, append(tc.args, "--json")...)

			for _, run := range []result{table, compact, asJSON} {
				if run.code != 0 || run.stderr != "" {
					t.Fatalf("exit %d, stderr %q", run.code, run.stderr)
				}
			}

			assertGolden(t, tc.name+".table", table.stdout)
			assertGolden(t, tc.name+".compact", compact.stdout)
			if asJSON.stdout != string(readFixture(t, tc.fixture)) {
				t.Errorf("--json changed the body:\n%s", asJSON.stdout)
			}
		})
	}
}

func TestMonitorsListFiltersByStatus(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

	run := runCLI(t, czar, options{}, "monitors", "list", "--status", "down")

	assertGolden(t, "monitors_list_down.compact", run.stdout)
}

func TestMonitorsListRejectsAnUnknownStatus(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{}, "monitors", "list", "--status", "sleepy")

	assertUsageError(t, run, `--status must be one of up, down, stale, paused, new; got "sleepy"`)
	if len(czar.requests) != 0 {
		t.Errorf("a usage error must not call the server")
	}
}

func TestIncidentsListSendsItsFiltersAsQueryParameters(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/incidents": ok("incidents")})

	runCLI(t, czar, options{}, "incidents", "list", "--open", "--since", "24h", "--monitor", "B2C4D6F8G1H3", "--cursor", "abc")

	query := czar.only().Query
	want := map[string]string{"status": "open", "range": "24h", "monitor": "B2C4D6F8G1H3", "cursor": "abc"}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("query %s = %q, want %q", key, got, value)
		}
	}
}

// Bars are about 95% of a monitor body and no command shows them, so no read asks.
func TestReadsNeverAskForBars(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/board":                 ok("board"),
		"GET /api/v1/monitors/B2C4D6F8G1H3": ok("monitor_http"),
		"GET /api/v1/status":                ok("status"),
	})

	runCLI(t, czar, options{}, "monitors", "list")
	runCLI(t, czar, options{}, "monitors", "show", "B2C4D6F8G1H3")
	runCLI(t, czar, options{}, "status")

	for _, seen := range czar.requests {
		if _, asked := seen.Query["bars"]; asked {
			t.Errorf("%s %s sent bars", seen.Method, seen.Path)
		}
	}
}

func TestReadsSendTheTokenAsABearerToken(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/status": ok("status")})

	runCLI(t, czar, options{}, "status")

	if got := czar.only().Auth; got != "Bearer "+testToken {
		t.Errorf("Authorization = %q", got)
	}
}

func TestAServerErrorBodyThatIsNotJSONStillPrintsAnError(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/status": {{http.StatusBadGateway, ""}}})

	run := runCLI(t, czar, options{}, "status")

	if run.code != 1 || run.stderr != "error: 502 Bad Gateway\ncode: server_error\n" {
		t.Errorf("exit %d, stderr %q", run.code, run.stderr)
	}
}

func assertUsageError(t *testing.T, run result, wantMessage string) {
	t.Helper()

	if run.code != exitUsage {
		t.Errorf("exit = %d, want %d", run.code, exitUsage)
	}
	want := "error: " + wantMessage + "\ncode: usage\n"
	if len(run.stderr) < len(want) || run.stderr[:len(want)] != want {
		t.Errorf("stderr = %q, want it to start with %q", run.stderr, want)
	}
}
