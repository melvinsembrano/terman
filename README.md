# terman

A terminal, simplified Postman: a TUI for building and saving HTTP requests,
plus a CLI for running saved requests directly (e.g. from scripts).

## Install / build

```sh
go build -o terman .
```

## TUI

Launch with no arguments:

```sh
terman
```

**List screen** (saved requests):

| Key      | Action                        |
|----------|--------------------------------|
| `enter`  | run the selected request       |
| `n`      | new request                    |
| `e`      | edit the selected request      |
| `d`      | delete the selected request    |
| `E`      | cycle the active environment   |
| `v`      | manage environments            |
| `/`      | filter                         |
| `q`      | quit                            |

**Editor screen:**

| Key             | Action                          |
|-----------------|----------------------------------|
| `tab`/`shift+tab` | move between fields            |
| `←`/`→`         | change HTTP method (when focused) |
| `ctrl+s`        | save                             |
| `esc`           | cancel                           |

Headers are entered one per line as `Key: Value`. URL, headers, and body may
reference environment variables as `{{name}}`.

**Response screen:** `↑`/`↓`/page up/down to scroll, `esc` to go back.

### Managing environments

Press `v` from the request list to open the environment manager:

**Environment list screen:**

| Key       | Action                          |
|-----------|-----------------------------------|
| `enter`/`e` | edit the selected environment (persisted environments only) |
| `n`       | new environment                  |
| `L`       | load a **session-only** environment (see below) |
| `d`       | delete the selected environment  |
| `u`       | set the selected environment active |
| `esc`/`q` | back to the request list         |

**Environment editor screen** (name field + a row-based list of variables):

| Key             | Action                              |
|-----------------|--------------------------------------|
| `tab`           | move between the name field and variable rows |
| `↑`/`↓`         | select a variable row               |
| `a`             | add a variable                      |
| `i`             | import variables from a `.env` file (merges into the rows above) |
| `enter`         | edit the selected variable           |
| `d`             | delete the selected variable          |
| `ctrl+s`        | save                                  |
| `esc`           | cancel (or close whichever modal is open, if any) |

Deleting or renaming the currently active environment resets the active
environment to "none".

### Session-only environments

Pressing `L` on the environment list opens the same editor, but saving
(`ctrl+s`) never writes to disk: the environment lives only in the running
TUI's memory, is marked `(session)` in the list, is set active immediately,
and disappears the moment you quit. Use `i` inside it to load variables
straight from a `.env` file. This is the TUI equivalent of the CLI's
`run --env-file` (below) — a way to try out variables without creating a
permanent saved environment.

## CLI

```sh
terman list                          # list saved requests
terman run <name> [flags]            # run a saved request

terman env list                      # list saved environments (* marks active)
terman env show <name>               # print an environment's variables
terman env set <name> <k=v>...       # create/update variables (repeatable)
terman env import <file> <name>      # merge a .env file's variables into an environment
terman env unset <name> <key>...     # remove variables
terman env delete <name>             # delete an environment
terman env use <name>                # set the active environment
```

`run` flags:

- `--env <name>` — use this environment instead of the active one
- `--env-file <path>` — load extra variables from a `.env` file, just for this
  run (not saved anywhere — see "Loading `.env` files" below)
- `--var k=v` — override/add a variable (repeatable)
- `-i` — also print response headers

Example:

```sh
terman env set dev base_url=https://httpbin.org token=abc123
terman env use dev
terman run "Get Anything" --var msg=hi -i
```

Exit code is non-zero if the request errors or the response status is not
2xx, so it's safe to use in scripts / CI.

Deleting the currently active environment (`env delete`) resets the active
environment to "none".

### Loading `.env` files

Two ways to bring in variables from a dotenv-style file (`.env`,
`.env.local`, `.env.production`, ...):

- **Persisted import** — `terman env import <file> <name>` parses the file
  and **merges** its variables into the named environment (creating it if
  needed), same upsert behavior as `env set`.
- **Session-only, CLI** — `terman run <name> --env-file <path>` overlays the
  file's variables on top of the active/`--env` environment for that single
  run only. Nothing is written to disk. Precedence (lowest to highest):
  active/`--env` environment → `--env-file` → `--var`.
- **Session-only, TUI** — see "Session-only environments" above.

`.env` file format: `KEY=VALUE` per line, blank lines and `#` comments
ignored, an optional `export ` prefix, and values may be single- or
double-quoted (double-quoted values support `\n`, `\t`, `\"`, `\\` escapes).

## Storage

Requests and environments are stored as one YAML file each, under
`$XDG_CONFIG_HOME/terman` (or `~/.config/terman` if unset):

```
~/.config/terman/
├── config.yaml          # remembers the active environment
├── requests/
│   └── get-anything.yaml
└── envs/
    └── dev.yaml
```

Request file:

```yaml
name: Get Anything
method: GET
url: "{{base_url}}/get?msg=hello"
headers:
  X-Test-Header: "{{msg}}"
```

Environment file:

```yaml
name: dev
vars:
  base_url: https://httpbin.org
```

Since these are plain files, you can hand-edit, version-control, or share
them directly.
