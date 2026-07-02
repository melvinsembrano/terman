// Package tui implements terman's interactive terminal UI: a list of
// saved requests, a form to create/edit them, and a response viewer.
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

type screen int

const (
	screenList screen = iota
	screenEditor
	screenResponse
	screenEnvList
	screenEnvEditor
)

// headerLines is how many rows the header (title/env line + blank line)
// consumes, subtracted from the terminal height before sizing screens.
const headerLines = 2

type appModel struct {
	screen screen
	width  int
	height int

	list     listScreen
	editor   editorScreen
	response responseScreen

	envList   envListScreen
	envEditor envEditorScreen

	activeEnv string
	envs      []model.Environment
}

// Run starts the Bubble Tea program and blocks until the user quits.
func Run() error {
	m, err := newAppModel()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
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
	envLst, err := newEnvListScreen(active)
	if err != nil {
		return appModel{}, err
	}
	return appModel{
		screen:    screenList,
		activeEnv: active,
		envs:      envs,
		list:      lst,
		editor:    newEditorScreen(),
		response:  newResponseScreen(),
		envList:   envLst,
		envEditor: newEnvEditorScreen(),
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
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
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
	}
	return m, nil
}

func (m appModel) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && !m.list.isFiltering() {
		switch key.String() {
		case "q":
			return m, tea.Quit
		case "n":
			m.editor.loadNew()
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
				_ = store.DeleteRequest(req.Name)
				_ = m.list.refresh()
			}
			return m, nil
		case "E":
			m.cycleActiveEnv()
			return m, nil
		case "v":
			_ = m.envList.refresh(m.activeEnv)
			m.screen = screenEnvList
			return m, nil
		case "enter":
			if req, ok := m.list.selected(); ok {
				m.response.showRunning(req.Name)
				m.screen = screenResponse
				return m, runRequestCmd(req, m.activeEnvVars())
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list.lst, cmd = m.list.lst.Update(msg)
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
			if err := store.SaveRequest(req, m.editor.prevName); err != nil {
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
	if key, ok := msg.(tea.KeyMsg); ok && !m.envList.isFiltering() {
		switch key.String() {
		case "esc", "q":
			m.screen = screenList
			return m, nil
		case "n":
			m.envEditor.loadNew()
			m.screen = screenEnvEditor
			return m, nil
		case "e", "enter":
			if env, ok := m.envList.selected(); ok {
				m.envEditor.loadEnvironment(env)
				m.screen = screenEnvEditor
			}
			return m, nil
		case "d":
			if env, ok := m.envList.selected(); ok {
				_ = store.DeleteEnv(env.Name)
				_ = m.reloadEnvs()
				_ = m.envList.refresh(m.activeEnv)
			}
			return m, nil
		case "u":
			if env, ok := m.envList.selected(); ok {
				m.activeEnv = env.Name
				_ = store.SetActiveEnv(env.Name)
				_ = m.envList.refresh(m.activeEnv)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.envList.lst, cmd = m.envList.lst.Update(msg)
	return m, cmd
}

func (m appModel) updateEnvEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if !m.envEditor.editing {
				m.envEditor.err = ""
				m.screen = screenEnvList
				return m, nil
			}
		case "ctrl+s":
			if m.envEditor.editing {
				// Block saving while the row-edit modal is open.
				return m, nil
			}
			env := m.envEditor.toEnvironment()
			if env.Name == "" {
				m.envEditor.err = "name is required"
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
			if err := m.envList.refresh(m.activeEnv); err != nil {
				m.envEditor.err = err.Error()
				return m, nil
			}
			m.envEditor.err = ""
			m.screen = screenEnvList
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.envEditor, cmd = m.envEditor.Update(msg)
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
	_ = store.SetActiveEnv(m.activeEnv)
}

// reloadEnvs re-reads the saved environments from disk. If the currently
// active environment no longer exists (e.g. it was just deleted or renamed),
// the active environment is cleared and persisted as "".
func (m *appModel) reloadEnvs() error {
	envs, err := store.LoadEnvs()
	if err != nil {
		return err
	}
	m.envs = envs

	if m.activeEnv == "" {
		return nil
	}
	for _, e := range envs {
		if strings.EqualFold(e.Name, m.activeEnv) {
			return nil
		}
	}
	m.activeEnv = ""
	return store.SetActiveEnv("")
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
	env := m.activeEnv
	if env == "" {
		env = "none"
	}
	header := titleStyle.Render("terman") + "  " + subtleStyle.Render("env: "+env) + "\n\n"

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
	}
	return header
}
