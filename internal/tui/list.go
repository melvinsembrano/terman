package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
	"github.com/melvinsembrano/terman/internal/store"
)

func listHelpKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/run")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "up a folder")),
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
// showGroup is set while browsing the cross-folder search view (see
// searchItems) so results from every folder can be told apart.
type requestItem struct {
	req       model.Request
	showGroup bool
}

func (i requestItem) Title() string { return i.req.Name }

func (i requestItem) Description() string {
	desc := i.req.Method + "  " + i.req.URL
	if i.showGroup && i.req.Group != "" {
		desc = i.req.Group + "  •  " + desc
	}
	return desc
}

// FilterValue widens the built-in "/" filter to match a request's name,
// method, URL, and group, not just its name.
func (i requestItem) FilterValue() string {
	parts := []string{i.req.Name, i.req.Method, i.req.URL}
	if i.req.Group != "" {
		parts = append(parts, i.req.Group)
	}
	return strings.Join(parts, " ")
}

// folderItem represents a subfolder of the group currently being browsed,
// shown above the requests it directly contains.
type folderItem struct {
	name  string // this folder's own path segment, not its full group path
	count int    // number of requests nested (at any depth) inside it
}

func (i folderItem) Title() string { return "▸ " + i.name + "/" }

func (i folderItem) Description() string {
	if i.count == 1 {
		return "1 request"
	}
	return fmt.Sprintf("%d requests", i.count)
}

func (i folderItem) FilterValue() string { return i.name }

// isDescendantGroup reports whether group is a strict descendant of
// parent, i.e. parent itself plus at least one more path segment.
func isDescendantGroup(parent, group string) bool {
	if group == parent {
		return false
	}
	if parent == "" {
		return group != ""
	}
	return strings.HasPrefix(group, parent+"/")
}

// firstSegmentAfter returns group's path segment immediately below
// parent, e.g. firstSegmentAfter("auth", "auth/oauth/v2") == "oauth".
func firstSegmentAfter(parent, group string) string {
	rest := group
	if parent != "" {
		rest = strings.TrimPrefix(group, parent+"/")
	}
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}

// childItems builds the items to show while browsing group: its immediate
// subfolders (sorted, with a recursive request count) followed by the
// requests directly inside it.
func childItems(all []model.Request, group string) []list.Item {
	counts := map[string]int{}
	var reqs []list.Item
	for _, r := range all {
		if r.Group == group {
			reqs = append(reqs, requestItem{req: r})
			continue
		}
		if isDescendantGroup(group, r.Group) {
			counts[firstSegmentAfter(group, r.Group)]++
		}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]list.Item, 0, len(names)+len(reqs))
	for _, name := range names {
		items = append(items, folderItem{name: name, count: counts[name]})
	}
	return append(items, reqs...)
}

// searchItems flattens every request across every group into a single
// list, used while the list's own "/" filter is active so search isn't
// confined to the folder currently being browsed.
func searchItems(all []model.Request) []list.Item {
	items := make([]list.Item, len(all))
	for i, r := range all {
		items[i] = requestItem{req: r, showGroup: true}
	}
	return items
}

// listScreen shows the saved requests as a navigable folder tree.
type listScreen struct {
	lst list.Model
	// delegate is the same value lst was built with, kept here because
	// list.Model exposes no getter for it — needed by handleMouse to
	// compute row heights for click hit-testing.
	delegate list.DefaultDelegate

	// allReqs is every saved request across every group, refreshed from
	// disk by refresh(). curGroup is the folder currently being browsed
	// ("" is the top level). applyView derives the list's items from
	// these two, except while the list's own filter is active, when it
	// shows every request across every group instead (see searchItems).
	allReqs  []model.Request
	curGroup string
}

func newListScreen() (listScreen, error) {
	all, err := store.LoadRequests()
	if err != nil {
		return listScreen{}, err
	}
	delegate := newStyledDelegate()
	lst := list.New(childItems(all, ""), delegate, 0, 0)
	lst.Title = "Saved Requests"
	lst.SetShowHelp(true)
	// Turned off so click-to-select math (listContentTop, mouse.go) can
	// rely on an exact, fixed title-block height instead of the variable
	// height these would otherwise add.
	lst.SetShowStatusBar(false)
	lst.SetShowPagination(false)
	lst.AdditionalShortHelpKeys = listHelpKeys
	lst.AdditionalFullHelpKeys = listHelpKeys
	styleListHelp(&lst)
	return listScreen{lst: lst, delegate: delegate, allReqs: all}, nil
}

func (s *listScreen) setSize(w, h int) {
	s.lst.SetSize(w, h)
}

// refresh re-reads every saved request from disk and rebuilds the
// current view.
func (s *listScreen) refresh() error {
	all, err := store.LoadRequests()
	if err != nil {
		return err
	}
	s.allReqs = all
	s.applyView()
	return nil
}

// applyView rebuilds the list's items for the current navigation state.
// list.Model.SetItems triggers an async re-filter (via a returned tea.Cmd)
// whenever a filter is active, so callers that invoke this while filtering
// (see handleKey) must run the returned cmd through the Bubble Tea runtime
// for the new items to actually take effect.
func (s *listScreen) applyView() tea.Cmd {
	if s.filtered() {
		return s.lst.SetItems(searchItems(s.allReqs))
	}
	s.lst.SetItems(childItems(s.allReqs, s.curGroup))
	if s.curGroup == "" {
		s.lst.Title = "Saved Requests"
	} else {
		s.lst.Title = "Saved Requests / " + s.curGroup
	}
	return nil
}

// filtered reports whether the list's own "/" filter is active (being
// typed, or applied), as opposed to plain folder browsing.
func (s listScreen) filtered() bool {
	return s.lst.FilterState() != list.Unfiltered
}

// openFolder descends into the subfolder name (relative to curGroup).
func (s *listScreen) openFolder(name string) {
	if s.curGroup == "" {
		s.curGroup = name
	} else {
		s.curGroup = s.curGroup + "/" + name
	}
	s.lst.Select(0)
	s.applyView()
}

// goUp ascends to the parent of the folder currently being browsed. It
// reports whether there was a parent to ascend to (false at the top
// level), so callers can decide what a "no-op" go-up should mean.
func (s *listScreen) goUp() bool {
	if s.curGroup == "" {
		return false
	}
	if idx := strings.LastIndex(s.curGroup, "/"); idx >= 0 {
		s.curGroup = s.curGroup[:idx]
	} else {
		s.curGroup = ""
	}
	s.lst.Select(0)
	s.applyView()
	return true
}

// handleKey lets the underlying list.Model process msg (cursor movement,
// its own filter input, etc.), then reconciles the item set (folder
// browsing vs. cross-folder search) with any resulting filter-state
// change.
func (s *listScreen) handleKey(msg tea.Msg) tea.Cmd {
	before := s.lst.FilterState()
	var cmd tea.Cmd
	s.lst, cmd = s.lst.Update(msg)
	if s.lst.FilterState() != before {
		cmd = tea.Batch(cmd, s.applyView())
	}
	return cmd
}

func (s listScreen) selected() (model.Request, bool) {
	item, ok := s.lst.SelectedItem().(requestItem)
	if !ok {
		return model.Request{}, false
	}
	return item.req, true
}

// selectedFolder returns the currently highlighted folder's own path
// segment (not its full group path), if the current item is a folder.
func (s listScreen) selectedFolder() (string, bool) {
	item, ok := s.lst.SelectedItem().(folderItem)
	if !ok {
		return "", false
	}
	return item.name, true
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
