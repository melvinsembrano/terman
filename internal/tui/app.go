// Package tui implements terman's interactive terminal UI: a list of
// saved requests, a form to create/edit them, and a response viewer.
package tui

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/curl"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
	"github.com/melvinsembrano/terman/internal/version"
)

type screen int

// listCurlCopiedMsg is sent after the curl command for a request has been
// written to the clipboard, so the list title can be temporarily updated
// to confirm the action.
type listCurlCopiedMsg struct{ reqTitle string }

// listTitleResetMsg is sent after a short delay to restore the list title
// to its normal value following a "curl copied" confirmation.
type listTitleResetMsg struct{}

const (
	screenList screen = iota
	screenEditor
	screenResponse
	screenEnvList
	screenEnvEditor
	screenCurlImport
)

// headerLines is how many rows the header (title/env line, divider, blank
// line) consumes, subtracted from the terminal height before sizing
// screens.
const headerLines = 3

type appModel struct {
	screen screen
	width  int
	height int

	list     listScreen
	editor   editorScreen
	response responseScreen

	envList    envListScreen
	envEditor  envEditorScreen
	curlImport curlImportScreen

	activeEnv string
	envs      []model.Environment

	// sessionEnvs marks (lower-cased) names in envs that are in-memory
	// only — loaded via the env list's "L" key, never persisted to disk,
	// and gone when the program exits.
	sessionEnvs map[string]bool

	// mouseEnabled tracks whether mouse capture is currently on (toggled
	// with "m"). Starts true — mouse mode is enabled at Program startup
	// in Run().
	mouseEnabled bool
}

// Run starts the Bubble Tea program and blocks until the user quits.
func Run() error {
	m, err := newAppModel()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newAppModel() (appModel, error) {
	active, err := store.GetActiveEnv()
	if err != nil {
		return appModel{}, err
	}
	envs, err := store.LoadEnvs()
	if err != nil {
		return appModel{}, err
	}
	lst, err := newListScreen()
	if err != nil {
		return appModel{}, err
	}
	return appModel{
		screen:       screenList,
		activeEnv:    active,
		envs:         envs,
		list:         lst,
		editor:       newEditorScreen(),
		response:     newResponseScreen(),
		envList:      newEnvListScreen(envs, active, nil),
		envEditor:    newEnvEditorScreen(),
		curlImport:   newCurlImportScreen(),
		mouseEnabled: true,
	}, nil
}

func (m appModel) Init() tea.Cmd { return nil }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		bodyH := msg.Height - headerLines
		if bodyH < 1 {
			bodyH = 1
		}
		m.list.setSize(msg.Width, bodyH)
		m.editor.setSize(msg.Width, bodyH)
		m.response.setSize(msg.Width, bodyH)
		m.envList.setSize(msg.Width, bodyH)
		m.envEditor.setSize(msg.Width, bodyH)
		m.curlImport.setSize(msg.Width, bodyH)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t":
			// A bare letter (e.g. "m") isn't safe here: this branch runs
			// before per-screen dispatch, so it would swallow that
			// character everywhere it's typed into a text field (names,
			// URLs, bodies, curl commands, ...). A ctrl-chord, like the
			// existing ctrl+s/ctrl+c, can't collide with typed text.
			m.mouseEnabled = !m.mouseEnabled
			if m.mouseEnabled {
				return m, tea.EnableMouseCellMotion
			}
			return m, tea.DisableMouse
		}
	case listCurlCopiedMsg:
		// Show a transient confirmation in the list title and schedule a
		// reset so it reverts to the normal "Saved Requests" label.
		m.list.setCurlCopiedTitle(msg.reqTitle)
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return listTitleResetMsg{}
		})
	case listTitleResetMsg:
		m.list.applyView()
		return m, nil
	}

	switch m.screen {
	case screenList:
		return m.updateList(msg)
	case screenEditor:
		return m.updateEditor(msg)
	case screenResponse:
		return m.updateResponse(msg)
	case screenEnvList:
		return m.updateEnvList(msg)
	case screenEnvEditor:
		return m.updateEnvEditor(msg)
	case screenCurlImport:
		return m.updateCurlImport(msg)
	}
	return m, nil
}

func (m appModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if m.list.handleMouse(tea.MouseEvent(mouseMsg)) {
			return m, nil
		}
	}
	if key, ok := msg.(tea.KeyMsg); ok && !m.list.isFiltering() {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "n":
			m.editor.loadNew(m.list.curGroup)
			m.screen = screenEditor
			return m, nil
		case "e":
			if req, ok := m.list.selected(); ok {
				m.editor.loadRequest(req)
				m.screen = screenEditor
			}
			return m, nil
		case "d":
			if req, ok := m.list.selected(); ok {
				_ = store.DeleteRequest(req.Group, req.Name)
				_ = m.list.refresh()
			}
			return m, nil
		case "E":
			m.cycleActiveEnv()
			return m, nil
		case "v":
			_ = m.reloadEnvs()
			m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
			m.screen = screenEnvList
			return m, nil
		case "I":
			m.curlImport.loadNew()
			m.screen = screenCurlImport
			return m, nil
		case "x":
			if req, ok := m.list.selected(); ok {
				return m, exportCurlCmd(req, m.activeEnvVars())
			}
			return m, nil
		case "enter":
			if name, ok := m.list.selectedFolder(); ok {
				m.list.openFolder(name)
				return m, nil
			}
			if req, ok := m.list.selected(); ok {
				spinCmd := m.response.showRunning(req.Name)
				m.screen = screenResponse
				return m, tea.Batch(spinCmd, runRequestCmd(req, m.activeEnvVars()))
			}
			return m, nil
		case "esc", "backspace":
			if !m.list.filtered() && m.list.goUp() {
				return m, nil
			}
			// Otherwise (already at the top level, or a filter is active)
			// fall through to the list widget's own handling below, e.g.
			// clearing an applied filter.
		}
	}
	cmd := m.list.handleKey(msg)
	return m, cmd
}

func (m appModel) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.editor.err = ""
			m.screen = screenList
			return m, nil
		case "ctrl+s":
			req := m.editor.toRequest()
			if req.Name == "" {
				m.editor.err = "name is required"
				return m, nil
			}
			if err := store.SaveRequest(req, m.editor.prevName, m.editor.prevGroup); err != nil {
				m.editor.err = err.Error()
				return m, nil
			}
			if err := m.list.refresh(); err != nil {
				m.editor.err = err.Error()
				return m, nil
			}
			m.editor.err = ""
			m.screen = screenList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

func (m appModel) updateEnvList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if m.envList.handleMouse(tea.MouseEvent(mouseMsg)) {
			return m, nil
		}
	}
	if key, ok := msg.(tea.KeyMsg); ok && !m.envList.isFiltering() {
		switch key.String() {
		case "esc", "q":
			m.screen = screenList
			return m, nil
		case "n":
			m.envEditor.loadNew()
			m.screen = screenEnvEditor
			return m, nil
		case "L":
			m.envEditor.loadNew()
			m.envEditor.sessionOnly = true
			m.screen = screenEnvEditor
			return m, nil
		case "e", "enter":
			if env, ok := m.envList.selected(); ok && !m.isSessionEnv(env.Name) {
				m.envEditor.loadEnvironment(env)
				m.screen = screenEnvEditor
			}
			return m, nil
		case "d":
			if env, ok := m.envList.selected(); ok {
				if m.isSessionEnv(env.Name) {
					m.removeSessionEnv(env.Name)
				} else {
					_ = store.DeleteEnv(env.Name)
					_ = m.reloadEnvs()
				}
				m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
			}
			return m, nil
		case "u":
			if env, ok := m.envList.selected(); ok {
				m.activeEnv = env.Name
				if !m.isSessionEnv(env.Name) {
					_ = store.SetActiveEnv(env.Name)
				}
				m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.envList.lst, cmd = m.envList.lst.Update(msg)
	return m, cmd
}

func (m appModel) updateEnvEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if m.envEditor.handleMouse(tea.MouseEvent(mouseMsg)) {
			return m, nil
		}
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if !m.envEditor.editing && !m.envEditor.importing {
				m.envEditor.err = ""
				m.screen = screenEnvList
				return m, nil
			}
		case "ctrl+s":
			if m.envEditor.editing || m.envEditor.importing {
				// Block saving while a modal (row edit or file import) is open.
				return m, nil
			}
			env := m.envEditor.toEnvironment()
			if env.Name == "" {
				m.envEditor.err = "name is required"
				return m, nil
			}
			if m.envEditor.sessionOnly {
				m.addSessionEnv(env)
				m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
				m.envEditor.err = ""
				m.screen = screenEnvList
				return m, nil
			}
			if err := store.SaveEnv(env, m.envEditor.prevName); err != nil {
				m.envEditor.err = err.Error()
				return m, nil
			}
			if err := m.reloadEnvs(); err != nil {
				m.envEditor.err = err.Error()
				return m, nil
			}
			m.envList.refresh(m.envs, m.activeEnv, m.sessionEnvs)
			m.envEditor.err = ""
			m.screen = screenEnvList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.envEditor, cmd = m.envEditor.Update(msg)
	return m, cmd
}

func (m appModel) updateCurlImport(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.curlImport.err = ""
			m.screen = screenList
			return m, nil
		case "ctrl+s":
			req, err := m.curlImport.parse()
			if err != nil {
				m.curlImport.err = err.Error()
				return m, nil
			}
			m.editor.loadRequest(req)
			m.editor.prevName = "" // brand new request, not editing a saved one
			m.screen = screenEditor
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.curlImport, cmd = m.curlImport.Update(msg)
	return m, cmd
}

func (m appModel) updateResponse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runResultMsg:
		if msg.err != nil {
			m.response.showError(msg.name, msg.err)
		} else {
			m.response.showResult(msg.name, msg.resp)
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.screen = screenList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.response, cmd = m.response.Update(msg)
	return m, cmd
}

// cycleActiveEnv rotates through "" (no environment) plus every saved
// environment, persisting the new choice.
func (m *appModel) cycleActiveEnv() {
	names := []string{""}
	for _, e := range m.envs {
		names = append(names, e.Name)
	}
	idx := 0
	for i, n := range names {
		if strings.EqualFold(n, m.activeEnv) {
			idx = i
			break
		}
	}
	m.activeEnv = names[(idx+1)%len(names)]
	if m.isSessionEnv(m.activeEnv) {
		return // ephemeral; never persist a session env as the active one
	}
	_ = store.SetActiveEnv(m.activeEnv)
}

// reloadEnvs re-reads the saved environments from disk, then re-appends any
// session-only environments that aren't shadowed by a persisted one of the
// same name (if a persisted env now shares the name, it wins and the
// session marker is dropped). If the currently active environment no
// longer exists anywhere, the active environment is cleared and (for a
// persisted active env) that clearing is itself persisted as "".
func (m *appModel) reloadEnvs() error {
	persisted, err := store.LoadEnvs()
	if err != nil {
		return err
	}

	havePersisted := map[string]bool{}
	for _, e := range persisted {
		havePersisted[strings.ToLower(e.Name)] = true
	}

	wasSessionActive := m.isSessionEnv(m.activeEnv)

	merged := persisted
	for _, e := range m.envs {
		lower := strings.ToLower(e.Name)
		if !m.sessionEnvs[lower] {
			continue
		}
		if havePersisted[lower] {
			delete(m.sessionEnvs, lower)
			continue
		}
		merged = append(merged, e)
	}
	m.envs = merged

	if m.activeEnv == "" {
		return nil
	}
	for _, e := range merged {
		if strings.EqualFold(e.Name, m.activeEnv) {
			return nil
		}
	}
	m.activeEnv = ""
	if wasSessionActive {
		return nil // was never persisted; nothing to clear on disk
	}
	return store.SetActiveEnv("")
}

// isSessionEnv reports whether name refers to an in-memory-only
// environment.
func (m appModel) isSessionEnv(name string) bool {
	return m.sessionEnvs[strings.ToLower(name)]
}

// addSessionEnv upserts env into m.envs as a session-only environment
// (never written to disk) and makes it the active environment.
func (m *appModel) addSessionEnv(env model.Environment) {
	replaced := false
	for i, e := range m.envs {
		if strings.EqualFold(e.Name, env.Name) {
			m.envs[i] = env
			replaced = true
			break
		}
	}
	if !replaced {
		m.envs = append(m.envs, env)
	}
	if m.sessionEnvs == nil {
		m.sessionEnvs = map[string]bool{}
	}
	m.sessionEnvs[strings.ToLower(env.Name)] = true
	m.activeEnv = env.Name
}

// removeSessionEnv drops a session-only environment from memory. If it was
// active, the active environment is cleared in memory only (never
// persisted, since it was never saved to disk in the first place).
func (m *appModel) removeSessionEnv(name string) {
	lower := strings.ToLower(name)
	delete(m.sessionEnvs, lower)
	for i, e := range m.envs {
		if strings.EqualFold(e.Name, name) {
			m.envs = append(m.envs[:i], m.envs[i+1:]...)
			break
		}
	}
	if strings.EqualFold(m.activeEnv, name) {
		m.activeEnv = ""
	}
}

// exportCurlCmd builds the curl command for req with variables resolved,
// writes it to the system clipboard, and returns a tea.Msg that carries
// the result back to Update so the list title can be updated.
func exportCurlCmd(req model.Request, v map[string]string) tea.Cmd {
	return func() tea.Msg {
		cmd := curl.ToCurl(req, v)
		if err := clipboard.WriteAll(cmd); err != nil {
			// On error fall through silently; the user still has the
			// request and can use the CLI instead.
			return nil
		}
		return listCurlCopiedMsg{reqTitle: req.Name}
	}
}

func (m appModel) activeEnvVars() map[string]string {
	for _, e := range m.envs {
		if strings.EqualFold(e.Name, m.activeEnv) {
			return e.Vars
		}
	}
	return nil
}

func (m appModel) View() string {
	envLabel := subtleStyle.Render("env: ")
	if m.activeEnv == "" {
		envLabel += subtleStyle.Render("none")
	} else {
		envLabel += statusOKStyle.Render(m.activeEnv)
	}
	header := titleStyle.Render("terman") + " " + subtleStyle.Render("v"+version.Version) +
		subtleStyle.Render("  •  ") + envLabel
	if !m.mouseEnabled {
		header += subtleStyle.Render("  •  mouse: off")
	}
	header += "\n"
	if m.width > 0 {
		header += dividerStyle.Render(strings.Repeat("─", m.width))
	}
	header += "\n"

	switch m.screen {
	case screenList:
		return header + m.list.View()
	case screenEditor:
		return header + m.editor.View()
	case screenResponse:
		return header + m.response.View()
	case screenEnvList:
		return header + m.envList.View()
	case screenEnvEditor:
		return header + m.envEditor.View()
	case screenCurlImport:
		return header + m.curlImport.View()
	}
	return header
}
