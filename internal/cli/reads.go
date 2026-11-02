package cli

import (
	"slices"

	"github.com/chrisggraham/pulseczar-cli/internal/api"
	"github.com/spf13/cobra"
)

var monitorStatuses = []string{"up", "down", "stale", "paused"}

func (a *app) newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show whether your monitors are up, with the counts",
		Example: "  pulseczar status\n  pulseczar status --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			status, raw, err := client.GetStatus()
			if err != nil {
				return fail(err)
			}

			a.print(statusView(status), raw)

			return nil
		},
	}
}

func (a *app) newMonitorsCommand() *cobra.Command {
	monitors := &cobra.Command{
		Use:   "monitors",
		Short: "List, show and change monitors",
	}

	monitors.AddCommand(
		a.newMonitorsListCommand(),
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
		Use:   "list",
		Short: "List your monitors",
		Example: `  pulseczar monitors list
  pulseczar monitors list --status down
  pulseczar monitors list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if status != "" && !slices.Contains(monitorStatuses, status) {
				return usageError("--status must be one of up, down, stale, paused; got %q", status)
			}

			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			board, raw, err := client.GetBoard()
			if err != nil {
				return fail(err)
			}

			a.print(monitorListView(withStatus(board.Monitors, status)), raw)

			return nil
		},
	}
	list.Flags().StringVar(&status, "status", "", "only monitors that are up, down, stale or paused")

	return list
}

// withStatus keeps the monitors in one state. An empty status keeps them all.
func withStatus(monitors []api.MonitorSummary, status string) []api.MonitorSummary {
	if status == "" {
		return monitors
	}

	var kept []api.MonitorSummary
	for _, monitor := range monitors {
		if monitor.Status == status {
			kept = append(kept, monitor)
		}
	}

	return kept
}

func (a *app) newMonitorsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Short:   "Show one monitor",
		Example: "  pulseczar monitors show 7K3M9XQ2T4VB\n  pulseczar monitors show 7K3M9XQ2T4VB --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			monitor, raw, err := client.GetMonitor(args[0])
			if err != nil {
				return fail(err)
			}

			a.print(monitorView(monitor), raw)

			return nil
		},
	}
}

func (a *app) newIncidentsCommand() *cobra.Command {
	incidents := &cobra.Command{
		Use:   "incidents",
		Short: "List and show incidents",
	}

	incidents.AddCommand(a.newIncidentsListCommand(), a.newIncidentsShowCommand())

	return incidents
}

func (a *app) newIncidentsListCommand() *cobra.Command {
	var filter api.IncidentFilter

	list := &cobra.Command{
		Use:   "list",
		Short: "List incidents across your monitors, newest first",
		Example: `  pulseczar incidents list
  pulseczar incidents list --open
  pulseczar incidents list --since 24h
  pulseczar incidents list --monitor 7K3M9XQ2T4VB`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			incidents, raw, err := client.ListIncidents(filter)
			if err != nil {
				return fail(err)
			}

			a.print(incidentListView(incidents), raw)

			return nil
		},
	}
	list.Flags().BoolVar(&filter.Open, "open", false, "only incidents that are still open")
	list.Flags().StringVar(&filter.Range, "since", "", "only this period: 24h, 7d, 30d, 90d, all or a count of seconds")
	list.Flags().StringVar(&filter.MonitorID, "monitor", "", "only this monitor's incidents")
	list.Flags().StringVar(&filter.Cursor, "cursor", "", "the next_cursor of the previous page")

	return list
}

func (a *app) newIncidentsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "show <id>",
		Short:   "Show one incident",
		Example: "  pulseczar incidents show 4T9WQ8ZN2HJ5\n  pulseczar incidents show 4T9WQ8ZN2HJ5 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			incident, raw, err := client.GetIncident(args[0])
			if err != nil {
				return fail(err)
			}

			a.print(incidentView(incident), raw)

			return nil
		},
	}
}
