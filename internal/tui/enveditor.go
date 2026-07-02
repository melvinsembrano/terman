package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

// envEditorScreen is the environment create/edit form: a name field plus a
// row-based list of key/value variable pairs.
type envEditorScreen struct {
	prevName string // original name, "" for a new environment
	name     textinput.Model
	pairs    []kvPair
	section  int // envSectionName | envSectionRows
	selected int // selected row index when section == envSectionRows

	editing   bool // row-edit modal open
	editIdx   int  // index into pairs being edited; -1 = adding a new row
	keyInput  textinput.Model
	valInput  textinput.Model
	editFocus int // envFocusKey | envFocusValue

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

	return envEditorScreen{name: name, keyInput: keyIn, valInput: valIn, editIdx: -1}
}

func (s *envEditorScreen) setSize(w, h int) {
	fieldW := w - 2
	if fieldW < 10 {
		fieldW = 10
	}
	s.name.Width = fieldW
	s.keyInput.Width = fieldW / 2
	s.valInput.Width = fieldW / 2
}

// loadNew resets the form for creating a new environment.
func (s *envEditorScreen) loadNew() {
	w := s.name.Width
	*s = newEnvEditorScreen()
	s.name.Width = w
	s.keyInput.Width = w / 2
	s.valInput.Width = w / 2
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

// startEditRow opens the row-edit modal on the currently selected row.
func (s *envEditorScreen) startEditRow() {
	if s.selected < 0 || s.selected >= len(s.pairs) {
		return
	}
	p := s.pairs[s.selected]
	s.editing = true
	s.editIdx = s.selected
	s.keyInput.SetValue(p.key)
	s.valInput.SetValue(p.value)
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
	} else {
		s.pairs = append(s.pairs, p)
		s.selected = len(s.pairs) - 1
	}
	s.closeRowModal()
}

func (s *envEditorScreen) closeRowModal() {
	s.editing = false
	s.keyInput.Blur()
	s.valInput.Blur()
}

// deleteSelectedRow removes the currently selected row, if any.
func (s *envEditorScreen) deleteSelectedRow() {
	if s.selected < 0 || s.selected >= len(s.pairs) {
		return
	}
	s.pairs = append(s.pairs[:s.selected], s.pairs[s.selected+1:]...)
	if s.selected >= len(s.pairs) {
		s.selected = len(s.pairs) - 1
	}
}

func (s *envEditorScreen) setSectionFocus(section int) {
	s.section = section
	if section == envSectionName {
		s.name.Focus()
	} else {
		s.name.Blur()
	}
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
				return s, nil
			}
		case "down":
			if s.section == envSectionRows && s.selected < len(s.pairs)-1 {
				s.selected++
				return s, nil
			}
		case "a":
			if s.section == envSectionRows {
				s.startAddRow()
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

	b.WriteString(labelStyle.Render("Name") + "\n" + s.name.View() + "\n\n")
	b.WriteString(labelStyle.Render("Variables") + "\n")
	if len(s.pairs) == 0 {
		b.WriteString(blurredStyle.Render("(none — press 'a' to add one)") + "\n")
	}
	for i, p := range s.pairs {
		line := fmt.Sprintf("%-20s = %s", p.key, p.value)
		if s.section == envSectionRows && i == s.selected && !s.editing {
			line = focusedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	if s.editing {
		b.WriteString(labelStyle.Render("Edit variable") + "\n")
		b.WriteString("Key:   " + s.keyInput.View() + "\n")
		b.WriteString("Value: " + s.valInput.View() + "\n\n")
		b.WriteString(helpStyle.Render("tab switch field • enter save • esc cancel"))
		return b.String()
	}

	if s.err != "" {
		b.WriteString(errorStyle.Render("error: "+s.err) + "\n\n")
	}
	b.WriteString(helpStyle.Render("tab move field • ↑/↓ select row • a add • enter edit row • d delete row • ctrl+s save • esc cancel"))
	return b.String()
}
