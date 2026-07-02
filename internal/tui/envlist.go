package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func envListHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "set active")),
	}
}

// envItem adapts model.Environment to bubbles/list's Item interface.
type envItem struct {
	env    model.Environment
	active bool
}

func (i envItem) Title() string {
	if i.active {
		return i.env.Name + " (active)"
	}
	return i.env.Name
}

func (i envItem) Description() string {
	n := len(i.env.Vars)
	if n == 1 {
		return "1 variable"
	}
	return fmt.Sprintf("%d variables", n)
}

func (i envItem) FilterValue() string { return i.env.Name }

// envListScreen shows the saved environments.
type envListScreen struct {
	lst list.Model
}

func envItems(active string) ([]list.Item, error) {
	envs, err := store.LoadEnvs()
	if err != nil {
		return nil, err
	}
	items := make([]list.Item, len(envs))
	for i, e := range envs {
		items[i] = envItem{env: e, active: strings.EqualFold(e.Name, active)}
	}
	return items, nil
}

func newEnvListScreen(active string) (envListScreen, error) {
	items, err := envItems(active)
	if err != nil {
		return envListScreen{}, err
	}
	delegate := list.NewDefaultDelegate()
	lst := list.New(items, delegate, 0, 0)
	lst.Title = "Environments"
	lst.SetShowHelp(true)
	lst.AdditionalShortHelpKeys = envListHelpKeys
	lst.AdditionalFullHelpKeys = envListHelpKeys
	return envListScreen{lst: lst}, nil
}

func (s *envListScreen) setSize(w, h int) {
	s.lst.SetSize(w, h)
}

// refresh reloads the environment list, marking active by name.
func (s *envListScreen) refresh(active string) error {
	items, err := envItems(active)
	if err != nil {
		return err
	}
	s.lst.SetItems(items)
	return nil
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
