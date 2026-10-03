package tui

import (
	"testing"

	gh "github.com/wangzi5151/hubtop/internal/github"
)

func testModel() *model {
	m := newModel(nil)
	m.repos = []gh.Repo{
		{Name: "mcpscope", FullName: "me/mcpscope"},
		{Name: "nethole-tester", FullName: "me/nethole-tester"},
		{Name: "hubtop", FullName: "me/hubtop"},
	}
	m.applyFilter()
	return m
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
