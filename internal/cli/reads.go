package cli

import (
	"fmt"
	"slices"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/spf13/cobra"
)

var monitorStatuses = []string{"up", "down", "stale", "paused", "new"}

func (a *app) newStatusCommand() *cobra.Command {
	var line bool

	status := &cobra.Command{
		Use:   "status",
		Short: "Show whether your monitors are up, with the counts",
		Long: `Show whether your monitors are up, with the counts.

With --line, print one line for a status bar and always exit 0. A signed-out or
offline state is a word on the line. The answer is cached for 30 seconds, so a bar
that runs the command often makes at most two calls a minute. Color follows NO_COLOR
and TERM=dumb, not the terminal check, because a bar runs the command in a pipe.`,
		Example: "  pulseczar status\n  pulseczar status --json\n  pulseczar status --line",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if line {
				return a.runStatusLine()
			}

			return a.showStatus()
		},
	}
	status.Flags().BoolVar(&line, "line", false, "print one line for a status bar")

	return status
}

func (a *app) runStatusLine() error {
	if a.asJSON {
		return usageError("--line prints text for a status bar, so it cannot be used with --json")
	}

	fmt.Fprintln(a.env.Stdout, a.statusLine())

	return nil
}

func (a *app) showStatus() error {
	client, err := a.signedInClient()
	if err != nil {
		return err
	}

	status, raw, err := client.GetStatus()
	if err != nil {
		return fail(err)
	}

	a.print(raw, statusView(status), func(t terminal) string { return t.statusScreen(status) })

	return nil
}

func (a *app) newMonitorsCommand() *cobra.Command {
	list := a.newMonitorsListCommand()

	monitors := &cobra.Command{
		Use:   "monitors",
		Short: "List, show and change monitors",
		Long:  "List, show and change monitors. Run alone, it lists them, as \"monitors list\" does.",
		Args:  cobra.NoArgs,
		RunE:  list.RunE,
	}
	monitors.Flags().AddFlagSet(list.Flags())

	monitors.AddCommand(
		list,
		a.newMonitorsShowCommand(),
		a.newMonitorsCreateCommand(),
		a.newMonitorsPauseCommand(),
		a.newMonitorsResumeCommand(),
		a.newMonitorsSnoozeCommand(),
		a.newMonitorsUnsnoozeCommand(),
	)

	return monitors
}

func (a *app) newMonitorsListCommand() *cobra.Command {
	var status string

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your monitors",
		Example: `  pulseczar monitors list
  pulseczar monitors list --status down
  pulseczar monitors list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if status != "" && !slices.Contains(monitorStatuses, status) {
				return usageError("--status must be one of up, down, stale, paused, new; got %q", status)
			}

			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			board, raw, err := client.GetBoard()
			if err != nil {
				return fail(err)
			}

			monitors := withStatus(board.Monitors, status)
			a.print(raw, monitorListView(monitors), func(t terminal) string { return t.monitorListScreen(monitors) })

			return nil
		},
	}
	list.Flags().StringVar(&status, "status", "", "only monitors that are up, down, stale, paused or new")

	return list
}

// withStatus keeps the monitors in one state. An empty status keeps them all.
func withStatus(monitors []api.MonitorSummary, status string) []api.MonitorSummary {
	if status == "" {
		return monitors
	}

	var kept []api.MonitorSummary
	for _, monitor := range monitors {
		if statusKey(monitor.Word) == status {
			kept = append(kept, monitor)
		}
	}

	return kept
}

func (a *app) newMonitorsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "show <monitor>",
		Short:             "Show one monitor",
		Long:              "Show one monitor. <monitor> is its id, its name, or the start of its name.",
		Example:           "  pulseczar monitors show checkout\n  pulseczar monitors show 7K3M9XQ2T4VB --json",
		Args:              monitorArgs(0),
		ValidArgsFunction: a.completeMonitorNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, id, err := a.signedInMonitor(args[0])
			if err != nil {
				return err
			}

			monitor, raw, err := client.GetMonitor(id)
			if err != nil {
				return fail(err)
			}

			a.print(raw, monitorView(monitor), func(t terminal) string { return t.monitorScreen(monitor, client.BaseURL) })

			return nil
		},
	}
}

func (a *app) newIncidentsCommand() *cobra.Command {
	list := a.newIncidentsListCommand()

	incidents := &cobra.Command{
		Use:   "incidents",
		Short: "List and show incidents",
		Long:  "List and show incidents. Run alone, it lists them, as \"incidents list\" does.",
		Args:  cobra.NoArgs,
		RunE:  list.RunE,
	}
	incidents.Flags().AddFlagSet(list.Flags())

	incidents.AddCommand(list, a.newIncidentsShowCommand())

	return incidents
}

func (a *app) newIncidentsListCommand() *cobra.Command {
	var filter api.IncidentFilter

	list := &cobra.Command{
		Use:   "list",
		Short: "List incidents across your monitors, newest first",
		Example: `  pulseczar incidents
  pulseczar incidents --open
  pulseczar incidents --since 24h
  pulseczar incidents --monitor checkout`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			if filter.MonitorID != "" {
				if filter.MonitorID, err = resolveMonitor(client, filter.MonitorID); err != nil {
					return err
				}
			}

			incidents, raw, err := client.ListIncidents(filter)
			if err != nil {
				return fail(err)
			}

			a.print(raw, incidentListView(incidents), func(t terminal) string { return t.incidentListScreen(incidents, filter) })

			return nil
		},
	}
	list.Flags().BoolVar(&filter.Open, "open", false, "only incidents that are still open")
	list.Flags().StringVar(&filter.Range, "since", "", "only this period: 24h, 7d, 30d, 90d, all or a count of seconds")
	list.Flags().StringVar(&filter.MonitorID, "monitor", "", "only this monitor's incidents: its name or id")
	list.Flags().StringVar(&filter.Cursor, "cursor", "", "the next_cursor of the previous page")
	_ = list.RegisterFlagCompletionFunc("monitor", a.completeMonitorFlag)

	return list
}

func (a *app) newIncidentsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Short:   "Show one incident",
		Example: "  pulseczar incidents show 4T9WQ8ZN2HJ5\n  pulseczar incidents show 4T9WQ8ZN2HJ5 --json",
		Args:    exactlyOneID("an incident id", "pulseczar incidents list"),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			incident, raw, err := client.GetIncident(args[0])
			if err != nil {
				return fail(err)
			}

			a.print(raw, incidentView(incident), func(t terminal) string { return t.incidentScreen(incident) })

			return nil
		},
	}
}
