package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/melvinsembrano/terman/internal/httpx"
	"github.com/melvinsembrano/terman/internal/model"
)

// runResultMsg carries the outcome of an asynchronously executed request
// back into the Bubble Tea update loop.
type runResultMsg struct {
	name string
	resp httpx.Response
	err  error
}

// runRequestCmd executes req in the background and reports the result as
// a runResultMsg.
func runRequestCmd(req model.Request, vars map[string]string) tea.Cmd {
	return func() tea.Msg {
		resp, err := httpx.Do(req, vars)
		return runResultMsg{name: req.Name, resp: resp, err: err}
	}
}

// responseScreen shows the outcome of running a request.
type responseScreen struct {
	vp    viewport.Model
	title string
}

func newResponseScreen() responseScreen {
	return responseScreen{vp: viewport.New(0, 0)}
}

func (s *responseScreen) setSize(w, h int) {
	s.vp.Width = w
	s.vp.Height = h
}

func (s *responseScreen) showRunning(name string) {
	s.title = "Running " + name + " ..."
	s.vp.SetContent("")
	s.vp.GotoTop()
}

func (s *responseScreen) showResult(name string, resp httpx.Response) {
	s.title = name
	s.vp.SetContent(formatResponse(resp))
	s.vp.GotoTop()
}

// formatResponse renders resp as the viewport body: a status/duration line,
// a blank line, the response headers (if any), and the body.
func formatResponse(resp httpx.Response) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  (%s)\n\n", resp.Status, resp.Duration.Round(1_000_000))
	if h := resp.HeadersString(); h != "" {
		b.WriteString(h)
		b.WriteString("\n")
	}
	b.WriteString(resp.Body)
	return b.String()
}

func (s *responseScreen) showError(name string, err error) {
	s.title = name
	s.vp.SetContent(errorStyle.Render("error: " + err.Error()))
	s.vp.GotoTop()
}

func (s responseScreen) Update(msg tea.Msg) (responseScreen, tea.Cmd) {
	var cmd tea.Cmd
	s.vp, cmd = s.vp.Update(msg)
	return s, cmd
}

func (s responseScreen) View() string {
	return titleStyle.Render(s.title) + "\n\n" + s.vp.View() + "\n" +
		helpStyle.Render("↑/↓ pgup/pgdn scroll • esc back")
}
