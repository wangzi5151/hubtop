// Command hubtop is a terminal dashboard for your GitHub repositories.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	gh "github.com/wangzi5151/hubtop/internal/github"
	"github.com/wangzi5151/hubtop/internal/tui"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hubtop:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		failures    = flag.Bool("failures", false, "list repos with failing/running CI and exit non-zero on failures (no TUI)")
		onlyFailing = flag.Bool("only-failing", false, "like --failures but list only repos whose latest CI run failed (no TUI)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hubtop - terminal dashboard for your GitHub repositories\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  hubtop                 open the dashboard (needs a terminal)\n  hubtop --failures      check CI status across repos, script-friendly\n  hubtop --only-failing  list only repos with failing CI\n\n")
		fmt.Fprintf(os.Stderr, "Auth: set GITHUB_TOKEN, or log in with `gh auth login`.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Printf("hubtop %s (%s)\n", version, commit)
		return nil
	}

	token := resolveToken()
	if token == "" {
		return fmt.Errorf("no GitHub token: set GITHUB_TOKEN or run `gh auth login`")
	}
	client := gh.New(token)

	if *failures || *onlyFailing {
		return runFailures(client, *onlyFailing)
	}

	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("no TTY detected: hubtop needs an interactive terminal (or use --failures)")
	}
	return tui.Run(client)
}

// resolveToken prefers GITHUB_TOKEN, then falls back to the gh CLI's token.
func resolveToken() string {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runFailures prints repos whose latest workflow run is failing or running.
// With onlyFailing it skips the "running" rows. It exits non-zero when at
// least one repo failed; API errors are never treated as "no runs".
func runFailures(client *gh.Client, onlyFailing bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	repos, err := client.ListRepos(ctx)
	if err != nil {
		return err
	}
	failed := 0
	var checkErrs []error
	for _, r := range repos {
		owner, name := split(r.FullName)
		runs, err := client.ListWorkflowRuns(ctx, owner, name, 1)
		if err != nil {
			checkErrs = append(checkErrs, fmt.Errorf("%s: %w", r.FullName, err))
			continue
		}
		if len(runs) == 0 {
			continue
		}
		run := runs[0]
		if run.Failed() {
			why := ""
			if jobs, err := client.ListRunJobs(ctx, owner, name, run.ID); err == nil {
				if ss := gh.FailedSteps(jobs); len(ss) > 0 {
					why = " ← " + strings.Join(ss, ", ")
				}
			}
			fmt.Printf("FAIL  %-32s %s %s%s %s\n", r.FullName, run.Name, run.HeadBranch, why, run.HTMLURL)
			failed++
		} else if run.ConclusionValue() == "" && !onlyFailing {
			fmt.Printf("RUN   %-32s %s %s\n", r.FullName, run.Name, run.HeadBranch)
		}
	}

	if len(checkErrs) > 0 {
		for _, err := range checkErrs {
			fmt.Fprintln(os.Stderr, "ERR  ", err)
		}
	}
	switch {
	case failed > 0 && len(checkErrs) > 0:
		return fmt.Errorf("%d repo(s) with failing CI; %d repo(s) could not be checked", failed, len(checkErrs))
	case failed > 0:
		return fmt.Errorf("%d repo(s) with failing CI", failed)
	case len(checkErrs) > 0:
		return fmt.Errorf("%d repo(s) could not be checked", len(checkErrs))
	}
	fmt.Println("all clear")
	return nil
}

func split(full string) (string, string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", full
}
