package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type githubClient struct {
	baseURL    string
	httpClient *http.Client
}

type githubPullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"html_url"`
}

func newGitHubClient(httpClient *http.Client) *githubClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &githubClient{
		baseURL:    "https://api.github.com",
		httpClient: httpClient,
	}
}

func (c *githubClient) Ensure(ctx context.Context, req PullRequestRequest) (PullRequestResult, error) {
	existing, err := c.findOpen(ctx, req)
	if err != nil {
		return PullRequestResult{}, err
	}
	if existing != nil {
		updated, err := c.update(ctx, req, existing.Number)
		if err != nil {
			return PullRequestResult{}, err
		}
		return PullRequestResult{
			Operation: PROperationUpdated,
			Number:    updated.Number,
			URL:       updated.URL,
		}, nil
	}

	created, err := c.create(ctx, req)
	if err != nil {
		return PullRequestResult{}, err
	}
	return PullRequestResult{
		Operation: PROperationCreated,
		Number:    created.Number,
		URL:       created.URL,
	}, nil
}

func (c *githubClient) findOpen(ctx context.Context, req PullRequestRequest) (*githubPullRequest, error) {
	owner, _, err := splitRepository(req.Repository)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+req.HeadBranch)
	query.Set("base", req.BaseBranch)
	query.Set("per_page", "1")
	endpoint := fmt.Sprintf("%s/repos/%s/pulls?%s", c.baseURL, req.Repository, query.Encode())
	request, err := c.newRequest(ctx, http.MethodGet, endpoint, req.Token, nil)
	if err != nil {
		return nil, err
	}

	var pulls []githubPullRequest
	if err := c.doJSON(request, &pulls); err != nil {
		return nil, err
	}
	if len(pulls) == 0 {
		return nil, nil
	}
	return &pulls[0], nil
}

func (c *githubClient) create(ctx context.Context, req PullRequestRequest) (githubPullRequest, error) {
	body := map[string]string{
		"title": req.Title,
		"head":  req.HeadBranch,
		"base":  req.BaseBranch,
		"body":  "Created by the updtr GitHub Action.",
	}
	request, err := c.newRequest(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/pulls", c.baseURL, req.Repository), req.Token, body)
	if err != nil {
		return githubPullRequest{}, err
	}
	var pull githubPullRequest
	if err := c.doJSON(request, &pull); err != nil {
		return githubPullRequest{}, err
	}
	return pull, nil
}

func (c *githubClient) update(ctx context.Context, req PullRequestRequest, number int) (githubPullRequest, error) {
	body := map[string]string{"title": req.Title}
	request, err := c.newRequest(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, req.Repository, number), req.Token, body)
	if err != nil {
		return githubPullRequest{}, err
	}
	var pull githubPullRequest
	if err := c.doJSON(request, &pull); err != nil {
		return githubPullRequest{}, err
	}
	return pull, nil
}

func (c *githubClient) newRequest(ctx context.Context, method string, endpoint string, token string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func (c *githubClient) doJSON(request *http.Request, out any) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = response.Status
		}
		return fmt.Errorf("github api %s %s: %s", request.Method, request.URL.Path, message)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode github api response: %w", err)
	}
	return nil
}

func splitRepository(repository string) (string, string, error) {
	owner, name, ok := strings.Cut(repository, "/")
	if !ok || owner == "" || name == "" {
		return "", "", fmt.Errorf("invalid repository %q", repository)
	}
	return owner, name, nil
}
