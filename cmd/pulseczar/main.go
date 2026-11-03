// Command pulseczar checks and manages PulseCzar monitors from the terminal.
package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cli/browser"
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

	// Without a cache folder the status line still works; it just calls every time.
	cacheDir, err := os.UserCacheDir()
	if err == nil {
		cacheDir = filepath.Join(cacheDir, "pulseczar")
	} else {
		cacheDir = ""
	}

	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "pulseczar-cli"
	}

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 0
	}

	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Getenv:           os.Getenv,
		ConfigPath:       config.DefaultPath(home),
		CacheDir:         cacheDir,
		IsTerminal:       term.IsTerminal(int(os.Stdout.Fd())),
		StderrIsTerminal: term.IsTerminal(int(os.Stderr.Fd())),
		StdinIsTerminal:  term.IsTerminal(int(os.Stdin.Fd())),
		HTTP:             &http.Client{Timeout: requestTimeout},
		Sleep:            time.Sleep,
		Now:              time.Now,
		Hostname:         hostname,
		OpenBrowser:      browser.OpenURL,
		Version:          version,
		Width:            width,
	}))
}
