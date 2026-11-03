package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var lineStatusRoute = "GET /api/v1/status"

// lineRun runs `status --line` with color on, the way a status bar sees it: not a terminal.
func lineRun(t *testing.T, czar *fakeCzar, opts options) result {
	t.Helper()

	opts.color = true
	run := runCLI(t, czar, opts, "status", "--line")
	if run.code != 0 || run.stderr != "" {
		t.Fatalf("exit %d, stderr %q", run.code, run.stderr)
	}

	return run
}

func assertLineGolden(t *testing.T, name string, run result) {
	t.Helper()

	if strings.Count(run.stdout, "\n") != 1 {
		t.Errorf("want one line, got %q", run.stdout)
	}
	assertGolden(t, "line_"+name, showEscapes(run.stdout))
}

func TestLineCallsTheServerAndPrintsTheStatusText(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: ok("status_down_stale")})

	run := lineRun(t, czar, options{})

	assertLineGolden(t, "down_stale", run)
	if got := czar.only().Auth; got != "Bearer "+testToken {
		t.Errorf("Authorization = %q", got)
	}
}

func TestLineDotFollowsTheCounts(t *testing.T) {
	cases := []struct{ fixture, golden string }{
		{"status_up", "up"},
		{"status_stale", "stale"},
		{"status_new", "new"},
		{"status_paused", "paused"},
		{"status_none", "none"},
	}

	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: ok(tc.fixture)})

			assertLineGolden(t, tc.golden, lineRun(t, czar, options{}))
		})
	}
}

func TestLineUnderThirtySecondsOldMakesNoHTTPCall(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: ok("status_down_stale")})
	cacheDir := filepath.Join(t.TempDir(), "pulseczar")
	first := lineRun(t, czar, options{cacheDir: cacheDir})

	second := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(29 * time.Second)})

	if len(czar.requests) != 1 {
		t.Errorf("%d requests, want 1 (the first run's)", len(czar.requests))
	}
	if second.stdout != first.stdout {
		t.Errorf("cached line = %q, want %q", second.stdout, first.stdout)
	}
}

func TestLineThirtySecondsOldCallsAgainAndRefreshesTheCache(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: {{http.StatusOK, "status_down_stale"}, {http.StatusOK, "status_up"}}})
	cacheDir := filepath.Join(t.TempDir(), "pulseczar")
	lineRun(t, czar, options{cacheDir: cacheDir})

	second := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(30 * time.Second)})
	third := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(40 * time.Second)})

	if len(czar.requests) != 2 {
		t.Errorf("%d requests, want 2", len(czar.requests))
	}
	if third.stdout != second.stdout || !strings.Contains(second.stdout, "All up") {
		t.Errorf("second %q, third %q; want the refreshed line twice", second.stdout, third.stdout)
	}
}

func TestLineWhenTheCallFailsShowsTheCachedLineWithItsAge(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: {{http.StatusOK, "status_down_stale"}, {http.StatusServiceUnavailable, ""}}})
	cacheDir := filepath.Join(t.TempDir(), "pulseczar")
	lineRun(t, czar, options{cacheDir: cacheDir})

	run := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(6*time.Minute + 20*time.Second)})

	assertLineGolden(t, "offline_cached", run)
}

func TestLineWhenTheCallFailsAndTheCacheIsAnHourOldIsOffline(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: {{http.StatusOK, "status_down_stale"}, {http.StatusServiceUnavailable, ""}}})
	cacheDir := filepath.Join(t.TempDir(), "pulseczar")
	lineRun(t, czar, options{cacheDir: cacheDir})

	run := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(time.Hour)})

	assertLineGolden(t, "offline", run)
}

func TestLineWithNoCacheAndNoServerIsOffline(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: {{http.StatusServiceUnavailable, ""}}})

	assertLineGolden(t, "offline", lineRun(t, czar, options{}))
}

func TestLineWithNoTokenIsSignedOutAndCallsNothing(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := lineRun(t, czar, options{noToken: true})

	assertLineGolden(t, "signed_out", run)
	if len(czar.requests) != 0 {
		t.Errorf("%d requests, want none", len(czar.requests))
	}
}

func TestLineWhenTheTokenIsRefusedSaysSignInAgainAndDropsTheCache(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: {{http.StatusOK, "status_up"}, {status, ""}}})
			cacheDir := filepath.Join(t.TempDir(), "pulseczar")
			lineRun(t, czar, options{cacheDir: cacheDir})

			run := lineRun(t, czar, options{cacheDir: cacheDir, now: testNow.Add(time.Minute)})

			assertLineGolden(t, "sign_in_again", run)
			if files := cacheFiles(t, cacheDir); len(files) != 0 {
				t.Errorf("cache files left: %v", files)
			}
		})
	}
}

func TestLineWithJSONIsAUsageError(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{}, "status", "--line", "--json")

	assertUsageError(t, run, "--line prints text for a status bar, so it cannot be used with --json")
	if run.stdout != "" || len(czar.requests) != 0 {
		t.Errorf("stdout %q, %d requests", run.stdout, len(czar.requests))
	}
}

func TestLineColorIgnoresTheTerminalCheckButNotNoColorOrDumb(t *testing.T) {
	cases := []struct {
		name      string
		opts      options
		wantColor bool
	}{
		{"piped", options{}, true},
		{"terminal", options{terminal: true}, true},
		{"NO_COLOR", options{env: map[string]string{"NO_COLOR": "1"}}, false},
		{"TERM=dumb", options{env: map[string]string{"TERM": "dumb"}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: ok("status_up")})
			tc.opts.color = true

			run := runCLI(t, czar, tc.opts, "status", "--line")

			if got := strings.Contains(run.stdout, escape); got != tc.wantColor {
				t.Errorf("has color = %v, want %v: %q", got, tc.wantColor, run.stdout)
			}
		})
	}
}

func TestLineCacheIsNamedByServerAndTokenAndHoldsNoToken(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{lineStatusRoute: ok("status_up")})
	cacheDir := filepath.Join(t.TempDir(), "pulseczar")
	lineRun(t, czar, options{cacheDir: cacheDir})
	lineRun(t, czar, options{cacheDir: cacheDir, env: map[string]string{envToken: "2|other-account"}})

	files := cacheFiles(t, cacheDir)

	if len(files) != 2 {
		t.Fatalf("cache files = %v, want one per token", files)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "test-token") || strings.Contains(string(data), "other-account") {
			t.Errorf("%s holds a token: %s", file, data)
		}
	}
}

func TestLoginAndLogoutDeleteTheLineCache(t *testing.T) {
	routes := loginRoutes(reply{http.StatusOK, "signin_approved"})
	routes[lineStatusRoute] = ok("status_up")
	routes["POST /api/v1/auth/logout"] = []reply{{http.StatusNoContent, ""}}

	cases := []struct {
		name string
		opts options
		args []string
	}{
		{"login", options{noToken: true}, []string{"auth", "login"}},
		{"logout", options{savedToken: testToken}, []string{"auth", "logout"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, routes)
			cacheDir := filepath.Join(t.TempDir(), "pulseczar")
			lineRun(t, czar, options{cacheDir: cacheDir})
			if len(cacheFiles(t, cacheDir)) != 1 {
				t.Fatal("the line did not write a cache")
			}

			tc.opts.cacheDir = cacheDir
			run := runCLI(t, czar, tc.opts, tc.args...)

			if run.code != 0 {
				t.Fatalf("exit %d: %s", run.code, run.stderr)
			}
			if files := cacheFiles(t, cacheDir); len(files) != 0 {
				t.Errorf("cache files left: %v", files)
			}
		})
	}
}

func cacheFiles(t *testing.T, dir string) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}

	return files
}
