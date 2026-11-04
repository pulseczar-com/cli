package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestHomeScreenSignedIn(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board"),
	})

	run := runCLI(t, czar, options{terminal: true, width: 80})

	assertGolden(t, "home.signed_in", run.stdout)
}

func TestHomeScreenSignedInWithColor(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board"),
	})

	run := runCLI(t, czar, options{terminal: true, color: true, width: 80})

	assertGolden(t, "home.signed_in.color", showEscapes(run.stdout))
}

func TestHomeScreenWithNoMonitors(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board_empty"),
	})

	run := runCLI(t, czar, options{terminal: true, width: 80})

	assertGolden(t, "home.no_monitors", run.stdout)
}

func TestHomeScreenAndMonitorListSayNoMonitorsOnceInTheSameWords(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board_empty"),
	})

	home := runCLI(t, czar, options{terminal: true, width: 80})
	list := runCLI(t, czar, options{terminal: true, width: 80}, "monitors", "list")

	if list.stdout != "No monitors.\n" {
		t.Errorf("monitors list = %q", list.stdout)
	}
	if strings.Count(home.stdout, "No monitors") != 1 || strings.Contains(home.stdout, "yet") {
		t.Errorf("home screen should say %q once:\n%s", "No monitors", home.stdout)
	}
}

func TestHomeScreenListsTenMonitorsAndCountsTheRest(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board_many"),
	})

	run := runCLI(t, czar, options{terminal: true, width: 80})

	if !strings.Contains(run.stdout, "and 3 more · pulseczar monitors list") {
		t.Errorf("no count of the rest:\n%s", run.stdout)
	}
	if strings.Count(run.stdout, "Monitor ") != maxHomeMonitors {
		t.Errorf("want %d monitors listed:\n%s", maxHomeMonitors, run.stdout)
	}
}

func TestHomeScreenSignedOutShowsTheLogoThenTheSignInPromptAndExitsFour(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{terminal: true, stderrTerminal: true, noToken: true, width: 80})

	assertGolden(t, "home.signed_out.stdout", run.stdout)
	assertGolden(t, "home.signed_out.stderr", run.stderr)
	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
	if len(czar.requests) != 0 {
		t.Errorf("the signed-out screen made requests: %+v", czar.requests)
	}
}

func TestHomeScreenPaintsTheBrandGreenByTerminalColorSupport(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"truecolor", map[string]string{"COLORTERM": "truecolor"}, "\x1b[38;2;61;220;132m"},
		{"24bit", map[string]string{"COLORTERM": "24bit"}, "\x1b[38;2;61;220;132m"},
		{"256 colors", map[string]string{"TERM": "xterm-256color"}, "\x1b[38;5;78m"},
		{"basic", map[string]string{"TERM": "xterm"}, "\x1b[32m"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runCLI(t, newFakeCzar(t, nil), options{terminal: true, noToken: true, color: true, width: 80, env: tc.env})

			if !strings.Contains(run.stdout, tc.want) {
				t.Errorf("no %q in %q", tc.want, run.stdout)
			}
		})
	}
}

func TestNoColorStillTurnsOffTheBrandColors(t *testing.T) {
	run := runCLI(t, newFakeCzar(t, nil), options{terminal: true, noToken: true, width: 80, env: map[string]string{"COLORTERM": "truecolor"}})

	if strings.Contains(run.stdout, escape) {
		t.Errorf("color codes with NO_COLOR set: %q", run.stdout)
	}
}

func TestHomeScreenWithARejectedSavedTokenMatchesTheSignedOutScreen(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    {{http.StatusUnauthorized, "unauthenticated"}},
		"GET /api/v1/board": {{http.StatusUnauthorized, "unauthenticated"}},
	})

	run := runCLI(t, czar, options{terminal: true, stderrTerminal: true, noToken: true, savedToken: "1|saved", width: 80})

	assertGolden(t, "home.signed_out.stdout", run.stdout)
	assertGolden(t, "home.signed_out.stderr", run.stderr)
	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
}

func TestHomeScreenWithARejectedEnvTokenSaysSo(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    {{http.StatusUnauthorized, "unauthenticated"}},
		"GET /api/v1/board": {{http.StatusUnauthorized, "unauthenticated"}},
	})

	run := runCLI(t, czar, options{terminal: true, stderrTerminal: true, width: 80})

	assertGolden(t, "home.signed_out.stdout", run.stdout)
	assertGolden(t, "home.rejected_env_token.stderr", run.stderr)
	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
}

func TestEveryCommandThatFailsSignedOutShowsTheSamePrompt(t *testing.T) {
	rejected := map[string][]reply{"GET /api/v1/board": {{http.StatusUnauthorized, "unauthenticated"}}}
	cases := []struct {
		name string
		czar *fakeCzar
		opts options
	}{
		{"no token", newFakeCzar(t, nil), options{noToken: true, stderrTerminal: true}},
		{"rejected saved token", newFakeCzar(t, rejected), options{stderrTerminal: true, noToken: true, savedToken: "1|saved"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runCLI(t, tc.czar, tc.opts, "monitors", "list")

			if want := "✗ You are not signed in.\n  Run `pulseczar auth login` to sign in.\n"; run.stderr != want {
				t.Errorf("stderr %q, want %q", run.stderr, want)
			}
			if strings.Contains(run.stdout, "░") {
				t.Errorf("a logo on a command that is not the bare command:\n%s", run.stdout)
			}
		})
	}
}

func TestACommandWithARejectedEnvTokenSaysSo(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/board": {{http.StatusUnauthorized, "unauthenticated"}}})

	run := runCLI(t, czar, options{stderrTerminal: true}, "monitors", "list")

	if want := "✗ PULSECZAR_TOKEN was rejected.\n  Unset it or replace it.\n"; run.stderr != want {
		t.Errorf("stderr %q, want %q", run.stderr, want)
	}
}

func TestLogoNeedsAWideTerminal(t *testing.T) {
	czar := newFakeCzar(t, nil)

	narrow := runCLI(t, czar, options{terminal: true, noToken: true, width: minLogoWidth - 1})
	wide := runCLI(t, czar, options{terminal: true, noToken: true, width: minLogoWidth})

	if strings.Contains(narrow.stdout, "░") {
		t.Errorf("logo on a narrow terminal:\n%s", narrow.stdout)
	}
	if !strings.Contains(wide.stdout, "░") {
		t.Errorf("no logo on a terminal %d columns wide:\n%s", minLogoWidth, wide.stdout)
	}
}

func TestBareCommandPipedOrWithJSONIsTheStatusCommand(t *testing.T) {
	cases := []struct {
		name string
		opts options
		args []string
	}{
		{"piped", options{width: 80}, nil},
		{"json", options{terminal: true, width: 80}, []string{"--json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/status": ok("status")})

			bare := runCLI(t, czar, tc.opts, tc.args...)
			status := runCLI(t, czar, tc.opts, append([]string{"status"}, tc.args...)...)

			if bare.stdout != status.stdout || strings.Contains(bare.stdout, "░") {
				t.Errorf("bare run differs from status:\n%s\n--- status\n%s", bare.stdout, status.stdout)
			}
		})
	}
}

func TestProblemMonitorsComeFirst(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{
		"GET /api/v1/me":    ok("me"),
		"GET /api/v1/board": ok("board"),
	})

	run := runCLI(t, czar, options{terminal: true, width: 80})

	order := []string{"Checkout, API", "Nightly backup", "Marketing site", "Old blog"}
	last := -1
	for _, name := range order {
		at := strings.Index(run.stdout, name)
		if at < last {
			t.Fatalf("%q is out of order:\n%s", name, run.stdout)
		}
		last = at
	}
}

func TestRootHelpEndsWithExamples(t *testing.T) {
	czar := newFakeCzar(t, nil)

	root := runCLI(t, czar, options{}, "--help")
	child := runCLI(t, czar, options{}, "status", "--help")

	if !strings.Contains(root.stdout, "Examples:\n  pulseczar ") || !strings.HasSuffix(root.stdout, "Guide: https://pulseczar.com/cli\n") {
		t.Errorf("root help has no examples before the guide:\n%s", root.stdout)
	}
	if strings.Contains(child.stdout, "see your monitors") {
		t.Errorf("subcommand help repeats the root examples:\n%s", child.stdout)
	}
}
