package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
)

func envListHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clone")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "load session env")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "set active")),
		key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "toggle mouse")),
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
	// delegate is the same value lst was built with, kept here because
	// list.Model exposes no getter for it — needed by handleMouse to
	// compute row heights for click hit-testing.
	delegate list.DefaultDelegate
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
	delegate := newStyledDelegate()
	lst := list.New(envItems(envs, active, sessionEnvs), delegate, 0, 0)
	lst.Title = "Environments"
	lst.SetShowHelp(true)
	// Turned off so click-to-select math (listContentTop, mouse.go) can
	// rely on an exact, fixed title-block height instead of the variable
	// height these would otherwise add.
	lst.SetShowStatusBar(false)
	lst.SetShowPagination(false)
	lst.AdditionalShortHelpKeys = envListHelpKeys
	lst.AdditionalFullHelpKeys = envListHelpKeys
	styleListHelp(&lst)
	return envListScreen{lst: lst, delegate: delegate}
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

// handleMouse applies wheel-scroll and click-to-select (see mouse.go). It
// reports whether it consumed the event.
func (s *envListScreen) handleMouse(msg tea.MouseEvent) bool {
	return listMouseEvent(msg, &s.lst, s.delegate)
}

func (s envListScreen) View() string {
	return s.lst.View()
}
