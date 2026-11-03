// Package cli holds the pulseczar commands.
package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/config"
	"github.com/pulseczar-com/cli/internal/render"
	"github.com/spf13/cobra"
)

// Environment variables that override the saved sign-in.
const (
	envToken = "PULSECZAR_TOKEN"
	envURL   = "PULSECZAR_URL"
)

// guideURL is the web page about the command line.
const guideURL = "https://pulseczar.com/cli"

// rootExamples ends the top help.
const rootExamples = `  pulseczar                                         see your monitors
  pulseczar monitors list --status down             only the monitors that are down
  pulseczar monitors create https://example.com     add a monitor
  pulseczar status --json                           the status as JSON, for scripts
`

// Env is everything the commands take from the outside, so tests can supply it.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	ConfigPath     string
	// CacheDir is where the status line keeps its cache. Empty means no cache.
	CacheDir string
	// IsTerminal is true when stdout is a terminal; it picks the styled screen over the compact form.
	IsTerminal bool
	// StderrIsTerminal is true when stderr is a terminal; it picks the styled error over the plain one.
	StderrIsTerminal bool
	// StdinIsTerminal is true when stdin is a terminal, so a person can press Enter.
	StdinIsTerminal bool
	HTTP            *http.Client
	Sleep           func(time.Duration)
	// Now is the clock for relative times; its zone is the zone times are shown in.
	Now      func() time.Time
	Hostname string
	// OpenBrowser shows an address in the person's browser.
	OpenBrowser func(url string) error
	// Version is the build's version, set when the program is linked.
	Version string
	// Width is the width of the terminal in columns; 0 when unknown.
	Width int
}

type app struct {
	env    Env
	asJSON bool
}

// Run executes the command line and returns the process exit code.
func Run(args []string, env Env) int {
	a := &app{env: env}

	root := a.newRoot()
	// A nil slice would make cobra read the process's own arguments.
	root.SetArgs(append([]string{}, args...))
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)

	err := root.Execute()
	if err == nil {
		return 0
	}

	return a.reportError(err)
}

func (a *app) newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "pulseczar",
		Short: "Check and manage your PulseCzar monitors from the terminal",
		Long: `Check and manage your PulseCzar monitors from the terminal.

Sign in once with "pulseczar auth login". Output is styled on a terminal, the
compact form when piped, and the API body unchanged with --json. Set NO_COLOR to
turn colors off.

Environment:
  PULSECZAR_TOKEN  use this token instead of the saved sign-in
  PULSECZAR_URL    use another PulseCzar server (default ` + config.DefaultURL + `)

Exit codes: 0 ok, 1 error, 2 usage, 4 not signed in or not allowed.`,
		Version:       a.env.Version,
		RunE:          func(cmd *cobra.Command, _ []string) error { return a.runHome() },
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("pulseczar {{.Version}}\n")
	root.SetUsageTemplate(root.UsageTemplate() + `{{if not .HasParent}}
Examples:
` + rootExamples + `{{end}}
Guide: ` + guideURL + "\n")
	root.PersistentFlags().BoolVar(&a.asJSON, "json", false, "print the API body unchanged")

	a.addGroupedCommands(root)

	return root
}

// addGroupedCommands lists the commands in groups in the help.
func (a *app) addGroupedCommands(root *cobra.Command) {
	groups := []struct {
		id, title string
		commands  []*cobra.Command
	}{
		{"start", "Start", []*cobra.Command{a.newStatusCommand(), a.newOpenCommand()}},
		{"monitors", "Monitors", []*cobra.Command{a.newMonitorsCommand()}},
		{"incidents", "Incidents", []*cobra.Command{a.newIncidentsCommand()}},
		{"account", "Account", []*cobra.Command{a.newAuthCommand()}},
	}

	for _, group := range groups {
		root.AddGroup(&cobra.Group{ID: group.id, Title: group.title})
		for _, command := range group.commands {
			command.GroupID = group.id
			root.AddCommand(command)
		}
	}

	root.SetHelpCommandGroupID("start")
}

// print writes a result in the form the caller asked for: the API body with --json,
// the styled screen on a terminal, and the compact form otherwise.
func (a *app) print(raw []byte, compact render.View, screen func(terminal) string) {
	switch {
	case a.asJSON:
		a.env.Stdout.Write(raw)
		if !bytes.HasSuffix(raw, []byte("\n")) {
			io.WriteString(a.env.Stdout, "\n")
		}
	case a.env.IsTerminal:
		io.WriteString(a.env.Stdout, screen(a.stdoutTerminal()))
	default:
		compact.Compact(a.env.Stdout)
	}
}

func (a *app) stdoutTerminal() terminal {
	return terminal{style: a.styleFor(a.env.IsTerminal), now: a.env.Now()}
}

// styleFor paints only on a terminal, and never when the person set NO_COLOR or TERM=dumb.
func (a *app) styleFor(isTerminal bool) render.Style {
	colorAllowed := a.env.Getenv("NO_COLOR") == "" && a.env.Getenv("TERM") != "dumb"

	return render.NewStyle(isTerminal && colorAllowed)
}

// settings is the server and token a command uses, with where the token came from.
type settings struct {
	URL         string
	Token       string
	TokenSource string
}

// loadSettings applies the order: environment, then the saved file, then the default.
func (a *app) loadSettings() (settings, error) {
	file, err := config.Load(a.env.ConfigPath)
	if err != nil {
		return settings{}, err
	}

	found := settings{URL: config.DefaultURL}
	if file.URL != "" {
		found.URL = file.URL
	}
	if file.Token != "" {
		found.Token, found.TokenSource = file.Token, "config file"
	}

	if value := a.env.Getenv(envURL); value != "" {
		found.URL = strings.TrimRight(value, "/")
	}
	if value := a.env.Getenv(envToken); value != "" {
		found.Token, found.TokenSource = value, envToken
	}

	return found, nil
}

// signedInClient returns a client with a token, or the not-signed-in error.
func (a *app) signedInClient() (*api.Client, error) {
	found, err := a.loadSettings()
	if err != nil {
		return nil, fail(err)
	}

	return a.clientFor(found)
}

func (a *app) clientFor(found settings) (*api.Client, error) {
	if found.Token == "" {
		return nil, notSignedIn()
	}

	return &api.Client{BaseURL: found.URL, Token: found.Token, HTTP: a.env.HTTP}, nil
}
