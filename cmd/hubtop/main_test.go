package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	gh "github.com/wangzi5151/hubtop/internal/github"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	callErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String(), callErr
}

func TestRunFailuresAPIErrorIsNotAllClear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/repos" {
			w.Write([]byte(`[{"name":"repo","full_name":"me/repo"}]`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_BASE", srv.URL)

	err := runFailures(gh.New("token"), false)
	if err == nil {
		t.Fatal("API error must not be treated as all clear")
	}
	if !strings.Contains(err.Error(), "could not be checked") {
		t.Fatalf("err = %v, want an API-check failure", err)
	}
}

func TestRunFailuresOnlyFailingSkipsRunning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user/repos":
			w.Write([]byte(`[
				{"name":"red","full_name":"me/red"},
				{"name":"green","full_name":"me/green"}
			]`))
		case strings.HasPrefix(r.URL.Path, "/repos/me/red/actions/runs"):
			w.Write([]byte(`{"workflow_runs":[{"id":1,"name":"CI","status":"completed","conclusion":"failure","head_branch":"main"}]}`))
		case strings.HasPrefix(r.URL.Path, "/repos/me/green/actions/runs"):
			w.Write([]byte(`{"workflow_runs":[{"id":2,"name":"CI","status":"in_progress","conclusion":null,"head_branch":"main"}]}`))
		case strings.HasSuffix(r.URL.Path, "/jobs"):
			w.Write([]byte(`{"jobs":[]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_BASE", srv.URL)

	out, err := captureStdout(t, func() error {
		return runFailures(gh.New("token"), true)
	})
	if err == nil {
		t.Fatal("want non-zero result for the failing repo")
	}
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "me/red") {
		t.Fatalf("output missing failing repo:\n%s", out)
	}
	if strings.Contains(out, "RUN   ") {
		t.Fatalf("--only-failing must not print running repos:\n%s", out)
	}
}

func TestRunFailuresAllClear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/repos" {
			w.Write([]byte(`[{"name":"green","full_name":"me/green"}]`))
			return
		}
		w.Write([]byte(`{"workflow_runs":[{"id":1,"name":"CI","status":"completed","conclusion":"success","head_branch":"main"}]}`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_BASE", srv.URL)

	out, err := captureStdout(t, func() error {
		return runFailures(gh.New("token"), false)
	})
	if err != nil {
		t.Fatalf("runFailures: %v", err)
	}
	if !strings.Contains(out, "all clear") {
		t.Fatalf("output = %q, want all clear", out)
	}
}
