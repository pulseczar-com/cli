package cli

import (
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pulseczar-com/cli/internal/config"
)

// pollWord is a poll answer that carries one of the RFC 8628 words.
func pollWord(word string) reply { return reply{http.StatusBadRequest, "poll_" + word} }

func pending() reply { return pollWord("authorization_pending") }

func loginRoutes(polls ...reply) map[string][]reply {
	return map[string][]reply{
		"POST /api/v1/auth/cli":       ok("signin_start"),
		"POST /api/v1/auth/cli/token": polls,
		"GET /api/v1/me":              ok("me"),
	}
}

func TestLoginPrintsTheAddressAndCodeThenSavesTheToken(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(pending(), pending(), reply{http.StatusOK, "signin_approved"}))

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	if run.code != 0 {
		t.Fatalf("exit %d: %s", run.code, run.stderr)
	}

	// The address and the code are on separate lines, and the code is not in a link.
	assertGolden(t, "login.stderr", run.stderr)
	assertGolden(t, "login.compact", run.stdout)

	saved, err := config.Load(run.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := (config.File{URL: czar.server.URL, Token: "1|secret-token-value"}); saved != want {
		t.Errorf("saved = %+v, want %+v", saved, want)
	}
}

func TestLoginSavesTheTokenWithMode0600(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(reply{http.StatusOK, "signin_approved"}))

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	info, err := os.Stat(run.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoginStartsWithTheHostnameAndPollsWithTheDeviceCode(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(reply{http.StatusOK, "signin_approved"}))

	runCLI(t, czar, options{noToken: true}, "auth", "login")

	start, poll := czar.requests[0], czar.requests[1]
	if !reflect.DeepEqual(start.Body, map[string]any{"device_name": "ada-laptop"}) {
		t.Errorf("start body = %v", start.Body)
	}
	wantPoll := map[string]any{
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
		"device_code": strings.Repeat("D", 40),
	}
	if !reflect.DeepEqual(poll.Body, wantPoll) {
		t.Errorf("poll body = %v", poll.Body)
	}
	if start.Auth != "" || poll.Auth != "" {
		t.Errorf("sign-in calls must send no token")
	}
}

func TestLoginWaitsTheIntervalBeforeEveryPollAndSlowsDownWhenTold(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(pending(), pollWord("slow_down"), reply{http.StatusOK, "signin_approved"}))

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	want := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second}
	if !reflect.DeepEqual(run.sleeps, want) {
		t.Errorf("sleeps = %v, want %v", run.sleeps, want)
	}
}

func TestLoginStopsWhenTheSignInIsDeniedOrExpired(t *testing.T) {
	cases := []struct {
		word string
		code string
	}{
		{"access_denied", "access_denied"},
		{"expired_token", "expired_token"},
	}

	for _, tc := range cases {
		t.Run(tc.word, func(t *testing.T) {
			czar := newFakeCzar(t, loginRoutes(pending(), pollWord(tc.word)))

			run := runCLI(t, czar, options{noToken: true}, "auth", "login")

			if run.code != exitAuth {
				t.Errorf("exit = %d, want %d", run.code, exitAuth)
			}
			assertErrorLines(t, run.stderr, tc.code)
			if _, err := os.Stat(run.configPath); !os.IsNotExist(err) {
				t.Errorf("a failed sign-in must save nothing")
			}
		})
	}
}

func TestLoginGivesUpWhenTheCodeWindowPasses(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(pending()))

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
	assertErrorLines(t, run.stderr, "expired_token")
	if len(run.sleeps) != 120 {
		t.Errorf("polled %d times, want 120 (600s at 5s)", len(run.sleeps))
	}
}

func TestLoginRejectsAnUnknownAnswer(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(pollWord("invalid_grant")))

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	assertErrorLines(t, run.stderr, "unexpected_response")
}

func TestPulseczarURLMayEndInASlash(t *testing.T) {
	czar := newFakeCzar(t, loginRoutes(reply{http.StatusOK, "signin_approved"}))

	run := runCLI(t, czar, options{noToken: true, env: map[string]string{envURL: czar.server.URL + "/"}}, "auth", "login")

	if run.code != 0 {
		t.Fatalf("exit %d: %s", run.code, run.stderr)
	}
	if czar.requests[0].Path != "/api/v1/auth/cli" {
		t.Errorf("path = %q", czar.requests[0].Path)
	}
}

func TestTheSavedTokenIsUsedAndTheEnvironmentTokenWinsOverIt(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"GET /api/v1/status": ok("status")})
	path := t.TempDir() + "/config.json"
	if err := config.Save(path, config.File{URL: czar.server.URL, Token: "1|saved"}); err != nil {
		t.Fatal(err)
	}

	run := func(env map[string]string) string {
		czar.requests = nil
		var out strings.Builder
		Run([]string{"status"}, Env{
			Stdout: &out, Stderr: &out, ConfigPath: path, HTTP: czar.server.Client(),
			Getenv: func(key string) string { return env[key] },
		})

		return czar.only().Auth
	}

	if got := run(nil); got != "Bearer 1|saved" {
		t.Errorf("saved token: Authorization = %q", got)
	}
	if got := run(map[string]string{envToken: "1|from-env"}); got != "Bearer 1|from-env" {
		t.Errorf("env token: Authorization = %q", got)
	}
}

func TestLogoutRevokesTheTokenAndDeletesTheFile(t *testing.T) {
	czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/auth/logout": {{http.StatusNoContent, ""}}})
	path := t.TempDir() + "/config.json"
	if err := config.Save(path, config.File{URL: czar.server.URL, Token: "1|saved"}); err != nil {
		t.Fatal(err)
	}

	run := runCLIWithConfig(t, czar, path, "auth", "logout")

	if run.code != 0 || run.stdout != "Logged out.\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", run.code, run.stdout, run.stderr)
	}
	if got := czar.only().Auth; got != "Bearer 1|saved" {
		t.Errorf("Authorization = %q", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the config file is still there")
	}
}

func TestLogoutStillDeletesTheFileWhenTheServerRefuses(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantExit int
	}{
		{"token already revoked", http.StatusUnauthorized, 0},
		{"server error", http.StatusInternalServerError, exitError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			czar := newFakeCzar(t, map[string][]reply{"POST /api/v1/auth/logout": {{tc.status, ""}}})
			path := t.TempDir() + "/config.json"
			if err := config.Save(path, config.File{URL: czar.server.URL, Token: "1|saved"}); err != nil {
				t.Fatal(err)
			}

			run := runCLIWithConfig(t, czar, path, "auth", "logout")

			if run.code != tc.wantExit {
				t.Errorf("exit = %d, want %d; stderr %q", run.code, tc.wantExit, run.stderr)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("the config file is still there")
			}
		})
	}
}

func TestLogoutWithNothingSavedIsNotSignedIn(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{noToken: true}, "auth", "logout")

	if run.code != exitAuth {
		t.Errorf("exit = %d, want %d", run.code, exitAuth)
	}
	assertErrorLines(t, run.stderr, "not_signed_in")
}

func TestLogoutLeavesAnEnvironmentTokenAlone(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{}, "auth", "logout")

	if run.code != exitAuth || len(czar.requests) != 0 {
		t.Errorf("exit %d, %d requests", run.code, len(czar.requests))
	}
}

func runCLIWithConfig(t *testing.T, czar *fakeCzar, path string, args ...string) result {
	t.Helper()

	var out, errOut strings.Builder
	code := Run(args, Env{
		Stdout: &out, Stderr: &errOut, ConfigPath: path, HTTP: czar.server.Client(),
		Getenv: func(string) string { return "" },
	})

	return result{stdout: out.String(), stderr: errOut.String(), code: code}
}

func TestTokenPrintsTheSavedTokenAndNothingElse(t *testing.T) {
	czar := newFakeCzar(t, nil)
	path := t.TempDir() + "/config.json"
	if err := config.Save(path, config.File{URL: czar.server.URL, Token: "1|saved"}); err != nil {
		t.Fatal(err)
	}

	run := runCLIWithConfig(t, czar, path, "auth", "token")

	if run.code != 0 || run.stdout != "1|saved\n" || run.stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", run.code, run.stdout, run.stderr)
	}
	if len(czar.requests) != 0 {
		t.Errorf("%d requests, want none", len(czar.requests))
	}
}

func TestTokenWithNothingSavedIsNotSignedIn(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{noToken: true}, "auth", "token")

	if run.code != exitAuth || run.stdout != "" {
		t.Errorf("exit %d, stdout %q", run.code, run.stdout)
	}
	assertErrorLines(t, run.stderr, "not_signed_in")
}

func TestTokenPrintsTheEnvironmentTokenInPlaceOfTheSavedOne(t *testing.T) {
	czar := newFakeCzar(t, nil)

	run := runCLI(t, czar, options{savedToken: "1|saved", env: map[string]string{envToken: "1|from-env"}}, "auth", "token")

	if run.code != 0 || run.stdout != "1|from-env\n" || run.stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", run.code, run.stdout, run.stderr)
	}
}
