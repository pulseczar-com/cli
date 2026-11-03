package cli

import (
	"fmt"
	"strings"

	"github.com/pulseczar-com/cli/internal/render"
	"github.com/spf13/cobra"
)

func (a *app) newOpenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "open [monitor]",
		Short: "Open the dashboard, or one monitor, in your browser",
		Long: `Open the PulseCzar dashboard in your browser, or the page of one monitor.

<monitor> is its id, its name, or the start of its name. The address is printed
too.`,
		Example:           "  pulseczar open\n  pulseczar open checkout",
		Args:              monitorArgsOptional,
		ValidArgsFunction: a.completeMonitorNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			address, err := a.webAddress(args)
			if err != nil {
				return err
			}

			return a.showAddress(address)
		},
	}
}

// monitorArgsOptional allows no argument, or one monitor.
func monitorArgsOptional(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return usageError("give one monitor name or id, not %d", len(args))
	}

	return nil
}

// webAddress is the dashboard, or the page of the monitor named in args.
func (a *app) webAddress(args []string) (string, error) {
	if len(args) == 0 {
		found, err := a.loadSettings()
		if err != nil {
			return "", fail(err)
		}

		return strings.TrimRight(found.URL, "/") + "/", nil
	}

	client, id, err := a.signedInMonitor(args[0])
	if err != nil {
		return "", err
	}

	return strings.TrimRight(client.BaseURL, "/") + "/monitors/" + id, nil
}

// showAddress opens the address and prints it. If no browser starts, the error
// carries the address.
func (a *app) showAddress(address string) error {
	if err := a.env.OpenBrowser(address); err != nil {
		return &commandError{
			exit:    exitError,
			code:    codeLocalFailure,
			message: "could not open a browser: " + err.Error(),
			details: []string{"hint: open " + address + " yourself"},
		}
	}

	opened := a.styleFor(a.env.IsTerminal).Paint(render.Green, render.MarkDone)
	fmt.Fprintf(a.env.Stdout, "%s Opened %s\n", opened, address)

	return nil
}
