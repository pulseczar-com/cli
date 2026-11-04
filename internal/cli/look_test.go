package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/config"
	"github.com/pulseczar-com/cli/internal/render"
)

const escape = "\x1b"

// showEscapes makes color codes readable in a golden file.
func showEscapes(text string) string {
	return strings.ReplaceAll(text, escape, "<ESC>")
}

func TestColorOnATerminalIsGolden(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

	run := runCLI(t, czar, options{terminal: true, color: true}, "monitors", "list")

	assertGolden(t, "monitors_list.color", showEscapes(run.stdout))
}

func TestNoColorWhenPipedOrWhenThePersonOptsOut(t *testing.T) {
	cases := []struct {
		name string
		opts options
	}{
		{"piped", options{color: true}},
		{"NO_COLOR", options{terminal: true, color: true, env: map[string]string{"NO_COLOR": "1"}}},
		{"TERM=dumb", options{terminal: true, color: true, env: map[string]string{"TERM": "dumb"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board")})

			run := runCLI(t, czar, tc.opts, "monitors", "list")

			if strings.Contains(run.stdout, escape) {
				t.Errorf("stdout has color codes: %q", run.stdout)
			}
		})
	}
}

func TestAMonitorWithNoResultYetShowsNewAndNeverUp(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board_new")})

	terminal := runCLI(t, czar, options{terminal: true}, "monitors", "list")
	compact := runCLI(t, czar, options{}, "monitors", "list")

	assertGolden(t, "monitors_list_new.table", terminal.stdout)
	assertGolden(t, "monitors_list_new.compact", compact.stdout)
}

func TestStatusNewFiltersByTheWebsWord(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": ok("board_new")})

	onlyNew := runCLI(t, czar, options{}, "monitors", "list", "--status", "new")
	onlyUp := runCLI(t, czar, options{}, "monitors", "list", "--status", "up")

	if !strings.Contains(onlyNew.stdout, "count: 1") || !strings.Contains(onlyNew.stdout, "N8P1Q3R5S7T9") {
		t.Errorf("--status new: %q", onlyNew.stdout)
	}
	if !strings.Contains(onlyUp.stdout, "count: 1") || strings.Contains(onlyUp.stdout, "N8P1Q3R5S7T9") {
		t.Errorf("--status up counted the new monitor: %q", onlyUp.stdout)
	}
}

func TestCreateOnATerminalConfirmsAndShowsTheHeartbeatPingAddress(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		args    []string
	}{
		{"monitors_create_http.table", "monitor_created", []string{"https://example.com"}},
		{"monitors_create_heartbeat.table", "monitor_heartbeat", []string{"--heartbeat", "--name", "Nightly backup", "--every", "1d", "--grace", "1h"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors": {{http.StatusCreated, tc.fixture}}})

			run := runCLI(t, czar, options{terminal: true}, append([]string{"monitors", "create"}, tc.args...)...)

			assertGolden(t, tc.name, run.stdout)
		})
	}
}

func TestLogoutOnATerminalMarksTheResultDone(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/auth/logout": {{http.StatusNoContent, ""}}})
	path := t.TempDir() + "/config.json"
	if err := config.Save(path, config.File{URL: czar.server.URL, Token: "1|saved"}); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	code := Run([]string{"auth", "logout"}, Env{
		Stdout: &out, Stderr: &out, ConfigPath: path, HTTP: czar.server.Client(), IsTerminal: true,
		Getenv: func(key string) string { return map[string]string{"NO_COLOR": "1"}[key] },
		Now:    func() time.Time { return testNow },
	})

	if code != 0 || out.String() != "✓ Logged out.\n" {
		t.Errorf("exit %d, output %q", code, out.String())
	}
}

func TestAnErrorOnAStderrTerminalIsStyledAndDropsTheCodeLine(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{noToken: true, stderrTerminal: true}, "status")

	assertGolden(t, "error_not_signed_in.stderr_terminal", run.stderr)
	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
}

func TestAReadOnlySignInOnATerminalSaysHowToGetWriteAccess(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/monitors/7K3M9XQ2T4VB/pause": {{http.StatusForbidden, "forbidden"}}})

	run := runCLI(t, czar, options{stderrTerminal: true}, "monitors", "pause", "7K3M9XQ2T4VB")

	assertGolden(t, "error_read_only.stderr_terminal", run.stderr)
	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
}

func TestAnErrorColorsOnlyTheMark(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{noToken: true, stderrTerminal: true, color: true}, "status")

	assertGolden(t, "error_not_signed_in.color", showEscapes(run.stderr))
}

func TestStderrAndStdoutAreCheckedOneByOne(t *testing.T) {
	czar := newFakeCzar(t, nil)

	stdoutOnly := runCLI(t, czar, options{noToken: true, terminal: true, color: true}, "status")

	if strings.Contains(stdoutOnly.stderr, escape) || !strings.Contains(stdoutOnly.stderr, "code: not_signed_in") {
		t.Errorf("a piped stderr got the styled error: %q", stdoutOnly.stderr)
	}
}

func TestParserErrorsUsePlainWords(t *testing.T) {
	cases := []struct {
		args    []string
		message string
	}{
		{[]string{"monitors", "show"}, "give a monitor name or id; run `pulseczar monitors list` to see them"},
		{[]string{"monitors", "pause"}, "give a monitor name or id; run `pulseczar monitors list` to see them"},
		{[]string{"incidents", "show"}, "give an incident id; run `pulseczar incidents list` to see them"},
		{[]string{"monitors", "show", "a", "b"}, "too many arguments: got 2, want at most 1"},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			run := runCLI(t, newFakeCzar(t, nil), options{}, tc.args...)

			assertUsageError(t, run, tc.message)
		})
	}
}

func TestVersionPrintsTheBuildVersion(t *testing.T) {
	run := runCLI(t, newFakeCzar(t, nil), options{}, "--version")

	if run.code != 0 || run.stdout != "pulseczar 1.2.3\n" {
		t.Errorf("exit %d, stdout %q", run.code, run.stdout)
	}
}

func TestHelpIsGroupedAndPointsToTheGuide(t *testing.T) {
	run := runCLI(t, newFakeCzar(t, nil), options{}, "--help")

	for _, want := range []string{"Start\n", "Monitors\n", "Incidents\n", "Account\n", "Guide: https://pulseczar.com/cli"} {
		if !strings.Contains(run.stdout, want) {
			t.Errorf("help is missing %q:\n%s", want, run.stdout)
		}
	}
	if strings.Contains(strings.ToLower(run.stdout), "watch") {
		t.Errorf("help says watch; the product word is monitor:\n%s", run.stdout)
	}
}

func TestStatusDotIsNeverGreenWithoutMonitors(t *testing.T) {
	cases := []struct {
		name   string
		counts api.Counts
		want   render.Color
	}{
		{"no monitors", api.Counts{}, render.Gray},
		{"all up", api.Counts{Up: 2, Total: 2}, render.Green},
		{"one stale", api.Counts{Up: 1, Stale: 1, Total: 2}, render.Yellow},
		{"down wins over stale", api.Counts{Down: 1, Stale: 1, Total: 2}, render.Red},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusColor(tc.counts); got != tc.want {
				t.Errorf("color = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestATimestampTheCLIcannotReadShowsAsSent(t *testing.T) {
	screen := terminal{style: render.NewStyle(false, render.Basic), now: testNow}

	if got := screen.ago("last Tuesday"); got != "last Tuesday" {
		t.Errorf("got %q", got)
	}
}

func TestAnUnreachableServerOnATerminalNamesTheServerAndTheFix(t *testing.T) {
	czar := newFakeCzar(t, nil)
	czar.server.Close()

	terminal := runCLI(t, czar, options{stderrTerminal: true}, "status")
	piped := runCLI(t, czar, options{}, "status")

	assertGolden(t, "error_unreachable.stderr_terminal", terminal.stderr)
	if !strings.HasPrefix(piped.stderr, "error: Get ") || !strings.Contains(piped.stderr, "\ncode: network_error\n") {
		t.Errorf("the piped form changed: %q", piped.stderr)
	}
	if terminal.code != exitError || piped.code != exitError {
		t.Errorf("exit codes = %d, %d; want %d", terminal.code, piped.code, exitError)
	}
}

func TestAMistypedCommandPutsTheSuggestionOnTheHintLine(t *testing.T) {
	czar := newFakeCzar(t, nil)

	piped := runCLI(t, czar, options{}, "statu")
	terminal := runCLI(t, czar, options{stderrTerminal: true}, "statu")

	wantPiped := "error: unknown command \"statu\" for \"pulseczar\"\ncode: usage\nhint: did you mean `status`?\n"
	if piped.stderr != wantPiped {
		t.Errorf("piped stderr = %q, want %q", piped.stderr, wantPiped)
	}
	wantTerminal := "✗ unknown command \"statu\" for \"pulseczar\"\n  did you mean `status`?\n"
	if terminal.stderr != wantTerminal {
		t.Errorf("terminal stderr = %q, want %q", terminal.stderr, wantTerminal)
	}
}

func TestAnUnknownCommandWithNoSuggestionKeepsTheHelpHint(t *testing.T) {
	run := runCLI(t, newFakeCzar(t, nil), options{}, "frobnicate")

	want := "error: unknown command \"frobnicate\" for \"pulseczar\"\ncode: usage\nhint: run `pulseczar --help`\n"
	if run.stderr != want {
		t.Errorf("stderr = %q, want %q", run.stderr, want)
	}
}

func TestCompletionScriptsAreAvailableForEachShell(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			run := runCLI(t, newFakeCzar(t, nil), options{}, "completion", shell)

			if run.code != 0 || !strings.Contains(run.stdout, "pulseczar") {
				t.Errorf("exit %d, stdout %q", run.code, run.stdout)
			}
		})
	}
}
