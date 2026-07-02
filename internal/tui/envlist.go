package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/melvinsembrano/terman/internal/model"
)

func envListHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "load session env")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "set active")),
	}
}

// envItem adapts model.Environment to bubbles/list's Item interface.
type envItem struct {
	env     model.Environment
	active  bool
	session bool
}

func (i envItem) Title() string {
	var tags []string
	if i.active {
		tags = append(tags, "active")
	}
	if i.session {
		tags = append(tags, "session")
	}
	if len(tags) == 0 {
		return i.env.Name
	}
	return i.env.Name + " (" + strings.Join(tags, ", ") + ")"
}

func (i envItem) Description() string {
	n := len(i.env.Vars)
	if n == 1 {
		return "1 variable"
	}
	return fmt.Sprintf("%d variables", n)
}

func (i envItem) FilterValue() string { return i.env.Name }

// envListScreen shows the saved and session-only environments. It holds no
// store connection of its own — it's a pure view over whatever appModel
// passes in, since session-only environments never exist on disk.
type envListScreen struct {
	lst list.Model
}

func envItems(envs []model.Environment, active string, sessionEnvs map[string]bool) []list.Item {
	items := make([]list.Item, len(envs))
	for i, e := range envs {
		items[i] = envItem{
			env:     e,
			active:  strings.EqualFold(e.Name, active),
			session: sessionEnvs[strings.ToLower(e.Name)],
		}
	}
	return items
}

func newEnvListScreen(envs []model.Environment, active string, sessionEnvs map[string]bool) envListScreen {
	delegate := list.NewDefaultDelegate()
	lst := list.New(envItems(envs, active, sessionEnvs), delegate, 0, 0)
	lst.Title = "Environments"
	lst.SetShowHelp(true)
	lst.AdditionalShortHelpKeys = envListHelpKeys
	lst.AdditionalFullHelpKeys = envListHelpKeys
	return envListScreen{lst: lst}
}

func (s *envListScreen) setSize(w, h int) {
	s.lst.SetSize(w, h)
}

// refresh replaces the list's items, marking active/session by name.
func (s *envListScreen) refresh(envs []model.Environment, active string, sessionEnvs map[string]bool) {
	s.lst.SetItems(envItems(envs, active, sessionEnvs))
}

func (s envListScreen) selected() (model.Environment, bool) {
	item, ok := s.lst.SelectedItem().(envItem)
	if !ok {
		return model.Environment{}, false
	}
	return item.env, true
}

func (s envListScreen) isFiltering() bool {
	return s.lst.SettingFilter()
}

func (s envListScreen) View() string {
	return s.lst.View()
}
