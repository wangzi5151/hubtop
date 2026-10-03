// Package github is a small GitHub REST API client used by hubtop.
// It covers exactly what the dashboard needs: repos, workflow runs,
// releases, issues and pull requests.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the GitHub REST API.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// New returns a client authenticating with the given token.
// The token is read from GITHUB_TOKEN or `gh auth token` by the caller.
// GITHUB_API_BASE overrides the API endpoint (tests, GitHub Enterprise).
func New(token string) *Client {
	base := "https://api.github.com"
	if v := strings.TrimSuffix(os.Getenv("GITHUB_API_BASE"), "/"); v != "" {
		base = v
	}
	return &Client{
		base:  base,
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

// setBase overrides the API base URL. Used by tests.
func (c *Client) setBase(base string) { c.base = base }

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hubtop")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Message == "" {
			apiErr.Message = resp.Status
		}
		return fmt.Errorf("github api %s: %s", path, apiErr.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// post sends an empty POST and expects a 2xx status.
func (c *Client) post(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hubtop")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &apiErr)
		if apiErr.Message == "" {
			apiErr.Message = resp.Status
		}
		return fmt.Errorf("github api %s: %s", path, apiErr.Message)
	}
	return nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Repo is the subset of repository fields the dashboard shows.
type Repo struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	Private       bool      `json:"private"`
	Stars         int       `json:"stargazers_count"`
	OpenIssues    int       `json:"open_issues_count"`
	Language      string    `json:"language"`
	HTMLURL       string    `json:"html_url"`
	UpdatedAt     time.Time `json:"updated_at"`
	DefaultBranch string    `json:"default_branch"`
}

// ListRepos returns the authenticated user's repos, most recently updated first.
func (c *Client) ListRepos(ctx context.Context) ([]Repo, error) {
	var repos []Repo
	q := url.Values{"per_page": {"100"}, "sort": {"updated"}, "affiliation": {"owner"}}
	if err := c.get(ctx, "/user/repos", q, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

// WorkflowRun is the subset of check-run fields the dashboard shows.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`     // queued | in_progress | completed
	Conclusion *string   `json:"conclusion"` // success | failure | cancelled | skipped | null
	Event      string    `json:"event"`
	HeadBranch string    `json:"head_branch"`
	HeadSHA    string    `json:"head_sha"`
	HTMLURL    string    `json:"html_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ConclusionValue returns "" when the run has no conclusion yet.
func (r WorkflowRun) ConclusionValue() string {
	if r.Conclusion == nil {
		return ""
	}
	return *r.Conclusion
}

// Duration reports how long the run took (or has been running).
func (r WorkflowRun) Duration() time.Duration {
	if r.CreatedAt.IsZero() {
		return 0
	}
	end := r.UpdatedAt
	if r.Status != "completed" {
		end = time.Now()
	}
	if end.Before(r.CreatedAt) {
		return 0
	}
	return end.Sub(r.CreatedAt).Round(time.Second)
}

// ListWorkflowRuns returns the most recent runs for a repo, newest first.
func (c *Client) ListWorkflowRuns(ctx context.Context, owner, repo string, perPage int) ([]WorkflowRun, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 10
	}
	var wrap struct {
		Runs []WorkflowRun `json:"workflow_runs"`
	}
	q := url.Values{"per_page": {fmt.Sprint(perPage)}}
	if err := c.get(ctx, "/repos/"+owner+"/"+repo+"/actions/runs", q, &wrap); err != nil {
		return nil, err
	}
	return wrap.Runs, nil
}

// Job is a job within a workflow run.
type Job struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Conclusion *string   `json:"conclusion"`
	Steps      []JobStep `json:"steps"`
}

// JobStep is a single step within a job.
type JobStep struct {
	Name       string  `json:"name"`
	Conclusion *string `json:"conclusion"`
	Number     int     `json:"number"`
}

// ListRunJobs returns the jobs of a workflow run, newest job last.
func (c *Client) ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]Job, error) {
	var wrap struct {
		Jobs []Job `json:"jobs"`
	}
	if err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID), nil, &wrap); err != nil {
		return nil, err
	}
	return wrap.Jobs, nil
}

// RerunWorkflowRun re-runs a workflow run. Needs Actions: write.
func (c *Client) RerunWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	return c.post(ctx, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/rerun", owner, repo, runID))
}

// FailedSteps returns "job / step" for every failed step, in order.
func FailedSteps(jobs []Job) []string {
	var out []string
	for _, j := range jobs {
		if c := strVal(j.Conclusion); c == "failure" || c == "timed_out" {
			hit := false
			for _, s := range j.Steps {
				if sc := strVal(s.Conclusion); sc == "failure" || sc == "timed_out" {
					out = append(out, j.Name+" / "+s.Name)
					hit = true
				}
			}
			if !hit {
				out = append(out, j.Name)
			}
		}
	}
	return out
}

// Release is the subset of release fields the dashboard shows.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
}

// LatestRelease returns the newest release, or (nil, nil) when there is none.
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	var releases []Release
	q := url.Values{"per_page": {"1"}}
	if err := c.get(ctx, "/repos/"+owner+"/"+repo+"/releases", q, &releases); err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, nil
	}
	return &releases[0], nil
}

// Issue is the subset of issue fields the dashboard shows.
type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ListOpenIssues returns open issues (excluding pull requests), newest first.
func (c *Client) ListOpenIssues(ctx context.Context, owner, repo string, perPage int) ([]Issue, error) {
	var raw []struct {
		Issue
		PullRequest *json.RawMessage `json:"pull_request"`
	}
	q := url.Values{"per_page": {fmt.Sprint(perPage)}, "state": {"open"}, "sort": {"created"}, "direction": {"desc"}}
	if err := c.get(ctx, "/repos/"+owner+"/"+repo+"/issues", q, &raw); err != nil {
		return nil, err
	}
	// The issues endpoint also returns pull requests; filter them out.
	var out []Issue
	for _, r := range raw {
		if r.PullRequest != nil {
			continue
		}
		out = append(out, r.Issue)
	}
	return out, nil
}

// PullRequest is the subset of PR fields the dashboard shows.
type PullRequest struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Draft bool `json:"draft"`
}

// ListOpenPulls returns open pull requests, newest first.
func (c *Client) ListOpenPulls(ctx context.Context, owner, repo string, perPage int) ([]PullRequest, error) {
	var pulls []PullRequest
	q := url.Values{"per_page": {fmt.Sprint(perPage)}, "state": {"open"}, "sort": {"created"}, "direction": {"desc"}}
	if err := c.get(ctx, "/repos/"+owner+"/"+repo+"/pulls", q, &pulls); err != nil {
		return nil, err
	}
	return pulls, nil
}
