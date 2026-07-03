# AGENTS.md

Guidance for coding agents (and humans) working in this repository.

## Project overview

`terman` is a Terminal API Client: a Bubble Tea TUI and CLI for building,
saving, and organizing HTTP requests and the environments that parameterize
them — then running them interactively or straight from the command line
for scripting and CI. Module path: `github.com/melvinsembrano/terman`. Go
version: **1.19** (see "Pinned dependency versions" below before upgrading).

## Build & test commands

```sh
go build -o terman .        # build the binary
go vet ./...                 # static checks (no linter is configured)
go test ./...                 # run all tests
go test ./... -cover -race     # full check used throughout development — run this before considering work done
```

There is no CI configuration and no linter beyond `go vet` — these commands
are the full verification bar for this repo.

## Code layout

- `main.go` — CLI dispatch (`run`, `list`, `env ...`, `import ...`) and the
  no-args TUI entry point.
- `internal/model` — persisted data structs (`Request`, `Environment`).
- `internal/store` — YAML persistence, one file per request/environment,
  under `$XDG_CONFIG_HOME/terman` (or `~/.config/terman`).
- `internal/vars` — `{{var}}` substitution and layered `Merge`.
- `internal/httpx` — builds/executes the HTTP request, formats the response.
- `internal/dotenv` — a small, dependency-free `.env` file parser.
- `internal/curl` — a small, dependency-free curl-command parser/tokenizer
  used by "import curl" on both surfaces.
- `internal/version` — the single `Version` constant shown by `terman
  version`/`--version` and in the TUI header.
- `internal/tui` — the Bubble Tea screens (request list/editor/response,
  env list/editor, curl import) and the root `appModel`.
  `internal/tui/mouse.go` holds the shared wheel-scroll/click-to-select
  math for the two `list.Model`-based screens (see bespoke convention #9).

## Code style

Standard Go idioms, consistent with what's already in the codebase:
- Doc comments on exported identifiers.
- Error wrapping with `fmt.Errorf("...: %w", err)`.
- Table-driven tests where the cases are homogeneous; otherwise one test
  function per behavior with a descriptive name
  (e.g. `TestSaveEnvRenameRemovesOldFile`).
- Small, composable helpers over duplicated logic (e.g. `upsertEnvVars` in
  `main.go` is shared by `env set` and `env import`).

## Bespoke project conventions

These are decisions specific to this repo that aren't obvious from the code
alone — read before making changes in these areas.

1. **Go 1.19 / pinned Charm library versions.** `go.mod` deliberately pins
   `github.com/charmbracelet/bubbletea@v0.25.0`,
   `github.com/charmbracelet/bubbles@v0.18.0`, and
   `github.com/charmbracelet/lipgloss@v0.9.1` instead of latest. Newer
   versions pull in `x/cellbuf`, which imports the stdlib `slices` package
   and requires Go 1.21+. **Do not run a bare `go get -u ./...`** on these —
   it will break the build on this Go toolchain. If you need a newer Charm
   version, first confirm (or explicitly raise) the Go version with the
   user.

2. **No new dependencies without a clear reason.** The project intentionally
   stays minimal: `bubbletea`, `bubbles`, `lipgloss`, `gopkg.in/yaml.v3` are
   the only direct dependencies. `internal/dotenv` was hand-written rather
   than pulling in `github.com/joho/godotenv` for exactly this reason.
   Default to a small stdlib-based implementation before reaching for a
   dependency.

3. **Test isolation is mandatory.** Any test that touches `internal/store`
   or TUI environment state (which reads/writes `~/.config/terman`) must
   start with:
   ```go
   t.Setenv("XDG_CONFIG_HOME", t.TempDir())
   ```
   Every existing test file that needs it does this
   (`internal/store/store_test.go`, `main_test.go`,
   `internal/tui/*_test.go`). Skipping it means the test reads/writes the
   developer's real config directory.

4. **CLI/TUI parity.** Persisted mutations exposed on one surface should
   exist on the other. For example, environment management mirrors exactly:
   `env set/import/unset/delete/use` on the CLI correspond to the TUI env
   editor's save/import-from-file/delete/set-active actions. When adding a
   new persisted capability, add both surfaces.

5. **Session-only vs. persisted environment state.** `internal/tui/app.go`'s
   `appModel.envs` holds a mix of persisted environments (from
   `internal/store`) and in-memory-only "session" environments (loaded via
   the TUI's `L` key, or the CLI's `run --env-file`, which is a
   single-invocation equivalent that never touches `appModel` at all).
   Session environments must never be written to disk. Any code that
   mutates `m.envs` or the active environment must check
   `m.isSessionEnv(name)` before calling `store.SaveEnv` /
   `store.SetActiveEnv` / `store.DeleteEnv`. See `addSessionEnv`,
   `removeSessionEnv`, and `reloadEnvs` in `internal/tui/app.go` for the
   existing pattern to follow.

6. **Clean up stray build artifacts.** Running `go build -o terman .` (or
   occasionally a bare `go build` in this directory) leaves a `terman`
   binary in the repo root. It's git-ignored (`/terman` in `.gitignore`)
   but should be deleted after manual/smoke testing to keep the working
   tree tidy.

7. **Testing patterns already established — reuse them:**
   - `httptest.NewServer` for anything exercising `internal/httpx.Do`
     (assert on the method/path/headers/body the server actually received).
   - `main_test.go`'s `captureStdout(t, func() { ... })` helper for
     asserting on CLI stdout output.
   - TUI tests call the unexported `update*` methods and screen helpers
     directly (no real `tea.Program` needed), driving them with synthetic
     messages like `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}`,
     `tea.KeyMsg{Type: tea.KeyCtrlS}`, or
     `tea.MouseMsg{Y: ..., Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}`.

8. **Versioning is a hand-bumped constant.** `internal/version.Version` is
   edited by hand when cutting a release — there's no ldflags injection,
   build tooling, or git tags yet. Bump it in the same commit as the
   changes it covers.

9. **List click-to-select math depends on the pinned bubbles version.**
   `bubbles@v0.18.0`'s `list.Model` has no built-in mouse handling, and the
   view methods that determine its title/status-bar/pagination heights are
   unexported — there's no way to query them at runtime. `internal/tui/mouse.go`'s
   `listContentTop` constant instead relies on two things staying true:
   `newListScreen`/`newEnvListScreen` (internal/tui/list.go,
   internal/tui/envlist.go) keep the status bar and pagination indicator
   turned off, and the pinned version's default `TitleBar` style
   (`Padding(0,0,1,2)`) renders a fixed 2-line title block for our short,
   non-wrapping titles. Bumping bubbles (see convention #1) or changing
   either `SetShowStatusBar`/`SetShowPagination` call means re-deriving
   this constant — the `TestListClickSelectsRow`/`TestListClickOutsideContentIsNoop`
   tests in `internal/tui/app_test.go` will fail loudly if it's wrong.
   `envEditorScreen`'s row clicks (internal/tui/enveditor.go) don't have
   this problem — that screen renders its own rows, so its offset constant
   is derived directly from code we own, not from an unexported dependency.
