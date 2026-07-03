# terman — Terminal API Client

terman is a Terminal API Client — the parts of Postman or Insomnia you
actually reach for, rebuilt as a fast, keyboard-driven tool that never
leaves your terminal. Save a request once, organize the variables it needs
into named environments, and run it again in a second — interactively in
the TUI, or headlessly from the CLI when you're scripting, debugging over
SSH, or wiring a smoke test into CI.

Everything terman manages is a plain YAML file on disk (one per request,
one per environment) under `~/.config/terman` — no account, no cloud sync,
nothing to install beyond the binary. That also means your requests and
environments are just files: diff them, put them in git alongside the
project they belong to, or edit them by hand when that's faster than the
TUI.

Environments carry the `{{variables}}` your requests reference (base URLs,
tokens, IDs), and can be built up manually, imported from a `.env` file, or
loaded for a single session/run without ever touching disk — so you can
try a value out without polluting a saved environment.

- Build and save requests (method, URL, headers, body) without leaving the terminal
- Organize variables into environments — persisted, or loaded session-only from a `.env` file
- Run any saved request straight from the CLI (`terman run <name>`) for scripting and CI

## Install

### Pre-built binaries

Download the binary for your platform from the
[latest release](https://github.com/melvinsembrano/terman/releases/latest):

| Platform                     | Binary              |
|-------------------------------|---------------------|
| Linux (amd64)                 | `terman-linux`      |
| Linux (arm64)                 | `terman-linux-arm`  |
| macOS (amd64, Intel)          | `terman-mac`        |
| macOS (arm64, Apple Silicon)  | `terman-mac-arm`    |
| Windows (amd64)               | `terman.exe`        |

macOS/Linux — download, make it executable, and put it on your `PATH`
(example for Apple Silicon macOS):

```sh
curl -LO https://github.com/melvinsembrano/terman/releases/latest/download/terman-mac-arm
chmod +x terman-mac-arm
sudo mv terman-mac-arm /usr/local/bin/terman
```

macOS only: these binaries aren't code-signed, so Gatekeeper will refuse to
run a freshly downloaded one ("cannot be opened because the developer
cannot be verified"). Clear the quarantine flag once, after moving it into
place:

```sh
xattr -d com.apple.quarantine /usr/local/bin/terman
```

Windows: download `terman.exe` from the same
[latest release](https://github.com/melvinsembrano/terman/releases/latest)
page and run it directly, or add its folder to your `PATH`.

### Build from source

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
| `I`      | import a request from a curl command |
| `ctrl+t` | toggle mouse capture            |
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

### Mouse support

The TUI responds to the mouse: scroll the wheel in the request list, the
environment list, or the response viewer to move the selection/scroll;
click a row in the request list, the environment list, or the environment
editor's variable rows to select it — never to run/open it, you still
press enter for that, same as with the keyboard. Press `ctrl+t` from any
screen to toggle mouse capture off and on: most terminals need a modifier
key held (e.g. Option/Shift while dragging) to select/copy text normally
while an app has the mouse captured, so `ctrl+t` lets you turn it off for
that and back on when you're done — the header shows `mouse: off` as a
reminder while it's disabled. Text fields (request name/URL/headers/body,
the curl-paste box, environment names) have no mouse support — clicking or
scrolling over them does nothing.

### Importing a request from curl

Press `I` from the request list to paste in a curl command (a name field
plus a multi-line box for the command). `ctrl+s` parses it and hands you
off into the regular request editor — pre-filled with the method, URL,
headers, and body it found — so you can review or tweak before saving with
`ctrl+s` again. `esc` at either step discards it; nothing is saved until
you save from the editor. See "Importing from curl" under CLI below for
exactly what's understood.

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
| `ctrl+t`  | toggle mouse capture              |
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

terman import curl <name> [file]     # save a request parsed from a curl command

terman version                       # print the version (also shown in the TUI header)
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

### Importing from curl

`terman import curl <name> [file]` reads a curl command from `file` if
given, otherwise from stdin — so paste-friendly input works without
fighting shell quoting:

```sh
pbpaste | terman import curl "Get Users"
terman import curl "Get Users" < curl.txt
terman import curl "Get Users" <<'EOF'
curl 'https://api.example.com/users' \
  -H 'Accept: application/json'
EOF
```

It's saved immediately (like `env set`, no confirmation step) under the
given name, overwriting any existing request with that name. The TUI's `I`
key (above) does the same parsing but hands off into the editor for review
before saving.

Understood flags: `-X`/`--request`; `-H`/`--header` (repeatable);
`-d`/`--data`/`--data-raw`/`--data-ascii`/`--data-binary`/`--data-urlencode`
(repeatable, joined with `&`; implies method `POST` unless `-X` is given);
`-u`/`--user` (→ a `Basic` auth `Authorization` header); `-G`/`--get`
(moves the assembled data into the URL's query string instead of the
body); `-A`/`--user-agent`; `-e`/`--referer`; `-b`/`--cookie`; `--url`.
Both `--flag=value` and glued short forms (`-XPOST`, `-H'Accept: json'`)
are understood, and a leading `curl` word is optional. Unrecognized flags
are ignored rather than rejected — real-world curl commands (especially
ones copied from browser devtools) are full of flags irrelevant to
building the request, like `--compressed`, `-k`, `-s`, `-L`, or
`--connect-timeout 5`.

Known limitations (not supported): TLS options (`-k`/`--insecure` — TLS
verification always stays on); `--compressed` is a deliberate no-op (Go's
HTTP client already negotiates and transparently decompresses gzip by
default, so translating it into a header would actually break that); file
uploads (`-F`/`--form`, `@file` data); ANSI-C `$'...'` quoting.

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

## License

MIT — see [LICENSE](LICENSE).
