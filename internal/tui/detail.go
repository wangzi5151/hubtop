package tui

import (
	"fmt"
	"strings"
)

func (m *model) viewDetail() string {
	var b strings.Builder
	hint := dimStyle.Render("esc back · r refresh · q quit")
	b.WriteString(headerStyle.Render("hubtop") + "  " + hint + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", max(10, m.w-1))) + "\n")
	if m.detail == nil {
		b.WriteString("loading…\n")
		return b.String()
	}
	b.WriteString(m.vp.View())
	return b.String()
}

func (m *model) renderDetail() string {
	d := m.detail
	var b strings.Builder

	lock := "public"
	if d.repo.Private {
		lock = "private"
	}
	b.WriteString(nameStyle.Render(d.repo.FullName) + dimStyle.Render("  "+lock))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  ★%d  %s", d.repo.Stars, d.repo.Language)) + "\n")
	if d.repo.Description != "" {
		b.WriteString(dimStyle.Render(d.repo.Description) + "\n")
	}
	b.WriteString("\n")

	// Workflow runs
	b.WriteString(sectionStyle.Render(fmt.Sprintf("Workflow runs (%d)", len(d.runs))) + "\n")
	if len(d.runs) == 0 {
		b.WriteString(dimStyle.Render("  no runs yet") + "\n")
	}
	for _, r := range d.runs {
		concl := r.ConclusionValue()
		if concl == "" {
			concl = r.Status
		}
		sha := r.HeadSHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s  %s  %s\n",
			runIcon(r.Status, r.ConclusionValue()),
			r.Name, r.HeadBranch, sha, concl, shortDur(r.Duration()), relTime(r.CreatedAt)))
	}
	b.WriteString("\n")

	// Latest release
	b.WriteString(sectionStyle.Render("Latest release") + "\n")
	if d.release == nil {
		b.WriteString(dimStyle.Render("  no releases") + "\n")
	} else {
		name := d.release.Name
		if name == "" {
			name = d.release.TagName
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n", nameStyle.Render(name), dimStyle.Render(relTime(d.release.PublishedAt))))
	}
	b.WriteString("\n")

	// Pull requests
	b.WriteString(sectionStyle.Render(fmt.Sprintf("Open pull requests (%d)", len(d.pulls))) + "\n")
	if len(d.pulls) == 0 {
		b.WriteString(dimStyle.Render("  none") + "\n")
	}
	for _, p := range d.pulls {
		draft := ""
		if p.Draft {
			draft = dimStyle.Render(" [draft]")
		}
		b.WriteString(fmt.Sprintf("  #%d %s%s %s\n", p.Number, p.Title, draft, dimStyle.Render(relTime(p.CreatedAt))))
	}
	b.WriteString("\n")

	// Issues
	b.WriteString(sectionStyle.Render(fmt.Sprintf("Open issues (%d)", len(d.issues))) + "\n")
	if len(d.issues) == 0 {
		b.WriteString(dimStyle.Render("  none") + "\n")
	}
	for _, is := range d.issues {
		b.WriteString(fmt.Sprintf("  #%d %s %s\n", is.Number, is.Title, dimStyle.Render(relTime(is.CreatedAt))))
	}

	return b.String()
}
