package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func listHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "run")),
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "cycle env")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "manage envs")),
		key.NewBinding(key.WithKeys("I"), key.WithHelp("I", "import curl")),
	}
}

// requestItem adapts model.Request to bubbles/list's Item interface.
type requestItem struct {
	req model.Request
}

func (i requestItem) Title() string       { return i.req.Name }
func (i requestItem) Description() string { return i.req.Method + "  " + i.req.URL }
func (i requestItem) FilterValue() string { return i.req.Name }

// listScreen shows the saved requests.
type listScreen struct {
	lst list.Model
}

func requestItems() ([]list.Item, error) {
	reqs, err := store.LoadRequests()
	if err != nil {
		return nil, err
	}
	items := make([]list.Item, len(reqs))
	for i, r := range reqs {
		items[i] = requestItem{req: r}
	}
	return items, nil
}

func newListScreen() (listScreen, error) {
	items, err := requestItems()
	if err != nil {
		return listScreen{}, err
	}
	delegate := list.NewDefaultDelegate()
	lst := list.New(items, delegate, 0, 0)
	lst.Title = "Saved Requests"
	lst.SetShowHelp(true)
	lst.AdditionalShortHelpKeys = listHelpKeys
	lst.AdditionalFullHelpKeys = listHelpKeys
	return listScreen{lst: lst}, nil
}

func (s *listScreen) setSize(w, h int) {
	s.lst.SetSize(w, h)
}

func (s *listScreen) refresh() error {
	items, err := requestItems()
	if err != nil {
		return err
	}
	s.lst.SetItems(items)
	return nil
}

func (s listScreen) selected() (model.Request, bool) {
	item, ok := s.lst.SelectedItem().(requestItem)
	if !ok {
		return model.Request{}, false
	}
	return item.req, true
}

func (s listScreen) isFiltering() bool {
	return s.lst.SettingFilter()
}

func (s listScreen) View() string {
	return s.lst.View()
}
