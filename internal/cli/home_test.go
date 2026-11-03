package cli

import (
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

func TestHomeScreenSignedOutIsAWelcomeWithExitZero(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{terminal: true, noToken: true, width: 80})

	assertGolden(t, "home.signed_out", run.stdout)
	if run.code != 0 {
		t.Errorf("exit code %d, want 0", run.code)
	}
	if len(czar.requests) != 0 {
		t.Errorf("the welcome screen made requests: %+v", czar.requests)
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
