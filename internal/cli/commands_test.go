package cli

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/pulseczar-com/cli/internal/api"
)

func TestAMonitorNameWorksWhereAnIDWorks(t *testing.T) {
	cases := []struct {
		name  string
		given string
	}{
		{"exact name", "Checkout, API"},
		{"name in another case", "checkout, api"},
		{"start of a name", "check"},
		{"id in lower case", "b2c4d6f8g1h3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{
				"GET /api/v1/board":                 ok("board"),
				"GET /api/v1/monitors/B2C4D6F8G1H3": ok("monitor_http"),
			})

			run := runCLI(t, czar, options{}, "monitors", "show", tc.given)

			if run.code != 0 {
				t.Fatalf("exit %d: %s", run.code, run.stderr)
			}
			if last := czar.requests[len(czar.requests)-1]; last.Path != "/api/v1/monitors/B2C4D6F8G1H3" {
				t.Errorf("asked for %s", last.Path)
			}
		})
	}
}

func TestAnIDIsNotLookedUpOnTheBoard(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/monitors/B2C4D6F8G1H3": ok("monitor_http")})

	runCLI(t, czar, options{}, "monitors", "show", "B2C4D6F8G1H3")

	czar.only()
}

func TestEveryMonitorCommandTakesAName(t *testing.T) {
	cases := [][]string{
		{"monitors", "pause", "checkout"},
		{"monitors", "resume", "checkout"},
		{"monitors", "unsnooze", "checkout"},
		{"monitors", "snooze", "checkout", "24h"},
	}

	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			routes := map[string][]reply{"GET /api/v1/board": ok("board")}
			for _, route := range []string{"pause", "mute"} {
				routes["POST /api/v1/monitors/B2C4D6F8G1H3/"+route] = ok("monitor_http")
			}
			czar := newFakeCzar(t, routes)

			run := runCLI(t, czar, options{}, args...)

			if run.code != 0 {
				t.Fatalf("exit %d: %s", run.code, run.stderr)
			}
		})
	}
}

func TestANameThatMatchesNothingIsNotFound(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

	run := runCLI(t, czar, options{}, "monitors", "show", "zzz")

	if run.code != exitError {
		t.Errorf("exit = %d, want %d", run.code, exitError)
	}
	want := "error: no monitor named \"zzz\"\ncode: not_found\n" + listMonitorsHint + "\n"
	if run.stderr != want {
		t.Errorf("stderr = %q, want %q", run.stderr, want)
	}
}

func TestPickMonitor(t *testing.T) {
	monitors := []api.MonitorSummary{
		{ID: "AAAAAAAAAAA1", Name: "API"},
		{ID: "AAAAAAAAAAA2", Name: "API gateway"},
		{ID: "AAAAAAAAAAA3", Name: "Billing"},
		{ID: "AAAAAAAAAAA4", Name: "Billing job"},
		{ID: "AAAAAAAAAAA5", Name: "billing job"},
	}

	cases := []struct {
		name, given, wantID, wantMessage string
	}{
		{"an exact name beats a longer name that starts the same", "api", "AAAAAAAAAAA1", ""},
		{"a word inside a name is not a start", "gate", "", `no monitor named "gate"`},
		{"a unique start of a name", "API g", "AAAAAAAAAAA2", ""},
		{"two names start the same", "bil", "", `"bil" matches 3 monitors`},
		{"two exact names", "billing job", "", `"billing job" matches 2 monitors`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := pickMonitor(monitors, tc.given)

			if tc.wantMessage == "" {
				if err != nil || id != tc.wantID {
					t.Errorf("got %q, %v; want %q", id, err, tc.wantID)
				}

				return
			}
			if err == nil || err.Error() != tc.wantMessage {
				t.Errorf("error = %v, want %q", err, tc.wantMessage)
			}
		})
	}
}

func TestTwoMatchesListBothMonitors(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board_similar")})

	run := runCLI(t, czar, options{}, "monitors", "show", "billing")

	if run.code != exitUsage {
		t.Fatalf("exit = %d, want %d; stderr %q", run.code, exitUsage, run.stderr)
	}
	for _, want := range []string{"Billing API  C1C1C1C1C1C1", "Billing job  D2D2D2D2D2D2", "hint: give more of the name, or the id"} {
		if !strings.Contains(run.stderr, want) {
			t.Errorf("stderr %q lacks %q", run.stderr, want)
		}
	}
}

func TestMonitorsAndIncidentsAloneList(t *testing.T) {
	cases := []struct {
		route, fixture string
		alone, listed  []string
	}{
		{"GET /api/v1/board", "board", []string{"monitors"}, []string{"monitors", "list"}},
		{"GET /api/v1/board", "board", []string{"monitors", "ls"}, []string{"monitors", "list"}},
		{"GET /api/v1/board", "board", []string{"monitors", "--status", "down"}, []string{"monitors", "list", "--status", "down"}},
		{"GET /api/v1/incidents", "incidents", []string{"incidents"}, []string{"incidents", "list"}},
		{"GET /api/v1/incidents", "incidents", []string{"incidents", "--open"}, []string{"incidents", "list", "--open"}},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.alone, " "), func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{tc.route: ok(tc.fixture)})

			alone := runCLI(t, czar, options{terminal: true}, tc.alone...)
			listed := runCLI(t, czar, options{terminal: true}, tc.listed...)

			if alone.code != 0 || alone.stdout != listed.stdout {
				t.Errorf("exit %d, stderr %q\n--- alone\n%s\n--- listed\n%s", alone.code, alone.stderr, alone.stdout, listed.stdout)
			}
		})
	}
}

func TestMonitorsAloneRejectsAStrayWord(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{}, "monitors", "lst")

	if run.code != exitUsage || !strings.Contains(run.stderr, `unknown command "lst"`) {
		t.Errorf("exit %d, stderr %q", run.code, run.stderr)
	}
}

func TestIncidentsTakeAMonitorName(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/board":     ok("board"),
		"GET /api/v1/incidents": ok("incidents"),
	})

	runCLI(t, czar, options{}, "incidents", "--monitor", "checkout")

	if got := czar.requests[1].Query.Get("monitor"); got != "B2C4D6F8G1H3" {
		t.Errorf("monitor = %q, want the id", got)
	}
}

func TestTheNextPageLineIsTheFullCommand(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/incidents": ok("incidents")})

	run := runCLI(t, czar, options{terminal: true}, "incidents", "--open", "--since", "7d", "--monitor", "B2C4D6F8G1H3")

	want := "  Next page: pulseczar incidents --open --since 7d --monitor B2C4D6F8G1H3 --cursor eyJpZCI6NDJ9\n"
	if !strings.HasSuffix(run.stdout, want) {
		t.Errorf("stdout = %q, want it to end with %q", run.stdout, want)
	}
}

func TestShellWordQuotesOnlyWhatAShellWouldChange(t *testing.T) {
	cases := map[string]string{
		"eyJpZCI6NDJ9": "eyJpZCI6NDJ9",
		"a+b":          "'a+b'",
		"it's":         `'it'\''s'`,
	}

	for word, want := range cases {
		if got := shellWord(word); got != want {
			t.Errorf("shellWord(%q) = %q, want %q", word, got, want)
		}
	}
}

func TestPlainArgumentErrors(t *testing.T) {
	cases := []struct {
		args    []string
		message string
	}{
		{[]string{"monitors", "snooze", "a", "1h", "x"}, "too many arguments: got 3, want at most 2"},
		{[]string{"monitors", "create", "a", "b"}, "give one address to check, not 2"},
		{[]string{"open", "a", "b"}, "give one monitor name or id, not 2"},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			czar := newFakeCzar(t, nil)

			assertUsageError(t, runCLI(t, czar, options{}, tc.args...), tc.message)
		})
	}
}

func TestOpenShowsTheDashboardOrAMonitorPage(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

	dashboard := runCLI(t, czar, options{}, "open")
	monitor := runCLI(t, czar, options{}, "open", "checkout")

	want := []string{czar.server.URL + "/"}
	if !reflect.DeepEqual(dashboard.opened, want) {
		t.Errorf("dashboard opened %v, want %v", dashboard.opened, want)
	}
	want = []string{czar.server.URL + "/monitors/B2C4D6F8G1H3"}
	if !reflect.DeepEqual(monitor.opened, want) {
		t.Errorf("monitor opened %v, want %v", monitor.opened, want)
	}
	if got := dashboard.stdout; got != "✓ Opened https://czar.test/\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestOpenNeedsNoSignInForTheDashboard(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{noToken: true}, "open")

	if run.code != 0 || len(run.opened) != 1 {
		t.Errorf("exit %d, opened %v, stderr %q", run.code, run.opened, run.stderr)
	}
}

func TestOpenWhenNoBrowserStartsPrintsTheAddress(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{browserFails: true}, "open")

	if run.code != exitError {
		t.Errorf("exit = %d, want %d", run.code, exitError)
	}
	if !strings.Contains(run.stderr, "could not open a browser: no browser") || !strings.Contains(run.stderr, "hint: open https://czar.test/ yourself") {
		t.Errorf("stderr = %q", run.stderr)
	}
}

func TestShellCompletionOffersMonitorNames(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"show", []string{"monitors", "show", ""}, boardNameCompletions},
		{"pause", []string{"monitors", "pause", ""}, boardNameCompletions},
		{"open", []string{"open", ""}, boardNameCompletions},
		{"snooze monitor", []string{"monitors", "snooze", ""}, boardNameCompletions},
		{"snooze length", []string{"monitors", "snooze", "checkout", ""}, snoozeLengths},
		{"incidents flag", []string{"incidents", "--monitor", ""}, boardNameCompletions},
		{"nothing after the monitor", []string{"monitors", "show", "checkout", ""}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

			run := runCLI(t, czar, options{}, append([]string{"__complete"}, tc.args...)...)

			got := completionsIn(run.stdout)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("completions = %q, want %q (stderr %q)", got, tc.want, run.stderr)
			}
		})
	}
}

var boardNameCompletions = []string{
	"Marketing site\t7K3M9XQ2T4VB",
	"Checkout, API\tB2C4D6F8G1H3",
	"Nightly backup\tM5N7P9Q1R3S5",
	"Old blog\tT2V4W6X8Y0Z1",
}

// completionsIn drops cobra's trailing ":<directive>" line and the stderr note.
func completionsIn(stdout string) []string {
	var found []string
	for _, line := range strings.Split(stdout, "\n") {
		if line != "" && !strings.HasPrefix(line, ":") {
			found = append(found, line)
		}
	}

	return found
}

func TestShellCompletionStaysSilentWhenTheBoardCannotBeRead(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": {{http.StatusInternalServerError, ""}}})

	failing := runCLI(t, czar, options{}, "__complete", "monitors", "show", "")
	signedOut := runCLI(t, czar, options{noToken: true}, "__complete", "monitors", "show", "")

	for name, run := range map[string]result{"server error": failing, "signed out": signedOut} {
		if got := completionsIn(run.stdout); len(got) != 0 || run.code != 0 {
			t.Errorf("%s: completions %q, exit %d", name, got, run.code)
		}
	}
}
