# pulseczar

`pulseczar` shows and manages your [PulseCzar](https://pulseczar.com) monitors from the terminal. It is a single binary with no runtime.

## Install

Pick one. Each installs the same program.

**Homebrew** (macOS and Linux):

```sh
brew install pulseczar-com/tap/pulseczar
```

This also installs completions for bash, zsh and fish.

**Install script** (Linux and macOS). It checks the checksum and needs no sudo. It installs to `~/.local/bin`; set `PULSECZAR_BIN_DIR` to use another folder, or `PULSECZAR_VERSION` for a version other than the latest.

```sh
curl -fsSL https://pulseczar.com/install.sh | sh
```

**Download**: get the archive for your system from the [releases page](https://github.com/pulseczar-com/cli/releases) and put `pulseczar` on your `PATH`. Windows gets a zip.

**Go** (1.24 or newer):

```sh
go install github.com/pulseczar-com/cli/cmd/pulseczar@latest
```

The program is not signed on macOS yet. Homebrew removes the quarantine flag for you. After a manual download, run `xattr -d com.apple.quarantine pulseczar` if macOS blocks it.

### Completion

Homebrew sets up completion. For other installs, run `pulseczar completion --help` and follow the steps for your shell.

## Sign in

```sh
pulseczar auth login
```

The command prints an address and a code:

```
Open: https://pulseczar.com/cli
Code: BCDF-GHJK
```

Open the address, sign in to PulseCzar if asked, type the code, choose **Read only** or **Read and write**, and approve. The command then saves a token to `~/.config/pulseczar/config.json` (mode 0600). No token is created or shown on the web.

| | |
| --- | --- |
| `pulseczar auth status` | Show the account you are signed in to. |
| `pulseczar auth token` | Print the saved token and nothing else. |
| `pulseczar auth logout` | Revoke the token on the server and delete it from this computer. |
| `PULSECZAR_TOKEN` | Use this token instead of the saved one (for CI and scripts). |
| `PULSECZAR_URL` | Use another PulseCzar server, such as a local one. |

## Use in CI

1. Sign in once on your computer with `pulseczar auth login`. Choose **Read only** unless the job writes.
2. Run `pulseczar auth token` and store the output as the CI secret `PULSECZAR_TOKEN`.

A token expires after 90 days without use.

## Commands

| Command | What it does |
| --- | --- |
| `status` | Whether your monitors are up, with the counts. `status --line` prints one line for a status bar. |
| `monitors [--status up\|down\|stale\|paused\|new]` | List your monitors (`monitors list` or `monitors ls` do the same). |
| `monitors show <monitor>` | One monitor, with its recent incidents. |
| `incidents [--open] [--since 24h] [--monitor <monitor>]` | Incidents, newest first, 25 to a page. On a terminal the last line is the command for the next page (`--cursor`). |
| `incidents show <id>` | One incident and the locations that saw it. |
| `monitors create <url> [--name <name>] [--every 5m]` | Check an address. |
| `monitors create --heartbeat --name <name> --every 1d [--grace 1h]` | Wait for your job to ping. |
| `monitors pause <monitor>` / `monitors resume <monitor>` | Stop or restart a monitor. |
| `monitors snooze <monitor> [1h\|24h\|next\|forever]` | Snooze alerts (1h if you give no length); the monitor keeps checking. |
| `monitors unsnooze <monitor>` | Turn alerts back on. |
| `open [monitor]` | Open the dashboard, or one monitor's page, in your browser. |

`<monitor>` is a monitor's id, its name, or the start of its name, in any case. Two monitors that match are an error that lists both.

Writes need a **Read and write** sign-in. `pulseczar <command> --help` shows flags and examples.

## Status line

`pulseczar status --line` prints one line for a status bar, such as `PulseCzar ● 1 down · 1 stale`. The text is the status the web header shows. It always exits 0, so a bar shows `PulseCzar ? offline` or `PulseCzar ○ signed out` and not an empty entry. `--line` cannot be used with `--json`.

- The answer is cached for 30 seconds in `os.UserCacheDir()/pulseczar/`, so one computer makes at most two calls a minute however many bars run the command. When the call fails, a cache under an hour old shows with its age (`· 6m ago`).
- The call has a 2-second timeout. A refused token shows `PulseCzar ○ sign in again`.
- Color is on unless `NO_COLOR` is set or `TERM=dumb`, even when piped, because a bar runs the command in a pipe.
- `auth login` and `auth logout` delete the cache.

Claude Code, in `~/.claude/settings.json` (or append the output to an existing status line script):

```json
{ "statusLine": { "type": "command", "command": "pulseczar status --line", "refreshInterval": 30 } }
```

herdr, in `~/.config/herdr/config.toml`:

```toml
[ui]
tab_bar_right = [ { type = "command", command = "pulseczar status --line", interval_seconds = 30, timeout_seconds = 3 } ]
```

tmux, in `~/.tmux.conf`:

```tmux
set -g status-right '#(NO_COLOR=1 pulseczar status --line)'
```

## Output

- On a terminal: colored status dots, human labels and relative times ("2 minutes ago"). Colors use the 16 standard colors and stop when `NO_COLOR` is set or `TERM=dumb`.
- Piped: the compact form, for scripts and agents.

  ```
  count: 2
  monitors[2]{id,name,status,uptime_24h}:
    7K3M9XQ2T4VB,Marketing site,up,99.98%
    B2C4D6F8G1H3,"Checkout, API",down,87.5%
  ```

- `--json`: the API body, unchanged.

The status is the word the web shows: Up, Down, Stale, Paused or New. A monitor with no result yet is New, never Up.

Errors go to stderr as `error:` and `code:` lines. On a terminal they show as a red `✗` line and a hint instead. `pulseczar --version` prints the build version. Exit codes: 0 ok, 1 error, 2 usage, 4 not signed in or not allowed.

## Develop

```sh
go test ./...                      # runs against a fake server; nothing calls production
go test ./internal/cli -update     # rewrite the golden files after a change you meant to make
```

## Release

Push a tag such as `v1.2.3`. The workflow in `.github/workflows/release.yml` runs the tests, then GoReleaser, which builds the archives and `pulseczar_<version>_checksums.txt` and creates the GitHub release. A tag with a suffix (`v1.2.3-rc.1`) makes a pre-release. When the repository secret `HOMEBREW_TAP_DEPLOY_KEY` is set, GoReleaser also pushes the cask to `pulseczar-com/homebrew-tap` over SSH; without it the cask is built and not pushed.
