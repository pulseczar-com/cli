package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/spf13/cobra"
)

// completionTimeout keeps a tab press from hanging when the server is slow.
const completionTimeout = 2 * time.Second

const listMonitorsHint = "hint: run `pulseczar monitors list` to see your monitors"

// monitorIDPattern is the 12-character id of a monitor, as the API writes it.
var monitorIDPattern = regexp.MustCompile(`^[0-9A-Z]{12}$`)

// signedInMonitor returns a client and the id of the monitor a person named. A person
// names a monitor by its id, its name, or the start of its name, in any case.
func (a *app) signedInMonitor(given string) (*api.Client, string, error) {
	client, err := a.signedInClient()
	if err != nil {
		return nil, "", err
	}

	id, err := resolveMonitor(client, given)

	return client, id, err
}

// resolveMonitor passes an id through and looks any other word up on the board.
func resolveMonitor(client *api.Client, given string) (string, error) {
	if monitorIDPattern.MatchString(given) {
		return given, nil
	}

	board, _, err := client.GetBoard()
	if err != nil {
		return "", fail(err)
	}

	return pickMonitor(board.Monitors, given)
}

// pickMonitor finds one monitor: an exact name or id first, then the start of a name.
// Two matches at the same step is an error that names them.
func pickMonitor(monitors []api.MonitorSummary, given string) (string, error) {
	exact := monitorsMatching(monitors, func(monitor api.MonitorSummary) bool {
		return strings.EqualFold(monitor.Name, given) || strings.EqualFold(monitor.ID, given)
	})
	if len(exact) > 0 {
		return singleMonitor(exact, given)
	}

	prefix := strings.ToLower(given)
	starts := monitorsMatching(monitors, func(monitor api.MonitorSummary) bool {
		return strings.HasPrefix(strings.ToLower(monitor.Name), prefix)
	})
	if len(starts) > 0 {
		return singleMonitor(starts, given)
	}

	return "", &commandError{
		exit:    exitError,
		code:    api.CodeNotFound,
		message: fmt.Sprintf("no monitor named %q", given),
		details: []string{listMonitorsHint},
	}
}

func monitorsMatching(monitors []api.MonitorSummary, matches func(api.MonitorSummary) bool) []api.MonitorSummary {
	var found []api.MonitorSummary
	for _, monitor := range monitors {
		if matches(monitor) {
			found = append(found, monitor)
		}
	}

	return found
}

func singleMonitor(matches []api.MonitorSummary, given string) (string, error) {
	if len(matches) == 1 {
		return matches[0].ID, nil
	}

	details := make([]string, 0, len(matches)+1)
	for _, monitor := range matches {
		details = append(details, fmt.Sprintf("%s  %s", monitor.Name, monitor.ID))
	}
	details = append(details, "hint: give more of the name, or the id")

	return "", &commandError{
		exit:    exitUsage,
		code:    codeUsage,
		message: fmt.Sprintf("%q matches %d monitors", given, len(matches)),
		details: details,
	}
}

// monitorArgs checks the arguments of a command that takes a monitor, plus up to
// extra more, with plain words in place of cobra's "accepts 1 arg(s)".
func monitorArgs(extra int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		switch {
		case len(args) == 0:
			return usageError("give a monitor name or id; run `pulseczar monitors list` to see them")
		case len(args) > 1+extra:
			return usageError("too many arguments: got %d, want at most %d", len(args), 1+extra)
		default:
			return nil
		}
	}
}

// completeMonitorNames lists the names on the board for the first argument. It
// stays silent on any failure: a tab press must never print an error.
func (a *app) completeMonitorNames(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return a.boardNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeMonitorFlag is the same list for a flag that takes a monitor.
func (a *app) completeMonitorFlag(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return a.boardNames(), cobra.ShellCompDirectiveNoFileComp
}

// completeSnooze offers the monitor names, then the snooze lengths.
func (a *app) completeSnooze(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 1 {
		return snoozeLengths, cobra.ShellCompDirectiveNoFileComp
	}

	return a.completeMonitorNames(cmd, args, toComplete)
}

// boardNames reads the monitor names from /board, each with its id as the description.
func (a *app) boardNames() []string {
	client, err := a.signedInClient()
	if err != nil {
		return nil
	}

	quick := *client
	quickHTTP := *a.env.HTTP
	quickHTTP.Timeout = completionTimeout
	quick.HTTP = &quickHTTP

	board, _, err := quick.GetBoard()
	if err != nil {
		return nil
	}

	names := make([]string, len(board.Monitors))
	for i, monitor := range board.Monitors {
		names[i] = monitor.Name + "\t" + monitor.ID
	}

	return names
}
