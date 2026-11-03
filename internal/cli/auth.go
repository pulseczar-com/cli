package cli

import (
	"errors"
	"fmt"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/config"
	"github.com/pulseczar-com/cli/internal/render"
	"github.com/spf13/cobra"
)

func (a *app) newAuthCommand() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Sign in, sign out and show who is signed in",
	}

	auth.AddCommand(
		&cobra.Command{
			Use:   "login",
			Short: "Sign in by approving a code in your browser",
			Long: `Sign in by approving a code in your browser.

The command shows a code, then opens PulseCzar in your browser when you press
Enter. Sign in if asked, type the code, choose Read only or Read and write, and
approve. Over SSH, or when input is not a terminal, the command prints the
address and the code and opens nothing. The command waits, then saves the token to ~/.config/pulseczar/config.json.`,
			Example: `  pulseczar auth login`,
			Args:    cobra.NoArgs,
			RunE:    a.runLogin,
		},
		&cobra.Command{
			Use:     "status",
			Short:   "Show the account this CLI is signed in to",
			Example: "  pulseczar auth status\n  pulseczar auth status --json",
			Args:    cobra.NoArgs,
			RunE:    a.runAuthStatus,
		},
		&cobra.Command{
			Use:   "logout",
			Short: "Revoke the saved token and delete it from this computer",
			Long: `Revoke the saved token on the server and delete it from this computer.

A token set in PULSECZAR_TOKEN is not touched.`,
			Example: `  pulseczar auth logout`,
			Args:    cobra.NoArgs,
			RunE:    a.runLogout,
		},
		&cobra.Command{
			Use:   "token",
			Short: "Print the saved token, for CI",
			Long: `Print the saved token and nothing else, so a script can capture it.

Store the output as the CI secret PULSECZAR_TOKEN. The command exits 4 when no
token is saved.`,
			Example: `  PULSECZAR_TOKEN=$(pulseczar auth token)`,
			Args:    cobra.NoArgs,
			RunE:    a.runAuthToken,
		},
	)

	return auth
}

func (a *app) runAuthStatus(cmd *cobra.Command, _ []string) error {
	found, err := a.loadSettings()
	if err != nil {
		return fail(err)
	}

	client, err := a.clientFor(found)
	if err != nil {
		return err
	}

	me, raw, err := client.GetMe()
	if err != nil {
		return fail(err)
	}

	a.print(raw, meView(me, found.URL, found.TokenSource), func(t terminal) string {
		return t.meScreen(me, found.URL, found.TokenSource)
	})

	return nil
}

// runLogout revokes the saved token, then deletes the file. The file goes even when
// the server cannot be reached, because the person asked to sign out here.
func (a *app) runLogout(cmd *cobra.Command, _ []string) error {
	file, err := config.Load(a.env.ConfigPath)
	if err != nil {
		return fail(err)
	}

	if file.Token == "" {
		return notSignedIn()
	}

	url := file.URL
	if url == "" {
		url = config.DefaultURL
	}

	revokeErr := (&api.Client{BaseURL: url, Token: file.Token, HTTP: a.env.HTTP}).Logout()

	if err := config.Remove(a.env.ConfigPath); err != nil {
		return fail(err)
	}

	a.clearStatusCaches()

	if isRejectedToken(revokeErr) {
		revokeErr = nil // The server no longer accepts the token, so there is nothing to revoke.
	}
	if revokeErr != nil {
		failure := fail(revokeErr)
		failure.message = "signed out on this computer, but the server did not revoke the token: " + failure.message

		return failure
	}

	message := "Logged out."
	if a.env.IsTerminal {
		message = a.stdoutTerminal().style.Paint(render.Green, render.MarkDone) + " " + message
	}
	fmt.Fprintln(a.env.Stdout, message)

	return nil
}

func (a *app) runAuthToken(cmd *cobra.Command, _ []string) error {
	file, err := config.Load(a.env.ConfigPath)
	if err != nil {
		return fail(err)
	}

	if file.Token == "" {
		return notSignedIn()
	}

	fmt.Fprintln(a.env.Stdout, file.Token)

	return nil
}

func isRejectedToken(err error) bool {
	var apiErr *api.Error

	return errors.As(err, &apiErr) && apiErr.Code == api.CodeUnauthenticated
}
