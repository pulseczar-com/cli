// Package cli holds the pulseczar commands.
package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chrisggraham/pulseczar-cli/internal/api"
	"github.com/chrisggraham/pulseczar-cli/internal/config"
	"github.com/chrisggraham/pulseczar-cli/internal/render"
	"github.com/spf13/cobra"
)

// Environment variables that override the saved sign-in.
const (
	envToken = "PULSECZAR_TOKEN"
	envURL   = "PULSECZAR_URL"
)

// Env is everything the commands take from the outside, so tests can supply it.
type Env struct {
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	ConfigPath     string
	// IsTerminal is true when stdout is a terminal; it picks the table over the compact form.
	IsTerminal bool
	HTTP       *http.Client
	Sleep      func(time.Duration)
	Hostname   string
}

type app struct {
	env    Env
	asJSON bool
}

// Run executes the command line and returns the process exit code.
func Run(args []string, env Env) int {
	a := &app{env: env}

	root := a.newRoot()
	root.SetArgs(args)
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
		Short: "Watch and manage your PulseCzar monitors from the terminal",
		Long: `Watch and manage your PulseCzar monitors from the terminal.

Sign in once with "pulseczar auth login". Output is a table on a terminal, the
compact form when piped, and the API body unchanged with --json.

Environment:
  PULSECZAR_TOKEN  use this token instead of the saved sign-in
  PULSECZAR_URL    use another PulseCzar server (default ` + config.DefaultURL + `)

Exit codes: 0 ok, 1 error, 2 usage, 4 not signed in or not allowed.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().BoolVar(&a.asJSON, "json", false, "print the API body unchanged")

	root.AddCommand(a.newAuthCommand(), a.newStatusCommand(), a.newMonitorsCommand(), a.newIncidentsCommand())

	return root
}

// print writes a result in the form the caller asked for.
func (a *app) print(view render.View, raw []byte) {
	switch {
	case a.asJSON:
		a.env.Stdout.Write(raw)
		if !bytes.HasSuffix(raw, []byte("\n")) {
			io.WriteString(a.env.Stdout, "\n")
		}
	case a.env.IsTerminal:
		view.Table(a.env.Stdout)
	default:
		view.Compact(a.env.Stdout)
	}
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
