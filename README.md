# pulseczar

`pulseczar` shows and manages your [PulseCzar](https://pulseczar.com) monitors from the terminal. It is a single binary with no runtime.

## Install from source

You need Go 1.24 or newer.

```sh
go install github.com/pulseczar-com/cli/cmd/pulseczar@latest
```

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
| `status` | Whether your monitors are up, with the counts. |
| `monitors list [--status down\|stale\|paused]` | List your monitors. |
| `monitors show <id>` | One monitor, with its recent incidents. |
| `incidents list [--open] [--since 24h] [--monitor <id>]` | Incidents, newest first, 25 to a page (`--cursor` for the next page). |
| `incidents show <id>` | One incident and the locations that saw it. |
| `monitors create <url> [--name <name>] [--every 5m]` | Check an address. |
| `monitors create --heartbeat --name <name> --every 1d [--grace 1h]` | Wait for your job to ping. |
| `monitors pause <id>` / `monitors resume <id>` | Stop or restart a monitor. |
| `monitors snooze <id> --for 1h\|24h\|next\|forever` | Snooze alerts; the monitor keeps checking. |
| `monitors unsnooze <id>` | Turn alerts back on. |

Writes need a **Read and write** sign-in. `pulseczar <command> --help` shows flags and examples.

## Output

- On a terminal: an aligned table.
- Piped: the compact form, for scripts and agents.

  ```
  count: 2
  monitors[2]{id,name,status,uptime_24h}:
    7K3M9XQ2T4VB,Marketing site,up,99.98%
    B2C4D6F8G1H3,"Checkout, API",down,87.5%
  ```

- `--json`: the API body, unchanged.

Errors go to stderr as `error:` and `code:` lines. Exit codes: 0 ok, 1 error, 2 usage, 4 not signed in or not allowed.

## Develop

```sh
go test ./...                      # runs against a fake server; nothing calls production
go test ./internal/cli -update     # rewrite the golden files after a change you meant to make
```
