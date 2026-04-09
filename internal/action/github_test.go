package action

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubClientCreatesPullRequestWhenMissing(t *testing.T) {
	var createBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/mishankov/updtr/pulls":
			_ = json.NewEncoder(w).Encode([]githubPullRequest{})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/mishankov/updtr/pulls":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(githubPullRequest{Number: 7, URL: "https://example.com/pr/7"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newGitHubClient(server.Client())
	client.baseURL = server.URL

	result, err := client.Ensure(context.Background(), PullRequestRequest{
		Repository: "mishankov/updtr",
		Token:      "secret",
		BaseBranch: "main",
		HeadBranch: "updtr/all-123",
		Title:      "deps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Operation != PROperationCreated || result.Number != 7 {
		t.Fatalf("result = %+v, want created pull request", result)
	}
	if createBody["head"] != "updtr/all-123" || createBody["title"] != "deps" {
		t.Fatalf("create body = %+v", createBody)
	}
}

func TestGitHubClientUpdatesExistingPullRequest(t *testing.T) {
	var patched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/mishankov/updtr/pulls":
			_ = json.NewEncoder(w).Encode([]githubPullRequest{{Number: 9, URL: "https://example.com/pr/9"}})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/mishankov/updtr/pulls/9":
			patched = true
			_ = json.NewEncoder(w).Encode(githubPullRequest{Number: 9, URL: "https://example.com/pr/9"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newGitHubClient(server.Client())
	client.baseURL = server.URL

	result, err := client.Ensure(context.Background(), PullRequestRequest{
		Repository: "mishankov/updtr",
		Token:      "secret",
		BaseBranch: "main",
		HeadBranch: "updtr/all-123",
		Title:      "deps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("existing pull request was not updated")
	}
	if result.Operation != PROperationUpdated || result.Number != 9 {
		t.Fatalf("result = %+v, want updated pull request", result)
	}
}
