package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/wangzi5151/hubtop/internal/github"
)

func testModel() *model {
	m := newModel(nil)
	m.repos = []gh.Repo{
		{Name: "mcpscope", FullName: "me/mcpscope", Stars: 5},
		{Name: "nethole-tester", FullName: "me/nethole-tester", Stars: 20},
		{Name: "hubtop", FullName: "me/hubtop", Stars: 12},
	}
	m.applyFilter()
	return m
}

func TestApplySort(t *testing.T) {
	m := testModel()
	m.sort = sortStars
	m.applyFilter()
	want := []string{"nethole-tester", "hubtop", "mcpscope"}
	for i, idx := range m.viewIdx {
		if m.repos[idx].Name != want[i] {
			t.Fatalf("stars sort: position %d = %s, want %s", i, m.repos[idx].Name, want[i])
		}
	}
	m.sort = sortName
	m.applyFilter()
	want = []string{"hubtop", "mcpscope", "nethole-tester"}
	for i, idx := range m.viewIdx {
		if m.repos[idx].Name != want[i] {
			t.Fatalf("name sort: position %d = %s, want %s", i, m.repos[idx].Name, want[i])
		}
	}
	m.sort = sortUpdated // default: API order
	m.applyFilter()
	if m.repos[m.viewIdx[0]].Name != "mcpscope" {
		t.Fatalf("updated sort should keep API order, got %s", m.repos[m.viewIdx[0]].Name)
	}
}

func TestHelpOverlayToggles(t *testing.T) {
	m := testModel()
	m.ready = true
	m.showHelp = true
	v := m.View()
	if !strings.Contains(v, "key bindings") || !strings.Contains(v, "cycle sort") {
		t.Fatalf("help overlay missing expected content:\n%s", v)
	}
	// any key closes the overlay
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.showHelp {
		t.Fatal("help overlay did not close on key press")
	}
}

func TestFailedStepsDisplay(t *testing.T) {
	d := &repoDetail{failedInfo: map[int64][]string{42: {"lint / gofmt", "test / build"}}}
	if ss, ok := d.failedInfo[42]; !ok || len(ss) != 2 {
		t.Fatalf("bad failedInfo: %v", d.failedInfo)
	}
}

func TestApplyFilter(t *testing.T) {
	m := testModel()
	if len(m.viewIdx) != 3 {
		t.Fatalf("want 3 visible, got %d", len(m.viewIdx))
	}
	m.filterInput.SetValue("scope")
	m.applyFilter()
	if len(m.viewIdx) != 1 || m.repos[m.viewIdx[0]].Name != "mcpscope" {
		t.Fatalf("bad filter result: %v", m.viewIdx)
	}
	m.filterInput.SetValue("ME/")
	m.applyFilter()
	if len(m.viewIdx) != 3 {
		t.Fatalf("filter should be case-insensitive, got %d", len(m.viewIdx))
	}
	m.filterInput.SetValue("zzz")
	m.applyFilter()
	if len(m.viewIdx) != 0 {
		t.Fatalf("want 0 visible, got %d", len(m.viewIdx))
	}
	// clearing restores all
	m.filterInput.SetValue("")
	m.applyFilter()
	if len(m.viewIdx) != 3 {
		t.Fatalf("want 3 visible after clear, got %d", len(m.viewIdx))
	}
}

func TestApplyFilterClampsCursor(t *testing.T) {
	m := testModel()
	m.cursor = 2
	m.filterInput.SetValue("mcpscope")
	m.applyFilter()
	if m.cursor != 0 {
		t.Fatalf("cursor not clamped: %d", m.cursor)
	}
}
