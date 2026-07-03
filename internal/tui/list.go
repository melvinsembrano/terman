package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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
		key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "toggle mouse")),
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
	// delegate is the same value lst was built with, kept here because
	// list.Model exposes no getter for it — needed by handleMouse to
	// compute row heights for click hit-testing.
	delegate list.DefaultDelegate
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
	// Turned off so click-to-select math (listContentTop, mouse.go) can
	// rely on an exact, fixed title-block height instead of the variable
	// height these would otherwise add.
	lst.SetShowStatusBar(false)
	lst.SetShowPagination(false)
	lst.AdditionalShortHelpKeys = listHelpKeys
	lst.AdditionalFullHelpKeys = listHelpKeys
	return listScreen{lst: lst, delegate: delegate}, nil
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

// handleMouse applies wheel-scroll and click-to-select (see mouse.go). It
// reports whether it consumed the event.
func (s *listScreen) handleMouse(msg tea.MouseEvent) bool {
	return listMouseEvent(msg, &s.lst, s.delegate)
}

func (s listScreen) View() string {
	return s.lst.View()
}
