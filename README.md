# terman — Terminal API Client

terman is a Terminal API Client — the parts of Postman or Insomnia you
actually reach for, rebuilt as a fast, keyboard-driven tool that never
leaves your terminal. Save a request once, organize the variables it needs
into named environments, and run it again in a second — interactively in
the TUI, or headlessly from the CLI when you're scripting, debugging over
SSH, or wiring a smoke test into CI.

Everything terman manages is a plain YAML file on disk (one per request,
one per environment), organized into folders you control — project-local
by default (a `.terman` directory next to the project it belongs to), no
account, no cloud sync, nothing to install beyond the binary. That also
means your requests and environments are just files: diff them, commit
`.terman` alongside the project, or edit them by hand when that's faster
than the TUI.

Environments carry the `{{variables}}` your requests reference (base URLs,
tokens, IDs), and can be built up manually, imported from a `.env` file, or
loaded for a single session/run without ever touching disk — so you can
try a value out without polluting a saved environment.

- Build and save requests (method, URL, headers, body) without leaving the terminal
- Organize requests into folders (e.g. `auth/login`) and browse them as a tree in the TUI
- Organize variables into environments — persisted, or loaded session-only from a `.env` file
- Run any saved request straight from the CLI (`terman run <name>`) for scripting and CI

## Getting started

```sh
terman init
```

Sets up `.terman` in the current directory with a working sample
environment (`dev`, pointing `base_url` at `https://httpbin.org`) and a
sample request (`Hello httpbin`) that uses it — so there's something to
run immediately:

```sh
terman run "Hello httpbin"
```

It's safe to run again later: `init` only fills in whatever's missing and
never overwrites an existing request or environment (pass `--force` to
reset the sample back to its defaults). See "Storage" below for exactly
where `.terman` ends up and how that's decided.

## Install

### Homebrew (macOS and Linux)

```sh
brew tap melvinsembrano/terman
brew trust melvinsembrano/terman
brew install terman
```

### Pre-built binaries

Download the archive for your platform from the
[latest release](https://github.com/melvinsembrano/terman/releases/latest):

| Platform                     | Archive                              |
|------------------------------|--------------------------------------|
| Linux (amd64)                | `terman-<version>-linux-amd64.tar.gz`  |
| Linux (arm64)                | `terman-<version>-linux-arm64.tar.gz`  |
| macOS (amd64, Intel)         | `terman-<version>-darwin-amd64.tar.gz` |
| macOS (arm64, Apple Silicon) | `terman-<version>-darwin-arm64.tar.gz` |
| Windows (amd64)              | `terman-<version>-windows-amd64.zip`   |

macOS/Linux — extract and put the binary on your `PATH`
(example for Apple Silicon macOS, replace `<version>` with the actual version):

```sh
curl -LO https://github.com/melvinsembrano/terman/releases/latest/download/terman-<version>-darwin-arm64.tar.gz
tar xzf terman-<version>-darwin-arm64.tar.gz
chmod +x terman
sudo mv terman /usr/local/bin/terman
```

macOS only: these binaries aren't code-signed, so Gatekeeper will refuse to
run a freshly downloaded one ("cannot be opened because the developer
cannot be verified"). Clear the quarantine flag once, after moving it into
place:

```sh
xattr -d com.apple.quarantine /usr/local/bin/terman
```

Windows: extract `terman-<version>-windows-amd64.zip` from the
[latest release](https://github.com/melvinsembrano/terman/releases/latest)
page and run `terman.exe` directly, or add its folder to your `PATH`.

### Using `go install`

Requires Go 1.19+:

```sh
go install github.com/melvinsembrano/terman@latest
```

This installs to `$(go env GOPATH)/bin` (typically `~/go/bin`) — make sure
that's on your `PATH`.

### Build from source

```sh
go build -o terman .
```

## TUI

Launch with no arguments:

```sh
terman
```

**List screen** (saved requests, shown as a navigable folder tree):

| Key      | Action                        |
|----------|--------------------------------|
| `enter`  | open the selected folder, or run the selected request |
| `esc`/`backspace` | go up a folder (no-op at the top level) |
| `n`      | new request (defaults to the folder you're browsing) |
| `e`      | edit the selected request      |
| `d`      | delete the selected request    |
| `E`      | cycle the active environment   |
| `v`      | manage environments            |
| `I`      | import a request from a curl command |
| `ctrl+t` | toggle mouse capture            |
| `/`      | search — matches name, method, URL, and folder across *every* folder, not just the one you're browsing |
| `q`      | quit                            |

**Editor screen:**

| Key             | Action                          |
|-----------------|----------------------------------|
| `tab`/`shift+tab` | move between fields            |
| `←`/`→`         | change HTTP method (when focused) |
| `ctrl+s`        | save                             |
| `esc`           | cancel                           |

Headers are entered one per line as `Key: Value`. URL, headers, and body may
reference environment variables as `{{name}}`. The Folder field (e.g.
`auth/oauth`) files the request into that folder in the tree; it defaults
to whatever folder you were browsing when you pressed `n`, and leaving it
blank keeps the request at the top level. Changing it on an existing
request moves the file to the new folder.

**Response screen:** while a request is in flight, a spinner and "sending
request…" message show so it's never just a blank screen. Once it
completes, the status line is colored by class (2xx green, 3xx cyan, 4xx
orange, 5xx red — the same convention curl/httpie/browser devtools use),
with clearly separated, labeled Headers and Body sections. JSON bodies
render as an interactive, syntax-highlighted tree, fx-style: `↑`/`↓` or a
click moves a line cursor, `enter`/`space` folds or unfolds the
object/array under it (collapsed containers show as `{…3}`/`[…5]`).
Non-JSON bodies stay plain scrollable text. `pgup`/`pgdn` and the mouse
wheel scroll either way; `esc` goes back.

### Mouse support

The TUI responds to the mouse: scroll the wheel in the request list, the
environment list, or the response viewer to move the selection/scroll;
click a row in the request list, the environment list, the environment
editor's variable rows, or a line in the response screen's JSON tree to
select it — never to run/open it, you still press enter for that, same as
with the keyboard. Press `ctrl+t` from any
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
terman init [--force]                # set up .terman here with a sample request + env

terman list                          # list saved requests (as group/name paths)
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

`<name>` above (for `run`, `import curl`) may be a bare request name or a
`group/name` path, e.g. `terman run auth/login` — see "Organizing requests
into folders" below.

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

### Organizing requests into folders

A request's name can be prefixed with a `/`-separated folder path — a
"group" — e.g. `auth/login`. On disk that's just a subfolder under
`requests/` (see "Storage" below); in the TUI it's a folder you browse into
in the request list (`enter` to open it, `esc` to go back up). From the
CLI, use the full `group/name` path anywhere a request name is expected:

```sh
terman run auth/login
terman import curl auth/login < curl.txt   # save straight into a folder
terman list                                # shows every request as its full group/name path
```

A bare name with no `/` still works and resolves normally, unless the same
name exists in more than one folder — then it's ambiguous and `run` reports
the full paths to choose from.

### Importing from curl

`terman import curl <name> [file]` reads a curl command from `file` if
given, otherwise from stdin — so paste-friendly input works without
fighting shell quoting (`<name>` may be a `group/name` path, see above):

```sh
pbpaste | terman import curl "Get Users"
terman import curl "Get Users" < curl.txt
terman import curl "Get Users" <<'EOF'
curl 'https://api.example.com/users' \
  -H 'Accept: application/json'
EOF
```

It's saved immediately (like `env set`, no confirmation step) under the
given name, overwriting any existing request with that name (in that same
folder). The TUI's `I` key (above) does the same parsing but hands off into
the editor for review before saving.

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

Requests and environments are stored as one YAML file each, in a `.terman`
directory resolved in this order:

1. `$XDG_CONFIG_HOME/terman`, if that's set — an explicit override.
2. The nearest `.terman` found by walking **up** from the current directory
   through its ancestors (project-local, git-style: works from any
   subdirectory of a project that already has one).
3. The legacy global `~/.config/terman`, if that already exists (so
   installs that predate project-local storage keep working).
4. Otherwise, a fresh `./.terman` in the current directory — created by
   `terman init` (see "Getting started" above), or lazily on first save.

`terman init` is the one exception to that walk-up: it always targets
`./.terman` in the *exact* current directory, the same way `git init` does,
even inside a subdirectory of an existing project.

Requests are organized into folders (see "Organizing requests into
folders" above) — a subdirectory per folder under `requests/`:

```
.terman/
├── config.yaml               # remembers the active environment
├── requests/
│   ├── hello-httpbin.yaml    # top-level request
│   └── auth/
│       └── login.yaml        # "auth/login"
└── envs/
    └── dev.yaml
```

Request file:

```yaml
name: Login
group: auth
method: POST
url: "{{base_url}}/login"
headers:
  Content-Type: application/json
body: '{"user": "{{user}}"}'
```

Environment file:

```yaml
name: dev
vars:
  base_url: https://httpbin.org
```

Since these are plain files, you can hand-edit, version-control, or share
them directly. A request's folder is derived from where its file actually
lives, not just the `group:` field — moving the file (or editing the
Folder field in the TUI) is what actually files it elsewhere.

## License

MIT — see [LICENSE](LICENSE).
