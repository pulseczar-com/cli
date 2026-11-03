package cli

import (
	"github.com/pulseczar-com/cli/internal/api"
	"github.com/spf13/cobra"
)

const (
	defaultHTTPEvery  = "5m"
	httpTimeoutSecond = 10 // The server's longest allowed timeout; the CLI has no flag for it.
)

// defaultSnooze is the length used when the person gives none.
const defaultSnooze = "1h"

// snoozeLengths are the words a person may give, in the order the help lists them.
var snoozeLengths = []string{"1h", "24h", "next", "forever"}

// snoozeModes maps the snooze lengths to the modes Czar's mute route takes.
var snoozeModes = map[string]string{
	"1h":      "1h",
	"24h":     "24h",
	"next":    "next_incident",
	"forever": "forever",
}

// monitorCall is one write to a monitor that answers with the changed monitor.
type monitorCall func(client *api.Client, id string) (api.MonitorDetail, []byte, error)

func (a *app) newMonitorsCreateCommand() *cobra.Command {
	var flags createFlags

	create := &cobra.Command{
		Use:   "create [url]",
		Short: "Create a monitor that checks an address, or one that waits for a heartbeat",
		Long: `Create a monitor.

With an address, PulseCzar checks it from several locations every --every
(default ` + defaultHTTPEvery + `). With --heartbeat, PulseCzar waits for your job to ping a
secret address and raises an incident when a ping is late by more than --grace.

A length is a number with s, m, h or d: 90s, 5m, 2h, 1d.`,
		Example: `  pulseczar monitors create https://example.com
  pulseczar monitors create https://example.com --name "Marketing site" --every 1m
  pulseczar monitors create --heartbeat --name "Nightly backup" --every 1d --grace 1h`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usageError("give one address to check, not %d", len(args))
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := buildCreateBody(args, flags)
			if err != nil {
				return err
			}

			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			monitor, raw, err := client.CreateMonitor(body)
			if err != nil {
				return fail(err)
			}

			a.print(raw, monitorView(monitor), func(t terminal) string { return t.createdScreen(monitor) })

			return nil
		},
	}
	create.Flags().StringVar(&flags.name, "name", "", "the monitor's name (default: the address's host)")
	create.Flags().StringVar(&flags.every, "every", "", "how often to check, or how often the job pings")
	create.Flags().StringVar(&flags.grace, "grace", "", "heartbeat only: how late a ping may be")
	create.Flags().BoolVar(&flags.heartbeat, "heartbeat", false, "create a heartbeat monitor instead of checking an address")

	return create
}

// createFlags are the flags of `monitors create`.
type createFlags struct {
	name      string
	every     string
	grace     string
	heartbeat bool
}

// buildCreateBody picks the http or the heartbeat body from the flags.
func buildCreateBody(args []string, flags createFlags) (api.MonitorCreate, error) {
	if flags.heartbeat {
		return buildHeartbeatBody(args, flags)
	}

	return buildHTTPBody(args, flags)
}

func buildHTTPBody(args []string, flags createFlags) (api.MonitorCreate, error) {
	if len(args) != 1 {
		return api.MonitorCreate{}, usageError("give the address to check, or use --heartbeat")
	}
	if flags.grace != "" {
		return api.MonitorCreate{}, usageError("--grace is for heartbeat monitors only")
	}
	every := flags.every
	if every == "" {
		every = defaultHTTPEvery
	}

	seconds, err := parseSeconds(every)
	if err != nil {
		return api.MonitorCreate{}, usageError("--every: %s", err)
	}

	return api.MonitorCreate{
		Type:            "http",
		Name:            flags.name,
		URL:             args[0],
		IntervalSeconds: seconds,
		TimeoutSeconds:  httpTimeoutSecond,
	}, nil
}

func buildHeartbeatBody(args []string, flags createFlags) (api.MonitorCreate, error) {
	if len(args) != 0 {
		return api.MonitorCreate{}, usageError("a heartbeat monitor has no address to check")
	}
	if flags.name == "" || flags.every == "" {
		return api.MonitorCreate{}, usageError("--heartbeat needs --name and --every")
	}

	seconds, err := parseSeconds(flags.every)
	if err != nil {
		return api.MonitorCreate{}, usageError("--every: %s", err)
	}

	body := api.MonitorCreate{Type: "heartbeat", Name: flags.name, IntervalSeconds: seconds}

	if flags.grace != "" {
		graceSeconds, err := parseSeconds(flags.grace)
		if err != nil {
			return api.MonitorCreate{}, usageError("--grace: %s", err)
		}
		body.GraceSeconds = &graceSeconds
	}

	return body, nil
}

func (a *app) newMonitorsPauseCommand() *cobra.Command {
	return a.newMonitorIDCommand(
		"pause <monitor>",
		"Pause a monitor: it stops checking and stops alerting",
		"  pulseczar monitors pause checkout",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetPaused(id, true)
		},
	)
}

func (a *app) newMonitorsResumeCommand() *cobra.Command {
	return a.newMonitorIDCommand(
		"resume <monitor>",
		"Resume a paused monitor",
		"  pulseczar monitors resume checkout",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetPaused(id, false)
		},
	)
}

func (a *app) newMonitorsUnsnoozeCommand() *cobra.Command {
	return a.newMonitorIDCommand(
		"unsnooze <monitor>",
		"Turn alerts back on for a snoozed monitor",
		"  pulseczar monitors unsnooze checkout",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetSnooze(id, "off")
		},
	)
}

// newMonitorIDCommand builds a command that takes one monitor, makes one write
// and prints the monitor it answers with.
func (a *app) newMonitorIDCommand(use, short, example string, write monitorCall) *cobra.Command {
	return &cobra.Command{
		Use:               use,
		Short:             short,
		Example:           example,
		Args:              monitorArgs(0),
		ValidArgsFunction: a.completeMonitorNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, id, err := a.signedInMonitor(args[0])
			if err != nil {
				return err
			}

			monitor, raw, err := write(client, id)
			if err != nil {
				return fail(err)
			}

			a.print(raw, monitorView(monitor), func(t terminal) string { return t.monitorScreen(monitor, client.BaseURL) })

			return nil
		},
	}
}

func (a *app) newMonitorsSnoozeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "snooze <monitor> [1h|24h|next|forever]",
		Short: "Snooze alerts for a monitor; it keeps checking",
		Long: `Snooze alerts for a monitor. The monitor keeps checking and keeps its history.

  1h, 24h   snooze for that long (1h when you give no length)
  next      snooze until the next incident ends
  forever   snooze until you run "monitors unsnooze"`,
		Example: `  pulseczar monitors snooze checkout
  pulseczar monitors snooze checkout 24h
  pulseczar monitors snooze checkout next`,
		Args:              monitorArgs(1),
		ValidArgsFunction: a.completeSnooze,
		RunE: func(cmd *cobra.Command, args []string) error {
			length := defaultSnooze
			if len(args) > 1 {
				length = args[1]
			}

			mode, ok := snoozeModes[length]
			if !ok {
				return usageError("the snooze length must be one of 1h, 24h, next, forever; got %q", length)
			}

			client, id, err := a.signedInMonitor(args[0])
			if err != nil {
				return err
			}

			monitor, raw, err := client.SetSnooze(id, mode)
			if err != nil {
				return fail(err)
			}

			a.print(raw, monitorView(monitor), func(t terminal) string { return t.monitorScreen(monitor, client.BaseURL) })

			return nil
		},
	}
}
