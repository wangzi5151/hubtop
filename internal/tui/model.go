package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	cursor int
	offset int
	gen    int // invalidates in-flight fetches

	loading bool
	err     error

	detail    *repoDetail
	detailIdx int

	vp    viewport.Model
	w, h  int
	ready bool
}

type repoDetail struct {
	repo    gh.Repo
	runs    []gh.WorkflowRun
	release *gh.Release
	issues  []gh.Issue
	pulls   []gh.PullRequest
}

func newModel(client *gh.Client) *model {
	return &model{
		client: client,
		runOf:  make(map[int]*gh.WorkflowRun),
		hasRun: make(map[int]bool),
	}
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
		d := &repoDetail{repo: repo}
		if runs, err := m.client.ListWorkflowRuns(ctx, owner, name, 10); err == nil {
			d.runs = runs
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
		if m.cursor >= len(m.repos) {
			m.cursor = max(0, len(m.repos)-1)
		}
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
		return m, tea.Batch(m.fetchReposCmd(m.gen), tickCmd)

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
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		m.gen++
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
		if m.view == viewRepos && len(m.repos) > 0 {
			m.view = viewDetail
			m.detailIdx = m.cursor
			m.detail = nil
			m.gen++
			return m, m.fetchDetailCmd(m.gen, m.cursor, m.repos[m.cursor])
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
			if m.cursor < len(m.repos)-1 {
				m.cursor++
				if m.h > 0 && m.cursor >= m.offset+m.visibleRows() {
					m.offset = m.cursor - m.visibleRows() + 1
				}
			}
		case "home", "g":
			m.cursor, m.offset = 0, 0
		case "end", "G":
			m.cursor = len(m.repos) - 1
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

func (m *model) visibleRows() int {
	n := m.h - 4
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
	hint := dimStyle.Render("j/k move · enter detail · r refresh · q quit")
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
	if len(m.repos) == 0 {
		b.WriteString(dimStyle.Render("no repositories found") + "\n")
		return b.String()
	}

	rows := m.visibleRows()
	for i := m.offset; i < len(m.repos) && i < m.offset+rows; i++ {
		b.WriteString(m.renderRepoRow(i) + "\n")
	}
	return b.String()
}

func (m *model) renderRepoRow(i int) string {
	r := m.repos[i]

	dot := dimStyle.Render("○")
	if m.hasRun[i] {
		run := m.runOf[i]
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

	if i == m.cursor {
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
