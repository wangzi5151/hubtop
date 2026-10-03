package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *[]http.Request) {
	t.Helper()
	var got []http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, *r)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := New("test-token")
	c.setBase(srv.URL)
	return c, &got
}

func TestAuthHeader(t *testing.T) {
	c, got := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	_, _ = c.ListRepos(context.Background())
	if len(*got) != 1 {
		t.Fatalf("want 1 request, got %d", len(*got))
	}
	if h := (*got)[0].Header.Get("Authorization"); h != "Bearer test-token" {
		t.Fatalf("bad auth header: %q", h)
	}
}

func TestListRepos(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/user/repos") {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		w.Write([]byte(`[{"name":"hubtop","full_name":"me/hubtop","private":false,"stargazers_count":7,"open_issues_count":3,"language":"Go","updated_at":"2026-10-03T10:00:00Z","default_branch":"main"}]`))
	})
	repos, err := c.ListRepos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "hubtop" || repos[0].Stars != 7 || repos[0].OpenIssues != 3 {
		t.Fatalf("bad repos: %+v", repos)
	}
}

func TestListWorkflowRuns(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"workflow_runs":[{"id":1,"name":"CI","status":"completed","conclusion":"success","event":"push","head_branch":"main","head_sha":"abc123","html_url":"https://example.com","created_at":"2026-10-03T10:00:00Z","updated_at":"2026-10-03T10:01:00Z"},{"id":2,"name":"CI","status":"in_progress","conclusion":null,"event":"push","head_branch":"main","head_sha":"def456","html_url":"https://example.com","created_at":"2026-10-03T11:00:00Z","updated_at":"2026-10-03T11:00:30Z"}]}`))
	})
	runs, err := c.ListWorkflowRuns(context.Background(), "me", "hubtop", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(runs))
	}
	if runs[0].ConclusionValue() != "success" {
		t.Fatalf("bad conclusion: %q", runs[0].ConclusionValue())
	}
	if runs[1].ConclusionValue() != "" {
		t.Fatalf("in-progress run should have empty conclusion, got %q", runs[1].ConclusionValue())
	}
	if d := runs[0].Duration(); d.String() != "1m0s" {
		t.Fatalf("bad duration: %s", d)
	}
}

func TestLatestRelease(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"tag_name":"v0.1.0","name":"v0.1.0","published_at":"2026-10-01T10:00:00Z","html_url":"https://example.com"}]`))
	})
	rel, err := c.LatestRelease(context.Background(), "me", "hubtop")
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil || rel.TagName != "v0.1.0" {
		t.Fatalf("bad release: %+v", rel)
	}

	c2, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	rel, err = c2.LatestRelease(context.Background(), "me", "empty")
	if err != nil || rel != nil {
		t.Fatalf("want (nil, nil), got (%+v, %v)", rel, err)
	}
}

func TestListOpenIssuesFiltersPRs(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"number":1,"title":"real issue","html_url":"x","created_at":"2026-10-03T10:00:00Z"},{"number":2,"title":"a pr","html_url":"x","created_at":"2026-10-03T10:00:00Z","pull_request":{}}]`))
	})
	issues, err := c.ListOpenIssues(context.Background(), "me", "hubtop", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Number != 1 {
		t.Fatalf("PR not filtered: %+v", issues)
	}
}

func TestAPIError(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	})
	_, err := c.ListRepos(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Bad credentials") {
		t.Fatalf("want api error, got %v", err)
	}
}
