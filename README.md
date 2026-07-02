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

## CLI

```sh
terman list                     # list saved requests
terman env list                 # list saved environments (* marks active)
terman env use <name>           # set the active environment
terman run <name> [flags]       # run a saved request
```

`run` flags:

- `--env <name>` — use this environment instead of the active one
- `--var k=v` — override/add a variable (repeatable)
- `-i` — also print response headers

Example:

```sh
terman run "Get Anything" --env dev --var msg=hi -i
```

Exit code is non-zero if the request errors or the response status is not
2xx, so it's safe to use in scripts / CI.

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
