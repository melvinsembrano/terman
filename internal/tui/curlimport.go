package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/curl"
	"github.com/melvinsembrano/terman/internal/model"
)

// Field focus within the curl import screen.
const (
	curlFocusName = iota
	curlFocusCmd
)

// curlImportScreen is a small capture step: a name plus a pasted curl
// command. It never saves anything itself — on success it hands the
// parsed model.Request off to the regular request editor for review, the
// same two-step shape the env list's "L" (session env) flow uses.
type curlImportScreen struct {
	name  textinput.Model
	cmd   textarea.Model
	focus int
	err   string
}

func newCurlImportScreen() curlImportScreen {
	name := textinput.New()
	name.Placeholder = "Get Users"
	name.CharLimit = 128

	cmd := textarea.New()
	cmd.Placeholder = "curl 'https://api.example.com/users' -H 'Accept: application/json'"
	cmd.ShowLineNumbers = false

	return curlImportScreen{name: name, cmd: cmd}
}

func (s *curlImportScreen) setSize(w, h int) {
	fieldW := w - 2
	if fieldW < 10 {
		fieldW = 10
	}
	s.name.Width = fieldW
	s.cmd.SetWidth(fieldW)
	s.cmd.SetHeight(10)
}

// loadNew resets the form for a fresh import.
func (s *curlImportScreen) loadNew() {
	w, cw, ch := s.name.Width, s.cmd.Width(), s.cmd.Height()
	*s = newCurlImportScreen()
	s.name.Width = w
	s.cmd.SetWidth(cw)
	s.cmd.SetHeight(ch)
	s.setFocus(curlFocusName)
}

func (s *curlImportScreen) setFocus(f int) {
	s.focus = f
	if f == curlFocusName {
		s.name.Focus()
		s.cmd.Blur()
	} else {
		s.name.Blur()
		s.cmd.Focus()
	}
}

// parse builds a model.Request from the current form values, naming it
// from the Name field. The curl command's own method-list restriction (the
// editor's method field only cycles GET/POST/PUT/PATCH/DELETE) means an
// unusual custom method parsed here may get coerced to GET once it reaches
// the editor — a pre-existing limitation of that screen, not this one.
func (s curlImportScreen) parse() (model.Request, error) {
	name := strings.TrimSpace(s.name.Value())
	if name == "" {
		return model.Request{}, fmt.Errorf("name is required")
	}
	req, err := curl.Parse(s.cmd.Value())
	if err != nil {
		return model.Request{}, err
	}
	req.Name = name
	return req, nil
}

func (s curlImportScreen) Update(msg tea.Msg) (curlImportScreen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", "shift+tab":
			if s.focus == curlFocusName {
				s.setFocus(curlFocusCmd)
			} else {
				s.setFocus(curlFocusName)
			}
			return s, nil
		}
	}

	var cmd tea.Cmd
	if s.focus == curlFocusName {
		s.name, cmd = s.name.Update(msg)
	} else {
		s.cmd, cmd = s.cmd.Update(msg)
	}
	return s, cmd
}

func (s curlImportScreen) View() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Name") + "\n" + s.name.View() + "\n\n")
	b.WriteString(labelStyle.Render("curl command (paste below)") + "\n" + s.cmd.View() + "\n\n")
	if s.err != "" {
		b.WriteString(errorStyle.Render("error: "+s.err) + "\n\n")
	}
	b.WriteString(helpStyle.Render("tab move field • ctrl+s parse & continue • esc cancel"))
	return b.String()
}
