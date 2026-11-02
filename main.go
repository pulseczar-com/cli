// Command pulseczar watches and manages PulseCzar monitors from the terminal.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/chrisggraham/pulseczar-cli/internal/cli"
	"github.com/chrisggraham/pulseczar-cli/internal/config"
)

const requestTimeout = 30 * time.Second

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "pulseczar-cli"
	}

	stdout, _ := os.Stdout.Stat()

	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Getenv:     os.Getenv,
		ConfigPath: config.DefaultPath(home),
		IsTerminal: stdout != nil && stdout.Mode()&os.ModeCharDevice != 0,
		HTTP:       &http.Client{Timeout: requestTimeout},
		Sleep:      time.Sleep,
		Hostname:   hostname,
	}))
}
