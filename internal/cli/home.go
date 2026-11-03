package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/pulseczar-com/cli/internal/api"
	"github.com/pulseczar-com/cli/internal/render"
)

// minLogoWidth is the narrowest terminal that shows the logo.
const minLogoWidth = 50

// maxHomeMonitors is how many monitors the home screen lists.
const maxHomeMonitors = 10

// The logo: four bars, then the name. The name is split where Pulse ends and Czar starts.
const logoBars = "░ ▒ ▓ █"

var logoRows = [3][2]string{
	{"╔═╗┬ ┬┬  ┌─┐┌─┐", "╔═╗┌─┐┌─┐┬─┐"},
	{"╠═╝│ ││  └─┐├┤ ", "║  ┌─┘├─┤├┬┘"},
	{"╩  └─┘┴─┘└─┘└─┘", "╚═╝└─┘┴ ┴┴└─"},
}

// runHome is the bare `pulseczar`. Piped or with --json it is `pulseczar status`.
func (a *app) runHome() error {
	if a.asJSON || !a.env.IsTerminal {
		return a.showStatus()
	}

	found, err := a.loadSettings()
	if err != nil {
		return fail(err)
	}

	if found.Token == "" {
		io.WriteString(a.env.Stdout, a.stdoutTerminal().welcomeScreen(a.logo()))

		return nil
	}

	client, err := a.clientFor(found)
	if err != nil {
		return err
	}

	me, board, err := fetchHome(client)
	if err != nil {
		return fail(err)
	}

	io.WriteString(a.env.Stdout, a.stdoutTerminal().homeScreen(a.logo(), me, board))

	return nil
}

// fetchHome asks for the account and the board at the same time.
func fetchHome(client *api.Client) (api.Me, api.Board, error) {
	type meAnswer struct {
		me  api.Me
		err error
	}

	meDone := make(chan meAnswer, 1)
	go func() {
		me, _, err := client.GetMe()
		meDone <- meAnswer{me, err}
	}()

	board, _, boardErr := client.GetBoard()
	account := <-meDone

	if account.err != nil {
		return api.Me{}, api.Board{}, account.err
	}

	return account.me, board, boardErr
}

// logo is empty when the terminal is too narrow for it.
func (a *app) logo() string {
	if a.env.Width < minLogoWidth {
		return ""
	}

	return a.stdoutTerminal().logo(a.env.Version)
}

func (t terminal) logo(version string) string {
	var out strings.Builder

	for i, row := range logoRows {
		fmt.Fprintf(&out, " %s  %s%s", t.style.Paint(render.Green, logoBars), row[0], t.style.Paint(render.Green, row[1]))
		if i == len(logoRows)-1 {
			fmt.Fprintf(&out, "  %s", t.gray(versionLabel(version)))
		}
		out.WriteString("\n")
	}

	return out.String()
}

// versionLabel writes a number as "v0.4.0" and leaves a word such as "dev" alone.
func versionLabel(version string) string {
	if version != "" && unicode.IsDigit(rune(version[0])) {
		return "v" + version
	}

	return version
}

func (t terminal) welcomeScreen(logo string) string {
	var out strings.Builder

	if logo != "" {
		out.WriteString(logo + "\n")
	}

	out.WriteString(" Uptime monitoring for your sites and jobs, in your terminal.\n\n")
	out.WriteString(" You are not signed in.\n\n")
	out.WriteString(t.nextLines([]nextStep{
		{"pulseczar auth login", "sign in with your browser"},
		{"pulseczar --help", "all commands"},
	}))
	fmt.Fprintf(&out, "\n In CI, set PULSECZAR_TOKEN.  Guide: %s\n", guideURL)

	return out.String()
}

func (t terminal) homeScreen(logo string, me api.Me, board api.Board) string {
	var out strings.Builder

	if logo != "" {
		out.WriteString(logo + "\n")
	}

	account := fmt.Sprintf("%s · %s · %d of %d", me.Email, me.PlanWord, me.MonitorCount, me.MonitorLimit)
	fmt.Fprintf(&out, " %s %s     %s\n\n", t.style.Paint(statusColor(board.Counts), "●"), board.Summary.Text, t.gray(account))

	monitors := problemsFirst(board.Monitors)
	if len(monitors) == 0 {
		out.WriteString("   No monitors yet.\n\n")
	} else {
		out.WriteString(t.homeMonitors(monitors))
		out.WriteString("\n")
	}

	out.WriteString(" Next\n" + t.nextLines(nextSteps(monitors)))

	return out.String()
}

// problemsFirst puts down, stale and new monitors ahead of the rest. The order inside each group stays.
func problemsFirst(monitors []api.MonitorSummary) []api.MonitorSummary {
	sorted := slices.Clone(monitors)
	slices.SortStableFunc(sorted, func(a, b api.MonitorSummary) int {
		return problemRank(a.Word) - problemRank(b.Word)
	})

	return sorted
}

func problemRank(word string) int {
	switch statusKey(word) {
	case "down":
		return 0
	case "stale":
		return 1
	case "new":
		return 2
	case "paused":
		return 4
	default:
		return 3
	}
}

func (t terminal) homeMonitors(monitors []api.MonitorSummary) string {
	shown := monitors[:min(len(monitors), maxHomeMonitors)]
	rows := make([][]render.Cell, len(shown))

	for i, monitor := range shown {
		rows[i] = []render.Cell{
			{Text: monitor.Name},
			t.wordCell(monitor.Word),
			{Text: monitor.FootLeft, Color: render.Gray},
			{Text: uptimeText(monitor.Uptime24)},
		}
	}

	var out strings.Builder
	for i, line := range t.style.Align(rows) {
		fmt.Fprintf(&out, "   %s %s\n", t.dot(shown[i].Word), line)
	}

	if more := len(monitors) - len(shown); more > 0 {
		fmt.Fprintf(&out, "   %s\n", t.gray(fmt.Sprintf("and %d more · pulseczar monitors list", more)))
	}

	return out.String()
}

// nextStep is a command to run next and why.
type nextStep struct {
	command, reason string
}

// nextSteps follows the state: a problem monitor first, then the common jobs.
func nextSteps(monitors []api.MonitorSummary) []nextStep {
	if len(monitors) == 0 {
		return []nextStep{
			{"pulseczar monitors create https://example.com", "add a monitor"},
			{"pulseczar --help", "all commands"},
		}
	}

	var steps []nextStep

	first := monitors[0]
	if problemRank(first.Word) <= problemRank("stale") {
		steps = append(steps, nextStep{"pulseczar monitors show " + shellWord(first.Name), "see why it is " + statusKey(first.Word)})
	}

	return append(steps,
		nextStep{"pulseczar incidents --open", "open incidents"},
		nextStep{"pulseczar monitors create <url>", "add a monitor"},
		nextStep{"pulseczar --help", "all commands"},
	)
}

func (t terminal) nextLines(steps []nextStep) string {
	rows := make([][]render.Cell, len(steps))
	for i, step := range steps {
		rows[i] = []render.Cell{{Text: step.command}, {Text: step.reason, Color: render.Gray}}
	}

	return t.indented(rows)
}
