package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestExitCodes(t *testing.T) {
	cases := []struct {
		name   string
		routes map[string][]reply
		opts   options
		args   []string
		exit   int
		code   string
	}{
		{"ok", map[string][]reply{"GET /api/v1/status": ok("status")}, options{}, []string{"status"}, 0, ""},
		{"not found", map[string][]reply{"GET /api/v1/monitors/NOPE": {{http.StatusNotFound, "not_found"}}}, options{}, []string{"monitors", "show", "NOPE"}, exitError, "not_found"},
		{"rate limited", map[string][]reply{"GET /api/v1/status": {{http.StatusTooManyRequests, ""}}}, options{}, []string{"status"}, exitError, "rate_limited"},
		{"unknown command", nil, options{}, []string{"frobnicate"}, exitUsage, "usage"},
		{"unknown flag", nil, options{}, []string{"status", "--nope"}, exitUsage, "usage"},
		{"missing argument", nil, options{}, []string{"monitors", "show"}, exitUsage, "usage"},
		{"no token", nil, options{noToken: true}, []string{"status"}, exitAuth, "not_signed_in"},
		{"rejected token", map[string][]reply{"GET /api/v1/status": {{http.StatusUnauthorized, "unauthenticated"}}}, options{}, []string{"status"}, exitAuth, "unauthenticated"},
		{"read only token", map[string][]reply{"POST /api/v1/monitors/7K3M9XQ2T4VB/pause": {{http.StatusForbidden, "forbidden"}}}, options{}, []string{"monitors", "pause", "7K3M9XQ2T4VB"}, exitAuth, "forbidden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, tc.routes)

			run := runCLI(t, czar, tc.opts, tc.args...)

			if run.code != tc.exit {
				t.Fatalf("exit = %d, want %d; stderr %q", run.code, tc.exit, run.stderr)
			}
			if tc.exit == 0 {
				return
			}
			assertErrorLines(t, run.stderr, tc.code)
		})
	}
}

// An error prints `error:` and then `code:`, whatever else is printed before or after.
func assertErrorLines(t *testing.T, stderr, code string) {
	t.Helper()

	lines := strings.Split(stderr, "\n")
	for i, line := range lines[:len(lines)-1] {
		if strings.HasPrefix(line, "error: ") && lines[i+1] == "code: "+code {
			return
		}
	}

	t.Errorf("stderr = %q, want `error: ...` then `code: %s`", stderr, code)
}

func TestAnUnreachableServerIsANetworkError(t *testing.T) {
	czar := newFakeCzar(t, nil)
	czar.server.Close()

	run := runCLI(t, czar, options{}, "status")

	if run.code != exitError {
		t.Errorf("exit = %d, want %d", run.code, exitError)
	}
	assertErrorLines(t, run.stderr, "network_error")
}
