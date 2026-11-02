package cli

import (
	"github.com/pulseczar-com/cli/internal/api"
	"github.com/spf13/cobra"
)

const (
	defaultHTTPEvery  = "5m"
	httpTimeoutSecond = 10 // The server's longest allowed timeout; the CLI has no flag for it.
)

// snoozeModes maps the --for words to the modes Czar's mute route takes.
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
		Args: cobra.MaximumNArgs(1),
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

			a.print(monitorView(monitor), raw)

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
		"pause <id>",
		"Pause a monitor: it stops checking and stops alerting",
		"  pulseczar monitors pause 7K3M9XQ2T4VB",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetPaused(id, true)
		},
	)
}

func (a *app) newMonitorsResumeCommand() *cobra.Command {
	return a.newMonitorIDCommand(
		"resume <id>",
		"Resume a paused monitor",
		"  pulseczar monitors resume 7K3M9XQ2T4VB",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetPaused(id, false)
		},
	)
}

func (a *app) newMonitorsUnsnoozeCommand() *cobra.Command {
	return a.newMonitorIDCommand(
		"unsnooze <id>",
		"Turn alerts back on for a snoozed monitor",
		"  pulseczar monitors unsnooze 7K3M9XQ2T4VB",
		func(client *api.Client, id string) (api.MonitorDetail, []byte, error) {
			return client.SetSnooze(id, "off")
		},
	)
}

// newMonitorIDCommand builds a command that takes one monitor id, makes one write
// and prints the monitor it answers with.
func (a *app) newMonitorIDCommand(use, short, example string, write monitorCall) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: example,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			monitor, raw, err := write(client, args[0])
			if err != nil {
				return fail(err)
			}

			a.print(monitorView(monitor), raw)

			return nil
		},
	}
}

func (a *app) newMonitorsSnoozeCommand() *cobra.Command {
	var length string

	snooze := &cobra.Command{
		Use:   "snooze <id> --for 1h|24h|next|forever",
		Short: "Snooze alerts for a monitor; it keeps checking",
		Long: `Snooze alerts for a monitor. The monitor keeps checking and keeps its history.

  1h, 24h   snooze for that long
  next      snooze until the next incident ends
  forever   snooze until you run "monitors unsnooze"`,
		Example: `  pulseczar monitors snooze 7K3M9XQ2T4VB --for 1h
  pulseczar monitors snooze 7K3M9XQ2T4VB --for next`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, ok := snoozeModes[length]
			if !ok {
				return usageError("--for must be one of 1h, 24h, next, forever; got %q", length)
			}

			client, err := a.signedInClient()
			if err != nil {
				return err
			}

			monitor, raw, err := client.SetSnooze(args[0], mode)
			if err != nil {
				return fail(err)
			}

			a.print(monitorView(monitor), raw)

			return nil
		},
	}
	snooze.Flags().StringVar(&length, "for", "", "how long: 1h, 24h, next or forever")
	_ = snooze.MarkFlagRequired("for")

	return snooze
}
