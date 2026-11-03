// Command pulseczar checks and manages PulseCzar monitors from the terminal.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/pulseczar-com/cli/internal/cli"
	"github.com/pulseczar-com/cli/internal/config"
	"golang.org/x/term"
)

const requestTimeout = 30 * time.Second

// version is set when the program is linked: -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "pulseczar-cli"
	}

	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Getenv:           os.Getenv,
		ConfigPath:       config.DefaultPath(home),
		IsTerminal:       term.IsTerminal(int(os.Stdout.Fd())),
		StderrIsTerminal: term.IsTerminal(int(os.Stderr.Fd())),
		HTTP:             &http.Client{Timeout: requestTimeout},
		Sleep:            time.Sleep,
		Now:              time.Now,
		Hostname:         hostname,
		Version:          version,
	}))
}
