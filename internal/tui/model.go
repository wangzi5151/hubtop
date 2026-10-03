package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	gh "github.com/wangzi5151/hubtop/internal/github"
)

type view int

const (
	viewRepos view = iota
	viewDetail
)

const (
	maxRunPrefetch = 30
	refreshEvery   = 60 * time.Second
)

// Run starts the hubtop TUI.
func Run(client *gh.Client) error {
	m := newModel(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type model struct {
	client *gh.Client

	view   view
	repos  []gh.Repo
	runOf  map[int]*gh.WorkflowRun // repo index -> latest run (nil = none yet)
	hasRun map[int]bool            // whether runOf[idx] was resolved

	cursor  int
	offset  int
	viewIdx []int // repo indices currently shown (after filter)
	gen     int   // invalidates in-flight fetches

	filtering   bool
	filterInput textinput.Model

	status string // transient status line, cleared on refresh

	loading bool
	err     error

	detail    *repoDetail
	detailIdx int

	vp    viewport.Model
	w, h  int
	ready bool
}

type repoDetail struct {
	repo       gh.Repo
	runs       []gh.WorkflowRun
	failedInfo map[int64]string // run ID -> "job / step"
	release    *gh.Release
	issues     []gh.Issue
	pulls      []gh.PullRequest
}

func newModel(client *gh.Client) *model {
	ti := textinput.New()
	ti.Placeholder = "filter repos…"
	ti.CharLimit = 64
	return &model{
		client:      client,
		runOf:       make(map[int]*gh.WorkflowRun),
		hasRun:      make(map[int]bool),
		filterInput: ti,
	}
}

// openBrowser opens a URL in the system browser (best effort).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// ---- messages ----

type reposMsg struct {
	gen   int
	repos []gh.Repo
	err   error
}

type runStatusMsg struct {
	gen int
	idx int
	run *gh.WorkflowRun // nil when the repo has no runs
}

type detailMsg struct {
	gen    int
	idx    int
	detail *repoDetail
	err    error
}

type tickMsg time.Time

type rerunMsg struct {
	gen     int
	idx     int
	err     error
	runName string
}

// ---- commands ----

func (m *model) fetchReposCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repos, err := m.client.ListRepos(ctx)
		return reposMsg{gen: gen, repos: repos, err: err}
	}
}

func (m *model) fetchRunCmd(gen, idx int, owner, repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		runs, err := m.client.ListWorkflowRuns(ctx, owner, repo, 1)
		if err != nil {
			return runStatusMsg{gen: gen, idx: idx}
		}
		var run *gh.WorkflowRun
		if len(runs) > 0 {
			run = &runs[0]
		}
		return runStatusMsg{gen: gen, idx: idx, run: run}
	}
}

func (m *model) fetchDetailCmd(gen, idx int, repo gh.Repo) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		owner, name := splitRepo(repo.FullName)
		d := &repoDetail{repo: repo, failedInfo: make(map[int64]string)}
		if runs, err := m.client.ListWorkflowRuns(ctx, owner, name, 10); err == nil {
			d.runs = runs
			// Resolve the failed job/step for the first couple of failed runs.
			n := 0
			for _, r := range runs {
				if c := r.ConclusionValue(); c == "failure" || c == "timed_out" {
					if jobs, err := m.client.ListRunJobs(ctx, owner, name, r.ID); err == nil {
						if s := gh.FailedStep(jobs); s != "" {
							d.failedInfo[r.ID] = s
						}
					}
					if n++; n >= 2 {
						break
					}
				}
			}
		}
		if rel, err := m.client.LatestRelease(ctx, owner, name); err == nil {
			d.release = rel
		}
		if issues, err := m.client.ListOpenIssues(ctx, owner, name, 5); err == nil {
			d.issues = issues
		}
		if pulls, err := m.client.ListOpenPulls(ctx, owner, name, 5); err == nil {
			d.pulls = pulls
		}
		return detailMsg{gen: gen, idx: idx, detail: d}
	}
}

func tickCmd() tea.Msg {
	time.Sleep(refreshEvery)
	return tickMsg(time.Now())
}

// rerunCmd re-runs the latest failed run in the current detail view.
func (m *model) rerunCmd() tea.Cmd {
	if m.detail == nil {
		m.status = "still loading…"
		return nil
	}
	var target *gh.WorkflowRun
	for i := range m.detail.runs {
		if c := m.detail.runs[i].ConclusionValue(); c == "failure" || c == "timed_out" {
			target = &m.detail.runs[i]
			break
		}
	}
	if target == nil {
		m.status = "no failed runs to re-run"
		return nil
	}
	run := *target
	owner, name := splitRepo(m.detail.repo.FullName)
	gen, idx := m.gen, m.detailIdx
	m.status = fmt.Sprintf("re-running %s…", run.Name)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := m.client.RerunWorkflowRun(ctx, owner, name, run.ID)
		return rerunMsg{gen: gen, idx: idx, err: err, runName: run.Name}
	}
}

// applyFilter rebuilds the visible repo index list from the filter text.
func (m *model) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	m.viewIdx = m.viewIdx[:0]
	for i, r := range m.repos {
		if q == "" || strings.Contains(strings.ToLower(r.Name), q) ||
			strings.Contains(strings.ToLower(r.FullName), q) {
			m.viewIdx = append(m.viewIdx, i)
		}
	}
	if m.cursor >= len(m.viewIdx) {
		m.cursor = max(0, len(m.viewIdx)-1)
	}
	m.offset = 0
}

func splitRepo(full string) (owner, repo string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", full
}

// ---- model ----

func (m *model) Init() tea.Cmd {
	m.loading = true
	m.gen++
	return tea.Batch(m.fetchReposCmd(m.gen), tickCmd)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.ready = true
		m.vp.Width = msg.Width
		m.vp.Height = msg.Height - 4
		return m, nil

	case reposMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.repos = msg.repos
		m.runOf = make(map[int]*gh.WorkflowRun)
		m.hasRun = make(map[int]bool)
		m.applyFilter()
		cmds := make([]tea.Cmd, 0, len(m.repos))
		for i, r := range m.repos {
			if i >= maxRunPrefetch {
				break
			}
			owner, name := splitRepo(r.FullName)
			cmds = append(cmds, m.fetchRunCmd(m.gen, i, owner, name))
		}
		return m, tea.Batch(cmds...)

	case runStatusMsg:
		if msg.gen != m.gen || msg.idx >= len(m.repos) {
			return m, nil
		}
		m.runOf[msg.idx] = msg.run
		m.hasRun[msg.idx] = true
		return m, nil

	case detailMsg:
		if msg.gen != m.gen || msg.idx != m.detailIdx {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detail = msg.detail
		m.vp.SetContent(m.renderDetail())
		m.vp.GotoTop()
		return m, nil

	case tickMsg:
		m.gen++
		m.status = ""
		return m, tea.Batch(m.fetchReposCmd(m.gen), tickCmd)

	case rerunMsg:
		if msg.gen != m.gen || msg.idx != m.detailIdx {
			return m, nil
		}
		if msg.err != nil {
			m.status = "re-run failed: " + msg.err.Error()
			return m, nil
		}
		m.status = "re-run started: " + msg.runName
		m.gen++
		m.detail = nil
		return m, m.fetchDetailCmd(m.gen, m.detailIdx, m.repos[m.detailIdx])

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.view == viewDetail {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter mode: all keys go to the input. The explicit m.filtering flag
	// (not Focused()) decides this, so typing q/r inside the filter is safe.
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
			return m, nil
		case "esc":
			m.filtering = false
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.applyFilter()
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		if m.view == viewRepos {
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "o":
		repo := m.currentRepo()
		if repo != nil && repo.HTMLURL != "" {
			openBrowser(repo.HTMLURL)
			m.status = "opened " + repo.FullName + " in browser"
		}
		return m, nil
	case "R":
		if m.view == viewDetail {
			return m, m.rerunCmd()
		}
		return m, nil
	case "r":
		m.gen++
		m.status = ""
		if m.view == viewDetail && m.detailIdx < len(m.repos) {
			m.detail = nil
			return m, m.fetchDetailCmd(m.gen, m.detailIdx, m.repos[m.detailIdx])
		}
		m.loading = true
		return m, m.fetchReposCmd(m.gen)
	case "esc":
		if m.view == viewDetail {
			m.view = viewRepos
			m.detail = nil
			return m, nil
		}
		return m, nil
	case "enter":
		if m.view == viewRepos && len(m.viewIdx) > 0 {
			m.view = viewDetail
			m.detailIdx = m.viewIdx[m.cursor]
			m.detail = nil
			m.gen++
			return m, m.fetchDetailCmd(m.gen, m.detailIdx, m.repos[m.detailIdx])
		}
		return m, nil
	}

	if m.view == viewRepos {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.viewIdx)-1 {
				m.cursor++
				if m.h > 0 && m.cursor >= m.offset+m.visibleRows() {
					m.offset = m.cursor - m.visibleRows() + 1
				}
			}
		case "home", "g":
			m.cursor, m.offset = 0, 0
		case "end", "G":
			m.cursor = len(m.viewIdx) - 1
			if m.h > 0 {
				m.offset = max(0, m.cursor-m.visibleRows()+1)
			}
		}
		return m, nil
	}

	// detail view: let the viewport scroll
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// currentRepo returns the repo under the cursor (or in the detail view).
func (m *model) currentRepo() *gh.Repo {
	if m.view == viewDetail && m.detail != nil {
		return &m.detail.repo
	}
	if m.view == viewRepos && len(m.viewIdx) > 0 && m.cursor < len(m.viewIdx) {
		r := m.repos[m.viewIdx[m.cursor]]
		return &r
	}
	return nil
}

func (m *model) bottomBars() int {
	n := 0
	if m.filtering {
		n++
	}
	if m.status != "" {
		n++
	}
	return n
}

func (m *model) visibleRows() int {
	n := m.h - 2 - m.bottomBars()
	if n < 1 {
		n = 1
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ---- view ----

func (m *model) View() string {
	if !m.ready {
		return "loading..."
	}
	if m.view == viewDetail {
		return m.viewDetail()
	}
	return m.viewRepos()
}

func (m *model) viewRepos() string {
	var b strings.Builder
	title := headerStyle.Render(fmt.Sprintf("hubtop — %d repos", len(m.repos)))
	hint := dimStyle.Render("j/k move · enter detail · / filter · o open · r refresh · q quit")
	b.WriteString(title + "  " + hint + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", max(10, m.w-1))) + "\n")

	if m.loading {
		b.WriteString("loading repos…\n")
		return b.String()
	}
	if m.err != nil {
		b.WriteString(red.Render("error: "+m.err.Error()) + "\n")
		return b.String()
	}
	if len(m.viewIdx) == 0 {
		if m.filterInput.Value() != "" {
			b.WriteString(dimStyle.Render("no repos match filter") + "\n")
		} else {
			b.WriteString(dimStyle.Render("no repositories found") + "\n")
		}
	} else {
		rows := m.visibleRows()
		for p := m.offset; p < len(m.viewIdx) && p < m.offset+rows; p++ {
			b.WriteString(m.renderRepoRow(m.viewIdx[p], p == m.cursor) + "\n")
		}
	}
	if m.filtering {
		b.WriteString("\n" + m.filterInput.View())
	}
	if m.status != "" {
		b.WriteString("\n" + dimStyle.Render(m.status))
	}
	return b.String()
}

func (m *model) renderRepoRow(repoIdx int, selected bool) string {
	r := m.repos[repoIdx]

	dot := dimStyle.Render("○")
	if m.hasRun[repoIdx] {
		run := m.runOf[repoIdx]
		if run == nil {
			dot = gray.Render("–")
		} else {
			dot = repoHealth(run.ConclusionValue())
		}
	}

	lock := ""
	if r.Private {
		lock = dimStyle.Render(" 🔒")
	}
	name := nameStyle.Render(r.Name)
	meta := dimStyle.Render(fmt.Sprintf("★%d  !%d  %s  %s", r.Stars, r.OpenIssues, r.Language, relTime(r.UpdatedAt)))
	line := fmt.Sprintf("%s %s%s  %s", dot, name, lock, meta)

	if selected {
		// pad to full width for the highlight bar
		pad := max(0, m.w-len(stripANSI(line))-1)
		return selStyle.Render(line + strings.Repeat(" ", pad))
	}
	return " " + line
}

// stripANSI is a tiny helper for width math; good enough for our own styles.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
