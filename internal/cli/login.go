package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/config"
	"github.com/pulseczar-com/cli/internal/render"
	"github.com/spf13/cobra"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultSignInWindow = 10 * time.Minute
	// The RFC 8628 step a client adds to its interval after a slow_down answer.
	slowDownStep = 5 * time.Second
)

func (a *app) runLogin(cmd *cobra.Command, _ []string) error {
	found, err := a.loadSettings()
	if err != nil {
		return fail(err)
	}

	client := &api.Client{BaseURL: found.URL, HTTP: a.env.HTTP}

	start, err := client.StartSignIn(a.env.Hostname)
	if err != nil {
		return fail(err)
	}

	a.showCode(start)

	token, err := a.awaitApproval(client, start)
	if err != nil {
		return fail(err)
	}

	if err := config.Save(a.env.ConfigPath, config.File{URL: found.URL, Token: token}); err != nil {
		return fail(err)
	}

	a.clearStatusCaches()

	client.Token = token
	me, raw, err := client.GetMe()
	if err != nil {
		return fail(err)
	}

	a.print(raw, meView(me, found.URL, "config file"), func(t terminal) string {
		return t.signedInScreen(me, a.homeRelative(a.env.ConfigPath))
	})

	return nil
}

// canOpenBrowser is false over SSH, where the browser is on another computer, and
// when stdin is not a terminal, where nobody can press Enter.
func (a *app) canOpenBrowser() bool {
	return a.env.StdinIsTerminal && a.env.Getenv("SSH_CONNECTION") == ""
}

// showCode tells the person what to do next, by opening the browser or by printing
// what to open elsewhere.
func (a *app) showCode(start api.SignInStart) {
	if a.canOpenBrowser() {
		a.openBrowserAfterEnter(start)
	} else {
		a.printAddressAndCode(start)
	}
}

func (a *app) printAddressAndCode(start api.SignInStart) {
	fmt.Fprintf(a.env.Stderr, "Open this address in a browser on any device:\n  %s\n\nThen type this code:  %s\n\n", start.VerificationURI, start.UserCode)
}

// openBrowserAfterEnter shows the code first and opens the browser only after Enter, so
// the person has the code before the page asks for it. The address is exactly what the
// server sent; the code is never added to it.
func (a *app) openBrowserAfterEnter(start api.SignInStart) {
	style := a.styleFor(a.env.StderrIsTerminal)
	out := a.env.Stderr

	fmt.Fprintf(out, "%s First copy your code: %s. Press Enter to open %s in your browser.\n",
		style.Paint(render.Yellow, "!"), start.UserCode, siteName(start.VerificationURI))
	bufio.NewReader(a.env.Stdin).ReadString('\n') // An end of input also continues.

	if err := a.env.OpenBrowser(start.VerificationURI); err != nil {
		fmt.Fprintf(out, "Could not open the browser. Go to this address:\n  %s\n\n", start.VerificationURI)

		return
	}

	fmt.Fprintf(out, "If the browser did not open, go to:\n  %s\n\n", start.VerificationURI)
}

func siteName(address string) string {
	if parsed, err := url.Parse(address); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}

	return "PulseCzar"
}

// awaitApproval polls until the person approves or denies, or the code expires.
func (a *app) awaitApproval(client *api.Client, start api.SignInStart) (string, error) {
	interval := time.Duration(start.Interval) * time.Second
	if interval <= 0 {
		interval = defaultPollInterval
	}

	window := time.Duration(start.ExpiresIn) * time.Second
	if window <= 0 {
		window = defaultSignInWindow
	}

	wait := newWaitLine(a.env.Stderr, a.env.StderrIsTerminal)
	defer wait.clear()

	for waited := time.Duration(0); waited < window; waited += interval {
		wait.show(window - waited)
		a.env.Sleep(interval)

		result, err := client.PollSignIn(start.DeviceCode)
		if err != nil {
			return "", err
		}

		switch result.Error {
		case "":
			return result.Token, nil
		case api.PollPending:
		case api.PollSlowDown:
			interval += slowDownStep
		case api.PollDenied:
			return "", deniedSignIn()
		case api.PollExpired:
			return "", expiredSignIn()
		default:
			return "", &commandError{exit: exitError, code: codeUnexpectedAnswer, message: "unexpected sign-in answer: " + result.Error}
		}
	}

	return "", expiredSignIn()
}

func deniedSignIn() error {
	return &commandError{
		exit:       exitAuth,
		code:       codeAccessDenied,
		message:    "the sign-in was denied",
		onTerminal: &terminalText{message: "The sign-in was denied in the browser."},
	}
}

func expiredSignIn() error {
	return &commandError{
		exit:       exitAuth,
		code:       codeExpiredToken,
		message:    "the code expired before it was approved",
		details:    []string{"hint: run `pulseczar auth login` again"},
		onTerminal: &terminalText{message: "The code expired. Run pulseczar auth login again."},
	}
}

// waitLine is the "Waiting" line. On a terminal it is one line that changes at every
// poll; in a pipe it is one plain line, printed once.
type waitLine struct {
	out      io.Writer
	terminal bool
	frame    int
	shown    bool
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const eraseLine = "\r\x1b[K"

func newWaitLine(out io.Writer, terminal bool) *waitLine {
	return &waitLine{out: out, terminal: terminal}
}

func (w *waitLine) show(remaining time.Duration) {
	if !w.terminal {
		if !w.shown {
			fmt.Fprintln(w.out, "Waiting for you to approve the code...")
		}
		w.shown = true

		return
	}

	seconds := int(remaining.Seconds())
	fmt.Fprintf(w.out, "%s%s Waiting for you to approve · code expires in %d:%02d · Ctrl-C to stop",
		eraseLine, spinnerFrames[w.frame%len(spinnerFrames)], seconds/60, seconds%60)
	w.frame++
	w.shown = true
}

// clear removes the changing line so the result starts on a clean line.
func (w *waitLine) clear() {
	if w.terminal && w.shown {
		io.WriteString(w.out, eraseLine)
	}
}

// homeRelative writes a path under the home directory as ~/..., as the person knows it.
func (a *app) homeRelative(path string) string {
	home := a.env.Getenv("HOME")
	if home != "" && strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}

	return path
}
