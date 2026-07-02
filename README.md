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
| `enter`/`e` | edit the selected environment  |
| `n`       | new environment                  |
| `d`       | delete the selected environment  |
| `u`       | set the selected environment active |
| `esc`/`q` | back to the request list         |

**Environment editor screen** (name field + a row-based list of variables):

| Key             | Action                              |
|-----------------|--------------------------------------|
| `tab`           | move between the name field and variable rows |
| `↑`/`↓`         | select a variable row               |
| `a`             | add a variable                      |
| `enter`         | edit the selected variable           |
| `d`             | delete the selected variable          |
| `ctrl+s`        | save                                  |
| `esc`           | cancel (or close the row editor, if open) |

Deleting or renaming the currently active environment resets the active
environment to "none".

## CLI

```sh
terman list                          # list saved requests
terman run <name> [flags]            # run a saved request

terman env list                      # list saved environments (* marks active)
terman env show <name>               # print an environment's variables
terman env set <name> <k=v>...       # create/update variables (repeatable)
terman env unset <name> <key>...     # remove variables
terman env delete <name>             # delete an environment
terman env use <name>                # set the active environment
```

`run` flags:

- `--env <name>` — use this environment instead of the active one
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
