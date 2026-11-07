package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

func manyRepos(n int) *model {
	m := newModel(nil)
	m.repos = make([]gh.Repo, n)
	for i := range m.repos {
		m.repos[i] = gh.Repo{Name: fmt.Sprintf("r%02d", i), FullName: fmt.Sprintf("me/r%02d", i)}
	}
	m.applyFilter()
	m.ready = true
	return m
}

func TestRunFetchTargetsNotCappedAtThirty(t *testing.T) {
	m := manyRepos(100)
	m.h = 200 // tall terminal: every row is visible
	if got := len(m.runFetchTargets()); got != 100 {
		t.Fatalf("visible targets = %d, want all 100 (no 30-repo cap)", got)
	}
}

func TestRunFetchTargetsTracksVisibleWindow(t *testing.T) {
	m := manyRepos(100)
	m.h = 12 // 10 visible rows
	m.offset = 40
	targets := m.runFetchTargets()
	if len(targets) != 10 {
		t.Fatalf("targets = %v, want 10 visible rows", targets)
	}
	for i, idx := range targets {
		if idx != 40+i {
			t.Fatalf("targets[%d] = %d, want %d", i, idx, 40+i)
		}
	}

	m.hasRun[40] = true
	m.runLoading[41] = true
	targets = m.runFetchTargets()
	if len(targets) != 8 || targets[0] != 42 {
		t.Fatalf("loaded/in-flight rows should be skipped, got %v", targets)
	}
}

func TestPrefetchVisibleRunsMarksInFlight(t *testing.T) {
	m := manyRepos(10)
	m.h = 12
	cmd := m.prefetchVisibleRuns()
	if cmd == nil {
		t.Fatal("prefetch returned no command")
	}
	if got := len(m.runFetchTargets()); got != 0 {
		t.Fatalf("targets after prefetch = %d, want 0 (all in flight)", got)
	}
	if cmd := m.prefetchVisibleRuns(); cmd != nil {
		t.Fatal("second prefetch should be a no-op while fetches are in flight")
	}
}

func TestOpenBrowserWaitsForChild(t *testing.T) {
	oldCmd, oldWait := browserCommand, waitBrowser
	t.Cleanup(func() {
		browserCommand, waitBrowser = oldCmd, oldWait
	})

	browserCommand = func(string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=^$")
	}
	waited := make(chan struct{})
	waitBrowser = func(cmd *exec.Cmd) error {
		defer close(waited)
		return cmd.Wait()
	}

	openBrowser("https://example.com")
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("browser child was not waited/reaped")
	}
}
