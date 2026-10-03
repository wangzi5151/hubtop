package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
	nameStyle   = lipgloss.NewStyle().Bold(true)

	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	gray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
)

// runIcon returns a colored status glyph for a workflow run.
func runIcon(status, conclusion string) string {
	switch conclusion {
	case "success":
		return green.Render("✓")
	case "failure", "timed_out":
		return red.Render("✗")
	case "cancelled", "skipped", "stale":
		return gray.Render("⊘")
	}
	switch status {
	case "in_progress":
		return yellow.Render("●")
	case "queued", "requested", "waiting", "pending":
		return gray.Render("○")
	}
	return gray.Render("?")
}

// relTime renders a short relative time like "5m ago".
func relTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// shortDur renders a duration compactly.
func shortDur(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// repoHealth returns the worst conclusion among recent runs for the list dot.
func repoHealth(conclusion string) string {
	switch conclusion {
	case "failure", "timed_out":
		return red.Render("●")
	case "success":
		return green.Render("●")
	case "":
		return yellow.Render("●") // running
	default:
		return gray.Render("●")
	}
}
