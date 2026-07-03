package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/model"
)

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func methodIndex(m string) int {
	for i, meth := range methods {
		if strings.EqualFold(meth, m) {
			return i
		}
	}
	return 0
}

// Field focus order within the editor.
const (
	focusMethod = iota
	focusName
	focusGroup
	focusURL
	focusHeaders
	focusBody
	focusCount
)

// editorScreen is the request create/edit form.
type editorScreen struct {
	prevName  string // original name, "" for a new request
	prevGroup string // original group, only meaningful when prevName != ""
	methodIdx int
	name      textinput.Model
	group     textinput.Model
	url       textinput.Model
	headers   textarea.Model
	body      textarea.Model
	focus     int
	err       string
}

func newEditorScreen() editorScreen {
	name := textinput.New()
	name.Placeholder = "My API request"
	name.CharLimit = 128

	group := textinput.New()
	group.Placeholder = "auth/oauth (optional)"
	group.CharLimit = 256

	url := textinput.New()
	url.Placeholder = "https://api.example.com/{{path}}"
	url.CharLimit = 2048

	headers := textarea.New()
	headers.Placeholder = "Header-Name: value\nOne per line"
	headers.ShowLineNumbers = false

	body := textarea.New()
	body.Placeholder = "Request body (may use {{vars}})"
	body.ShowLineNumbers = false

	return editorScreen{name: name, group: group, url: url, headers: headers, body: body}
}

func (s *editorScreen) setSize(w, h int) {
	fieldW := w - 2
	if fieldW < 10 {
		fieldW = 10
	}
	s.name.Width = fieldW
	s.group.Width = fieldW
	s.url.Width = fieldW
	s.headers.SetWidth(fieldW)
	s.headers.SetHeight(4)
	s.body.SetWidth(fieldW)
	s.body.SetHeight(6)
}

// loadNew resets the form for creating a new request, defaulting its
// folder to defaultGroup (typically the folder currently being browsed in
// the request list).
func (s *editorScreen) loadNew(defaultGroup string) {
	w, hw, hh, bw, bh := s.name.Width, s.headers.Width(), s.headers.Height(), s.body.Width(), s.body.Height()
	*s = newEditorScreen()
	s.name.Width, s.group.Width, s.url.Width = w, w, w
	s.headers.SetWidth(hw)
	s.headers.SetHeight(hh)
	s.body.SetWidth(bw)
	s.body.SetHeight(bh)
	s.group.SetValue(defaultGroup)
	s.setFocus(focusName)
}

// loadRequest populates the form from an existing saved request.
func (s *editorScreen) loadRequest(r model.Request) {
	s.loadNew("")
	s.prevName = r.Name
	s.prevGroup = r.Group
	s.methodIdx = methodIndex(r.Method)
	s.name.SetValue(r.Name)
	s.group.SetValue(r.Group)
	s.url.SetValue(r.URL)

	lines := make([]string, 0, len(r.Headers))
	for k, v := range r.Headers {
		lines = append(lines, fmt.Sprintf("%s: %s", k, v))
	}
	sort.Strings(lines)
	s.headers.SetValue(strings.Join(lines, "\n"))
	s.body.SetValue(r.Body)
	s.setFocus(focusName)
}

func (s *editorScreen) setFocus(f int) {
	s.focus = ((f % focusCount) + focusCount) % focusCount
	s.name.Blur()
	s.group.Blur()
	s.url.Blur()
	s.headers.Blur()
	s.body.Blur()
	switch s.focus {
	case focusName:
		s.name.Focus()
	case focusGroup:
		s.group.Focus()
	case focusURL:
		s.url.Focus()
	case focusHeaders:
		s.headers.Focus()
	case focusBody:
		s.body.Focus()
	}
}

// normalizeGroupInput cleans up a raw "Folder" field value into a
// "/"-separated group path: no leading/trailing slashes, no empty
// segments. The store layer's own per-segment slugging is what actually
// determines the on-disk folder names — a saved request's Group then
// reflects that (slugged, lower-cased) path once it's next loaded.
func normalizeGroupInput(v string) string {
	v = strings.ReplaceAll(strings.TrimSpace(v), "\\", "/")
	var kept []string
	for _, part := range strings.Split(v, "/") {
		if part = strings.TrimSpace(part); part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "/")
}

// toRequest builds a model.Request from the current form values.
func (s editorScreen) toRequest() model.Request {
	headers := map[string]string{}
	for _, line := range strings.Split(s.headers.Value(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if len(headers) == 0 {
		headers = nil
	}
	return model.Request{
		Name:    strings.TrimSpace(s.name.Value()),
		Group:   normalizeGroupInput(s.group.Value()),
		Method:  methods[s.methodIdx],
		URL:     strings.TrimSpace(s.url.Value()),
		Headers: headers,
		Body:    s.body.Value(),
	}
}

func (s editorScreen) Update(msg tea.Msg) (editorScreen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			s.setFocus(s.focus + 1)
			return s, nil
		case "shift+tab":
			s.setFocus(s.focus - 1)
			return s, nil
		case "left":
			if s.focus == focusMethod {
				s.methodIdx = (s.methodIdx - 1 + len(methods)) % len(methods)
				return s, nil
			}
		case "right":
			if s.focus == focusMethod {
				s.methodIdx = (s.methodIdx + 1) % len(methods)
				return s, nil
			}
		}
	}

	var cmd tea.Cmd
	switch s.focus {
	case focusName:
		s.name, cmd = s.name.Update(msg)
	case focusGroup:
		s.group, cmd = s.group.Update(msg)
	case focusURL:
		s.url, cmd = s.url.Update(msg)
	case focusHeaders:
		s.headers, cmd = s.headers.Update(msg)
	case focusBody:
		s.body, cmd = s.body.Update(msg)
	}
	return s, cmd
}

func (s editorScreen) View() string {
	var b strings.Builder

	title := "New Request"
	if s.prevName != "" {
		title = "Edit Request"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	method := methods[s.methodIdx]
	if s.focus == focusMethod {
		b.WriteString(labelStyle.Render("Method: ") + focusedStyle.Render("◀ "+method+" ▶") + "\n\n")
	} else {
		b.WriteString(labelStyle.Render("Method: ") + blurredStyle.Render(method) + "\n\n")
	}

	b.WriteString(labelStyle.Render("Name") + "\n" + s.name.View() + "\n\n")
	b.WriteString(labelStyle.Render("Folder (optional, e.g. auth/oauth)") + "\n" + s.group.View() + "\n\n")
	b.WriteString(labelStyle.Render("URL") + "\n" + s.url.View() + "\n\n")
	b.WriteString(labelStyle.Render("Headers (Key: Value, one per line)") + "\n" + s.headers.View() + "\n\n")
	b.WriteString(labelStyle.Render("Body") + "\n" + s.body.View() + "\n\n")

	if s.err != "" {
		b.WriteString(errorStyle.Render("error: "+s.err) + "\n\n")
	}
	b.WriteString(renderHints(
		keyHint{"tab/shift+tab", "move field"},
		keyHint{"←/→", "change method"},
		keyHint{"ctrl+s", "save"},
		keyHint{"esc", "cancel"},
	))
	return b.String()
}
