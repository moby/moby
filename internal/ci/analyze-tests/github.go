package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v68/github"
)

// githubClient is the small GitHub REST API client used by the analyzer.
// It intentionally covers only the endpoints needed for workflow artifacts,
// issue tracking, and labels.
type githubClient struct {
	repo    string
	token   string
	client  *http.Client
	verbose bool
}

// githubAPI returns a go-github client configured like the local REST client.
func (g *githubClient) githubAPI() *github.Client {
	c := github.NewClient(g.client)
	if g.token != "" {
		c = c.WithAuthToken(g.token)
	}
	return c
}

// repoParts splits the configured owner/name repository string.
func (g *githubClient) repoParts() (string, string, error) {
	owner, repo, ok := strings.Cut(g.repo, "/")
	if !ok || owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/name", g.repo)
	}
	return owner, repo, nil
}

// downloadArtifact downloads the zip archive for one GitHub Actions artifact.
func (g *githubClient) downloadArtifact(ctx context.Context, artifactID int64) ([]byte, error) {
	owner, repo, err := g.repoParts()
	if err != nil {
		return nil, err
	}
	u, _, err := g.githubAPI().Actions.DownloadArtifact(ctx, owner, repo, artifactID, 0)
	if err != nil {
		return nil, g.githubError("get artifact download URL", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, g.httpStatusError("GET", u.String(), resp, body)
	}
	return io.ReadAll(resp.Body)
}

func (g *githubClient) githubError(operation string, err error) error {
	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) || ghErr.Response == nil {
		return fmt.Errorf("%s for repo %s: %w", operation, g.repo, err)
	}

	resp := ghErr.Response
	parts := []string{
		fmt.Sprintf("%s for repo %s failed", operation, g.repo),
		fmt.Sprintf("status=%s", resp.Status),
	}
	if resp.Request != nil && resp.Request.URL != nil {
		parts = append(parts, fmt.Sprintf("endpoint=%s", resp.Request.URL.Redacted()))
	}
	parts = append(parts, fmt.Sprintf("authenticated=%t", g.token != ""))
	if requestID := resp.Header.Get("X-GitHub-Request-Id"); requestID != "" {
		parts = append(parts, fmt.Sprintf("github_request_id=%s", requestID))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		parts = append(parts, "hint=check GH_TOKEN/GITHUB_TOKEN or gh auth token; token needs access to repository Actions artifacts")
	}

	return fmt.Errorf("%s: %w", strings.Join(parts, "; "), err)
}

func (g *githubClient) httpStatusError(method, url string, resp *http.Response, body []byte) error {
	parts := []string{
		fmt.Sprintf("%s %s failed", method, url),
		fmt.Sprintf("status=%s", resp.Status),
		fmt.Sprintf("authenticated=%t", g.token != ""),
	}
	if requestID := resp.Header.Get("X-GitHub-Request-Id"); requestID != "" {
		parts = append(parts, fmt.Sprintf("github_request_id=%s", requestID))
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		parts = append(parts, fmt.Sprintf("body=%q", trimmed))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		parts = append(parts, "hint=check GH_TOKEN/GITHUB_TOKEN or gh auth token; token needs access to repository Actions artifacts")
	}

	return errors.New(strings.Join(parts, "; "))
}
