package cli

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const approveAddress = "https://pulseczar.com/cli/approve"

func approvedLogin() map[string][]reply {
	return loginRoutes(pending(), reply{http.StatusOK, "signin_approved"})
}

func TestLoginOnATerminalShowsTheCodeFirstThenOpensTheAddressTheServerSent(t *testing.T) {
	czar := newFakeCzar(t, approvedLogin())

	run := runCLI(t, czar, options{noToken: true, terminal: true, stderrTerminal: true, stdinTerminal: true}, "auth", "login")

	if run.code != 0 {
		t.Fatalf("exit %d: %s", run.code, run.stderr)
	}
	assertGolden(t, "login.terminal.stderr", run.stderr)
	assertGolden(t, "login.terminal.stdout", run.stdout)

	// The address is exactly what the server sent; the code stays out of it.
	if want := []string{approveAddress}; !reflect.DeepEqual(run.opened, want) {
		t.Errorf("opened = %v, want %v", run.opened, want)
	}
}

func TestLoginWhenTheBrowserDoesNotOpenSaysSoAndStillShowsTheAddress(t *testing.T) {
	czar := newFakeCzar(t, approvedLogin())

	run := runCLI(t, czar, options{noToken: true, stdinTerminal: true, browserFails: true}, "auth", "login")

	assertGolden(t, "login.browser_fails.stderr", run.stderr)
	if run.code != 0 {
		t.Errorf("a browser that does not open must not fail the sign-in; exit %d", run.code)
	}
}

func TestLoginOverSSHOpensNoBrowserAndAsksForNoEnter(t *testing.T) {
	czar := newFakeCzar(t, approvedLogin())

	run := runCLI(t, czar, options{noToken: true, stdinTerminal: true, env: map[string]string{"SSH_CONNECTION": "10.0.0.1 22 10.0.0.2 22"}}, "auth", "login")

	assertGolden(t, "login.stderr", run.stderr) // the same words as a pipe
	if len(run.opened) != 0 {
		t.Errorf("opened %v over SSH", run.opened)
	}
}

func TestLoginWithPipedInputOpensNoBrowser(t *testing.T) {
	czar := newFakeCzar(t, approvedLogin())

	run := runCLI(t, czar, options{noToken: true}, "auth", "login")

	assertGolden(t, "login.stderr", run.stderr)
	if len(run.opened) != 0 {
		t.Errorf("opened %v with stdin not a terminal", run.opened)
	}
	if strings.Contains(run.stderr, "\x1b") {
		t.Errorf("a pipe must get no escape codes: %q", run.stderr)
	}
}

func TestLoginDeniedAndExpiredOnATerminalUsePlainWords(t *testing.T) {
	cases := map[string]string{
		"access_denied": "login.denied.stderr",
		"expired_token": "login.expired.stderr",
	}

	for word, golden := range cases {
		t.Run(word, func(t *testing.T) {
			czar := newFakeCzar(t, loginRoutes(pending(), pollWord(word)))

			run := runCLI(t, czar, options{noToken: true, stderrTerminal: true, stdinTerminal: true}, "auth", "login")

			if run.code != exitAuth {
				t.Errorf("exit = %d, want %d", run.code, exitAuth)
			}
			assertGolden(t, golden, run.stderr)
		})
	}
}
