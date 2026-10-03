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
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "hubtop - terminal dashboard for your GitHub repositories\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  hubtop            open the dashboard (needs a terminal)\n  hubtop --failures check CI status across repos, script-friendly\n\n")
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

	if *failures {
		return runFailures(client)
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
// It exits non-zero when at least one repo has a failed run.
func runFailures(client *gh.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	repos, err := client.ListRepos(ctx)
	if err != nil {
		return err
	}
	failed := 0
	for _, r := range repos {
		owner, name := split(r.FullName)
		runs, err := client.ListWorkflowRuns(ctx, owner, name, 1)
		if err != nil || len(runs) == 0 {
			continue
		}
		run := runs[0]
		concl := run.ConclusionValue()
		if concl == "failure" || concl == "timed_out" {
			fmt.Printf("FAIL  %-32s %s %s %s\n", r.FullName, run.Name, run.HeadBranch, run.HTMLURL)
			failed++
		} else if concl == "" {
			fmt.Printf("RUN   %-32s %s %s\n", r.FullName, run.Name, run.HeadBranch)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d repo(s) with failing CI", failed)
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
