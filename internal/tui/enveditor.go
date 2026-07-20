package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/dotenv"
	"github.com/melvinsembrano/terman/internal/model"
)

// Section focus within the env editor's browse mode.
const (
	envSectionName = iota
	envSectionRows
)

// Focus within the row-edit modal.
const (
	envFocusKey = iota
	envFocusValue
)

// kvPair is one row in the environment's variable list.
type kvPair struct {
	key   string
	value string
}

// visiblePair pairs a kvPair with its original index in the full pairs
// slice. Used to map filtered-view selections back to real slice positions.
type visiblePair struct {
	idx  int
	pair kvPair
}

// envEditorScreen is the environment create/edit form: a name field plus a
// scrollable, filterable list of key/value variable pairs.
type envEditorScreen struct {
	prevName string // original name, "" for a new environment
	name     textinput.Model
	pairs    []kvPair
	section  int // envSectionName | envSectionRows
	selected int // selected index in the *visible* (filtered) set

	// vp renders the variable rows as a scrollable window.
	// rowOffset is the authoritative scroll position — it is applied to
	// vp.YOffset inside View() every render. This decouples scroll state
	// from rendering so that ensureSelectedVisible works even when View()
	// hasn't been called yet (e.g. in tests that only call Update).
	vp        viewport.Model
	rowOffset int

	// filtering is true while the user is typing in the filter bar.
	// filter holds the current query; its value persists until clearFilter.
	filtering bool
	filter    textinput.Model

	editing   bool // row-edit modal open
	editIdx   int  // index into pairs being edited; -1 = adding a new row
	keyInput  textinput.Model
	valInput  textinput.Model
	editFocus int // envFocusKey | envFocusValue

	importing bool // import-from-file modal open
	pathInput textinput.Model
	importErr string

	// sessionOnly marks an environment created via the list's "L" (load
	// session env) key. Saving it never touches disk — see appModel's
	// addSessionEnv.
	sessionOnly bool

	err string
}

func newEnvEditorScreen() envEditorScreen {
	name := textinput.New()
	name.Placeholder = "dev"
	name.CharLimit = 128

	keyIn := textinput.New()
	keyIn.Placeholder = "KEY"
	keyIn.CharLimit = 128

	valIn := textinput.New()
	valIn.Placeholder = "value"
	valIn.CharLimit = 2048

	pathIn := textinput.New()
	pathIn.Placeholder = ".env or .env.production"
	pathIn.CharLimit = 4096

	filterIn := textinput.New()
	filterIn.Placeholder = "type to filter…"
	filterIn.CharLimit = 256

	return envEditorScreen{
		name:      name,
		keyInput:  keyIn,
		valInput:  valIn,
		pathInput: pathIn,
		filter:    filterIn,
		editIdx:   -1,
		vp:        viewport.New(0, 0),
	}
}

func (s *envEditorScreen) setSize(w, h int) {
	fieldW := w - 2
	if fieldW < 10 {
		fieldW = 10
	}
	s.name.Width = fieldW
	s.keyInput.Width = fieldW / 2
	s.valInput.Width = fieldW / 2
	s.pathInput.Width = fieldW
	s.filter.Width = fieldW
	s.vp.Width = w

	// Fixed lines owned by this screen above the viewport:
	//   title + blank (2), name label (1), name field (1), blank (1),
	//   "Variables" label (1), filter bar (1) = 7
	// Fixed lines below the viewport:
	//   blank after viewport (1), hints bar (1) = 2
	// Total = 9. One filter-bar line is always budgeted even when the
	// filter bar is hidden, so the viewport height stays constant.
	vpH := h - 9
	if vpH < 1 {
		vpH = 1
	}
	s.vp.Height = vpH
}

// loadNew resets the form for creating a new environment.
func (s *envEditorScreen) loadNew() {
	w := s.name.Width
	vpW, vpH := s.vp.Width, s.vp.Height
	*s = newEnvEditorScreen()
	s.name.Width = w
	s.keyInput.Width = w / 2
	s.valInput.Width = w / 2
	s.pathInput.Width = w
	s.filter.Width = w
	s.vp.Width = vpW
	s.vp.Height = vpH
	s.setSectionFocus(envSectionName)
}

// loadEnvironment populates the form from an existing saved environment.
func (s *envEditorScreen) loadEnvironment(e model.Environment) {
	s.loadNew()
	s.prevName = e.Name
	s.name.SetValue(e.Name)

	keys := make([]string, 0, len(e.Vars))
	for k := range e.Vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s.pairs = make([]kvPair, 0, len(keys))
	for _, k := range keys {
		s.pairs = append(s.pairs, kvPair{key: k, value: e.Vars[k]})
	}
}

// toEnvironment builds a model.Environment from the current form values.
// The filter is not applied — all variables (including those hidden by an
// active filter) are included in the saved result.
func (s envEditorScreen) toEnvironment() model.Environment {
	vars := map[string]string{}
	for _, p := range s.pairs {
		k := strings.TrimSpace(p.key)
		if k == "" {
			continue
		}
		vars[k] = p.value
	}
	if len(vars) == 0 {
		vars = nil
	}
	return model.Environment{
		Name: strings.TrimSpace(s.name.Value()),
		Vars: vars,
	}
}

// visiblePairs returns the subset of pairs that match the current filter
// query (case-insensitive key or value substring match). When the filter
// is empty, all pairs are returned. Each entry preserves the original
// index into s.pairs so callers can map back for edits/deletes.
func (s *envEditorScreen) visiblePairs() []visiblePair {
	q := strings.ToLower(s.filter.Value())
	out := make([]visiblePair, 0, len(s.pairs))
	for i, p := range s.pairs {
		if q == "" ||
			strings.Contains(strings.ToLower(p.key), q) ||
			strings.Contains(strings.ToLower(p.value), q) {
			out = append(out, visiblePair{idx: i, pair: p})
		}
	}
	return out
}

// ensureSelectedVisible updates rowOffset so the selected row is within
// the viewport window. No-op when the viewport hasn't been sized yet.
func (s *envEditorScreen) ensureSelectedVisible() {
	if s.vp.Height <= 0 {
		return
	}
	if s.selected < s.rowOffset {
		s.rowOffset = s.selected
	} else if s.selected >= s.rowOffset+s.vp.Height {
		s.rowOffset = s.selected - s.vp.Height + 1
	}
}

// clearFilter resets the filter to empty and exits filter mode, restoring
// the full variable list and scrolling back to the top.
func (s *envEditorScreen) clearFilter() {
	s.filter.SetValue("")
	s.filtering = false
	s.filter.Blur()
	s.selected = 0
	s.rowOffset = 0
}

// startAddRow opens the row-edit modal to add a new variable.
func (s *envEditorScreen) startAddRow() {
	s.editing = true
	s.editIdx = -1
	s.keyInput.SetValue("")
	s.valInput.SetValue("")
	s.editFocus = envFocusKey
	s.keyInput.Focus()
	s.valInput.Blur()
}

// startEditRow opens the row-edit modal on the currently selected row,
// resolved through the active filter's visible set.
func (s *envEditorScreen) startEditRow() {
	visible := s.visiblePairs()
	if s.selected < 0 || s.selected >= len(visible) {
		return
	}
	vp := visible[s.selected]
	s.editing = true
	s.editIdx = vp.idx
	s.keyInput.SetValue(vp.pair.key)
	s.valInput.SetValue(vp.pair.value)
	s.editFocus = envFocusKey
	s.keyInput.Focus()
	s.valInput.Blur()
}

// commitRow saves the row-edit modal's key/value into pairs, appending or
// replacing depending on editIdx.
func (s *envEditorScreen) commitRow() {
	p := kvPair{key: strings.TrimSpace(s.keyInput.Value()), value: s.valInput.Value()}
	if s.editIdx >= 0 && s.editIdx < len(s.pairs) {
		s.pairs[s.editIdx] = p
		// Keep selection pointing at the same visible row after an edit.
	} else {
		s.pairs = append(s.pairs, p)
		// Move selection to the newly appended row in the visible set.
		visible := s.visiblePairs()
		for i, vp := range visible {
			if vp.idx == len(s.pairs)-1 {
				s.selected = i
				break
			}
		}
		s.ensureSelectedVisible()
	}
	s.closeRowModal()
}

func (s *envEditorScreen) closeRowModal() {
	s.editing = false
	s.keyInput.Blur()
	s.valInput.Blur()
}

// deleteSelectedRow removes the currently selected row, resolved through
// the active filter's visible set.
func (s *envEditorScreen) deleteSelectedRow() {
	visible := s.visiblePairs()
	if s.selected < 0 || s.selected >= len(visible) {
		return
	}
	idx := visible[s.selected].idx
	s.pairs = append(s.pairs[:idx], s.pairs[idx+1:]...)
	// Recompute visible after deletion and clamp selection.
	newVisible := s.visiblePairs()
	if s.selected >= len(newVisible) {
		s.selected = len(newVisible) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}
	s.ensureSelectedVisible()
}

// startImport opens the "import from .env file" modal.
func (s *envEditorScreen) startImport() {
	s.importing = true
	s.importErr = ""
	s.pathInput.SetValue("")
	s.pathInput.Focus()
}

func (s *envEditorScreen) closeImportModal() {
	s.importing = false
	s.pathInput.Blur()
}

// commitImport parses the file at the modal's path and upserts its
// variables into pairs: existing keys are updated in place, new keys are
// appended (sorted for deterministic ordering), mirroring the CLI's
// "env import" merge semantics.
func (s *envEditorScreen) commitImport() {
	path := strings.TrimSpace(s.pathInput.Value())
	if path == "" {
		s.importErr = "path is required"
		return
	}
	parsed, err := dotenv.ParseFile(path)
	if err != nil {
		s.importErr = err.Error()
		return
	}

	index := map[string]int{}
	for i, p := range s.pairs {
		index[p.key] = i
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := parsed[k]
		if i, ok := index[k]; ok {
			s.pairs[i].value = v
		} else {
			s.pairs = append(s.pairs, kvPair{key: k, value: v})
			index[k] = len(s.pairs) - 1
		}
	}
	s.closeImportModal()
}

func (s *envEditorScreen) setSectionFocus(section int) {
	s.section = section
	if section == envSectionName {
		s.name.Focus()
	} else {
		s.name.Blur()
	}
}

// envRowsContentTop is the number of terminal rows above the start of the
// variable-row viewport in envEditorScreen.View(). It equals the app
// header rows (headerLines-1, because lipgloss.Height over-counts by 1)
// plus the lines the screen itself emits before the filter bar:
//
//	title (1) + blank (1) + name-label (1) + name-field (1) + blank (1)
//	+ "Variables" label (1) = 6
//
// The filter bar itself is one additional line when shown; callers that
// need click math below the filter bar add 1 to this constant.
//
// Keep in sync with View() if the layout above the rows changes.
const envRowsContentTop = headerLines - 1 + 6

// handleMouse applies click-to-select to the variable rows via the
// viewport: a left click sets the selection to the row at that terminal
// row (accounting for the viewport's scroll offset and whether the filter
// bar is currently visible). It never opens the row-edit modal — that
// still requires enter. Ignored while a modal is open or when the click
// misses every visible row. Reports whether it consumed the event.
func (s *envEditorScreen) handleMouse(msg tea.MouseEvent) bool {
	if s.editing || s.importing {
		return false
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return false
	}
	// Viewport content starts one row lower when the filter bar is shown.
	viewportTop := envRowsContentTop
	if s.filtering || s.filter.Value() != "" {
		viewportTop++
	}
	idx := msg.Y - viewportTop + s.rowOffset
	visible := s.visiblePairs()
	if idx < 0 || idx >= len(visible) {
		return false
	}
	s.section = envSectionRows
	s.selected = idx
	return true
}

func (s envEditorScreen) Update(msg tea.Msg) (envEditorScreen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if s.editing {
			switch key.String() {
			case "esc":
				s.closeRowModal()
				return s, nil
			case "tab", "shift+tab":
				if s.editFocus == envFocusKey {
					s.editFocus = envFocusValue
					s.keyInput.Blur()
					s.valInput.Focus()
				} else {
					s.editFocus = envFocusKey
					s.valInput.Blur()
					s.keyInput.Focus()
				}
				return s, nil
			case "enter":
				s.commitRow()
				return s, nil
			}
			var cmd tea.Cmd
			if s.editFocus == envFocusKey {
				s.keyInput, cmd = s.keyInput.Update(msg)
			} else {
				s.valInput, cmd = s.valInput.Update(msg)
			}
			return s, cmd
		}

		if s.importing {
			switch key.String() {
			case "esc":
				s.closeImportModal()
				return s, nil
			case "enter":
				s.commitImport()
				return s, nil
			}
			var cmd tea.Cmd
			s.pathInput, cmd = s.pathInput.Update(msg)
			return s, cmd
		}

		// Filter mode: typing updates the filter query; navigation still
		// works so the user can move without leaving the filter bar.
		if s.filtering {
			switch key.String() {
			case "esc":
				s.clearFilter()
				return s, nil
			case "enter", "tab":
				// Commit the filter and return to normal navigation.
				s.filtering = false
				s.filter.Blur()
				return s, nil
			case "up":
				if s.selected > 0 {
					s.selected--
					s.ensureSelectedVisible()
				}
				return s, nil
			case "down":
				visible := s.visiblePairs()
				if s.selected < len(visible)-1 {
					s.selected++
					s.ensureSelectedVisible()
				}
				return s, nil
			}
			var cmd tea.Cmd
			s.filter, cmd = s.filter.Update(msg)
			// After the filter text changes, clamp selection and reset scroll.
			visible := s.visiblePairs()
			if s.selected >= len(visible) {
				s.selected = len(visible) - 1
			}
			if s.selected < 0 {
				s.selected = 0
			}
			s.vp.GotoTop()
			s.ensureSelectedVisible()
			return s, cmd
		}

		switch key.String() {
		case "tab", "shift+tab":
			if s.section == envSectionName {
				s.setSectionFocus(envSectionRows)
			} else {
				s.setSectionFocus(envSectionName)
			}
			return s, nil
		case "up":
			if s.section == envSectionRows && s.selected > 0 {
				s.selected--
				s.ensureSelectedVisible()
				return s, nil
			}
		case "down":
			visible := s.visiblePairs()
			if s.section == envSectionRows && s.selected < len(visible)-1 {
				s.selected++
				s.ensureSelectedVisible()
				return s, nil
			}
		case "f", "/":
			if s.section == envSectionRows {
				s.filtering = true
				s.filter.Focus()
				return s, nil
			}
		case "a":
			if s.section == envSectionRows {
				s.startAddRow()
				return s, nil
			}
		case "i":
			if s.section == envSectionRows {
				s.startImport()
				return s, nil
			}
		case "enter":
			if s.section == envSectionRows {
				s.startEditRow()
				return s, nil
			}
		case "d":
			if s.section == envSectionRows {
				s.deleteSelectedRow()
				return s, nil
			}
		}
	}

	if s.section == envSectionName {
		var cmd tea.Cmd
		s.name, cmd = s.name.Update(msg)
		return s, cmd
	}
	return s, nil
}

func (s envEditorScreen) View() string {
	var b strings.Builder

	title := "New Environment"
	if s.prevName != "" {
		title = "Edit Environment"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	nameLabel := labelStyle.Render("Name")
	if s.sessionOnly {
		nameLabel += "  " + subtleStyle.Render("(session — not saved to disk)")
	}
	b.WriteString(nameLabel + "\n" + s.name.View() + "\n\n")

	b.WriteString(labelStyle.Render("Variables") + "\n")

	// Filter bar — shown whenever a filter is active or has a value.
	if s.filtering || s.filter.Value() != "" {
		prefix := blurredStyle.Render("  / ")
		if s.filtering {
			prefix = focusedStyle.Render("  / ")
		}
		b.WriteString(prefix + s.filter.View() + "\n")
	}

	// Build row content for the viewport.
	var rowsB strings.Builder
	visible := s.visiblePairs()
	switch {
	case len(s.pairs) == 0:
		rowsB.WriteString(blurredStyle.Render("(none — press 'a' to add one)") + "\n")
	case len(visible) == 0:
		rowsB.WriteString(blurredStyle.Render("(no matches — press esc to clear filter)") + "\n")
	default:
		for i, vp := range visible {
			line := fmt.Sprintf("%-20s = %s", vp.pair.key, vp.pair.value)
			if s.section == envSectionRows && i == s.selected && !s.editing {
				line = focusedStyle.Render("> " + line)
			} else {
				line = "  " + line
			}
			rowsB.WriteString(line + "\n")
		}
	}
	// Drive the viewport with the freshly built content and render it.
	// SetContent must come before SetYOffset — SetContent resets YOffset to
	// 0, so the offset must be (re-)applied afterwards.
	s.vp.SetContent(rowsB.String())
	s.vp.SetYOffset(s.rowOffset)
	b.WriteString(s.vp.View())
	b.WriteString("\n")

	if s.editing {
		b.WriteString(labelStyle.Render("Edit variable") + "\n")
		b.WriteString("Key:   " + s.keyInput.View() + "\n")
		b.WriteString("Value: " + s.valInput.View() + "\n\n")
		b.WriteString(renderHints(
			keyHint{"tab", "switch field"},
			keyHint{"enter", "save"},
			keyHint{"esc", "cancel"},
		))
		return b.String()
	}

	if s.importing {
		b.WriteString(labelStyle.Render("Import from .env file") + "\n")
		b.WriteString("Path: " + s.pathInput.View() + "\n\n")
		if s.importErr != "" {
			b.WriteString(errorStyle.Render("error: "+s.importErr) + "\n\n")
		}
		b.WriteString(renderHints(
			keyHint{"enter", "import"},
			keyHint{"esc", "cancel"},
		))
		return b.String()
	}

	if s.err != "" {
		b.WriteString(errorStyle.Render("error: "+s.err) + "\n\n")
	}

	if s.filtering {
		b.WriteString(renderHints(
			keyHint{"↑/↓", "select row"},
			keyHint{"enter/tab", "done"},
			keyHint{"esc", "clear filter"},
		))
	} else if s.filter.Value() != "" {
		b.WriteString(renderHints(
			keyHint{"tab", "move field"},
			keyHint{"↑/↓", "select row"},
			keyHint{"a", "add"},
			keyHint{"enter", "edit row"},
			keyHint{"d", "delete row"},
			keyHint{"f//", "filter: " + s.filter.Value()},
			keyHint{"ctrl+s", "save"},
			keyHint{"esc", "cancel"},
		))
	} else {
		b.WriteString(renderHints(
			keyHint{"tab", "move field"},
			keyHint{"↑/↓", "select row"},
			keyHint{"a", "add"},
			keyHint{"i", "import"},
			keyHint{"enter", "edit row"},
			keyHint{"d", "delete row"},
			keyHint{"f//", "filter"},
			keyHint{"ctrl+s", "save"},
			keyHint{"esc", "cancel"},
		))
	}
	return b.String()
}
