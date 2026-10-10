package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"tsv/internal/svn"
)

func TestStalePreviewAndRefresh(t *testing.T) {
	m := New(svn.Client{})
	m.loading = false
	m.commits = []svn.Commit{{Revision: 2}, {Revision: 1}}
	m.exhausted = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if cmd != nil || m.selected != 1 || m.detail {
		t.Fatal("movement should select without opening detail")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	defer m.cancel()
	if cmd == nil || !m.detail || !m.focusDiff {
		t.Fatal("Enter did not open detail")
	}
	next, _ = m.Update(preview{id: m.request - 1, text: "stale"})
	m = next.(Model)
	if strings.Contains(m.view.View(), "stale") {
		t.Fatal("stale result accepted")
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if !m.loading || cmd == nil {
		t.Fatal("refresh not started")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.(Model).selected != 1 {
		t.Fatal("selection changed during refresh")
	}
}

func TestUnifiedListAndClose(t *testing.T) {
	m := New(svn.Client{})
	m.loading = false
	m.commits = []svn.Commit{{Revision: 7, Author: "tester", Message: "历史提交"}}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m = next.(Model)
	text := m.View()
	if !strings.Contains(text, "Not Committed Yet") || !strings.Contains(text, "Unstaged changes") || !strings.Contains(text, "[r7]") {
		t.Fatalf("missing unified rows: %s", text)
	}
	m.detail, m.focusDiff = true, true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if next.(Model).detail || cmd != nil {
		t.Fatal("q should close detail first")
	}
}

func TestSafeText(t *testing.T) {
	if got := safeText("\x1b[31mhello\x1b[0m\x07\tworld"); got != "hello world" {
		t.Fatalf("unsafe text: %q", got)
	}
}

func TestHeadAndBaseNavigation(t *testing.T) {
	m := New(svn.Client{})
	m.loading = false
	m.info = svn.Info{Revision: "8", BaseCommit: 7, MinRevision: 8, MaxRevision: 8}
	m.commits = []svn.Commit{{Revision: 10}, {Revision: 7}}
	m.exhausted = true
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	m = next.(Model)
	if text := m.View(); !strings.Contains(text, "[HEAD]") || !strings.Contains(text, "[ROOT BASE]") {
		t.Fatalf("missing position markers: %s", text)
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = next.(Model)
	if cmd != nil || m.selected != 2 {
		t.Fatal("BASE navigation failed")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if next.(Model).selected != 1 {
		t.Fatal("HEAD navigation failed")
	}
	m.exhausted = false
	m.commits = m.commits[:1]
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = next.(Model)
	if cmd == nil || !m.locatingBase {
		t.Fatal("BASE should load older history")
	}
	next, _ = m.Update(moreLoaded{generation: m.generation, commits: []svn.Commit{{Revision: 7}}})
	if next.(Model).selected != 2 || next.(Model).locatingBase {
		t.Fatal("BASE not selected after loading")
	}
}

func TestHistoryVisibleWhilePositionLoads(t *testing.T) {
	m := New(svn.Client{})
	next, cmd := m.Update(loaded{client: svn.Client{}, info: svn.Info{Root: "working-copy", Revision: "8"}, commits: []svn.Commit{{Revision: 10}}})
	m = next.(Model)
	if m.loading || m.positionLoading || !m.paging || cmd == nil {
		t.Fatal("local position should be available without scanning before history loads")
	}
	m.positionLoading = true
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	m = next.(Model)
	if !m.locatingBase {
		t.Fatal("BASE request should wait for background scan")
	}
	next, _ = m.Update(positionLoaded{generation: m.generation, info: svn.Info{BaseCommit: 10}})
	m = next.(Model)
	if m.positionLoading || m.locatingBase || m.selected != 1 {
		t.Fatal("pending BASE request was not completed")
	}
}
