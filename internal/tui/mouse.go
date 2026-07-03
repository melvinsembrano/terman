package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// listContentTop is the fixed number of lines before a list.Model's items
// begin, for the two list.Model-based screens in this app: terman's own
// header (headerLines) plus the list's own title block, which is always
// exactly 2 lines (a 1-line Title plus TitleBar's default bottom padding
// line) for our short, non-wrapping titles. This depends on
// bubbles@v0.18.0's default TitleBar styling and on the status bar and
// pagination indicator being turned off (see newListScreen/
// newEnvListScreen, and the note in AGENTS.md) — re-verify if that
// dependency version ever changes.
const listContentTop = headerLines + 2

// listMouseEvent applies wheel-scroll and click-to-select to lst, built
// with delegate (list.Model exposes no getter for the delegate it was
// constructed with, so callers keep their own copy and pass it in). It
// reports whether it consumed the event; a false return lets the caller
// fall through to lst.Update for anything else (e.g. right-click, plain
// motion).
func listMouseEvent(msg tea.MouseEvent, lst *list.Model, delegate list.DefaultDelegate) bool {
	if msg.Action != tea.MouseActionPress {
		return false
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		lst.CursorUp()
		return true
	case tea.MouseButtonWheelDown:
		lst.CursorDown()
		return true
	case tea.MouseButtonLeft:
		if idx, ok := listClickedIndex(msg, *lst, delegate); ok {
			lst.Select(idx)
		}
		return true
	}
	return false
}

// listClickedIndex maps a left click's absolute terminal Y to an item
// index in lst. ok is false if the click landed outside the visible items
// (on the title, the help footer, or blank filler space on a
// partially-filled last page) — callers should treat that as a no-op,
// never a selection.
func listClickedIndex(msg tea.MouseEvent, lst list.Model, delegate list.DefaultDelegate) (int, bool) {
	rel := msg.Y - listContentTop
	if rel < 0 {
		return 0, false
	}
	stride := delegate.Height() + delegate.Spacing()
	if stride <= 0 {
		return 0, false
	}
	idxOnPage := rel / stride

	start, end := lst.Paginator.GetSliceBounds(len(lst.VisibleItems()))
	abs := start + idxOnPage
	if abs < start || abs >= end {
		return 0, false
	}
	return abs, true
}
