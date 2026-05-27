package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
)

const (
	label         = "status/flaky-test"
	historyMarker = "flaky-history:json"
	maxIssuePages = 5
)

// historyData is persisted in an HTML comment inside each managed issue.
type historyData struct {
	FirstSeen string       `json:"first_seen"`
	History   []historyRow `json:"history"`
}

// historyRow records one analyzer observation for a managed issue.
type historyRow struct {
	Date     string `json:"date"`
	Passes   int    `json:"passes"`
	Failures int    `json:"failures"`
	Rate     string `json:"rate"`
}

// upsertIssue creates or updates the GitHub issue that tracks one test result.
// In dry-run mode it only reports the action that would be taken.
func upsertIssue(ctx context.Context, gh *githubClient, write bool, r *result, classification string, ratePct int) error {
	title := issueTitle(r)
	existing, err := findIssue(ctx, gh, title)
	if err != nil {
		return err
	}

	body, err := buildIssueBody(ctx, gh, existing, r, classification, ratePct)
	if err != nil {
		return err
	}

	if !write {
		if existing != "" {
			slog.Info("dry-run would update issue", "issue", existing, "title", title)
		} else {
			slog.Info("dry-run would create issue", "title", title)
		}
		return nil
	}
	return writeIssue(ctx, gh, existing, title, body)
}

// issueTitle returns the managed GitHub issue title for a test result.
func issueTitle(r *result) string {
	leaf := r.Test[strings.LastIndex(r.Test, "/")+1:]
	return "Flaky test: " + leaf
}

// buildIssueBody loads prior issue history and renders the next body.
func buildIssueBody(ctx context.Context, gh *githubClient, issueNumber string, r *result, classification string, ratePct int) (string, error) {
	existingHistory, err := loadIssueHistory(ctx, gh, issueNumber)
	if err != nil {
		return "", err
	}
	history := updateHistory(existingHistory, r.Passes, r.Failures, strconv.Itoa(ratePct))
	return renderIssueBody(gh.repo, r, classification, ratePct, history)
}

// loadIssueHistory returns stored history for an existing issue.
// A missing issue number means the issue does not exist yet.
func loadIssueHistory(ctx context.Context, gh *githubClient, issueNumber string) (historyData, error) {
	if issueNumber == "" {
		return historyData{}, nil
	}
	owner, repo, err := gh.repoParts()
	if err != nil {
		return historyData{}, err
	}
	number, err := strconv.Atoi(issueNumber)
	if err != nil {
		return historyData{}, err
	}
	issue, _, err := gh.githubAPI().Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return historyData{}, err
	}
	return extractHistory(issue.GetBody()), nil
}

// writeIssue applies a rendered issue body to GitHub.
func writeIssue(ctx context.Context, gh *githubClient, existing, title, body string) error {
	if err := ensureLabel(ctx, gh); err != nil {
		return err
	}
	if existing != "" {
		return updateIssue(ctx, gh, existing, title, body)
	}
	return createIssue(ctx, gh, title, body)
}

// createIssue creates a new managed issue.
func createIssue(ctx context.Context, gh *githubClient, title, body string) error {
	slog.Info("creating issue", "title", title)
	owner, repo, err := gh.repoParts()
	if err != nil {
		return err
	}
	labels := []string{label}
	_, _, err = gh.githubAPI().Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title:  new(title),
		Body:   new(body),
		Labels: &labels,
	})
	return err
}

// updateIssue updates an existing managed issue and reopens it when necessary.
func updateIssue(ctx context.Context, gh *githubClient, issueNumber, title, body string) error {
	slog.Info("updating issue", "issue", issueNumber, "title", title)
	owner, repo, err := gh.repoParts()
	if err != nil {
		return err
	}
	number, err := strconv.Atoi(issueNumber)
	if err != nil {
		return err
	}
	issue, _, err := gh.githubAPI().Issues.Edit(ctx, owner, repo, number, &github.IssueRequest{Body: new(body)})
	if err != nil {
		return err
	}
	return reopenIssueIfClosed(ctx, gh, issue)
}

// reopenIssueIfClosed reopens a managed issue after its body is refreshed.
func reopenIssueIfClosed(ctx context.Context, gh *githubClient, issue *github.Issue) error {
	if !strings.EqualFold(issue.GetState(), "closed") {
		return nil
	}
	owner, repo, err := gh.repoParts()
	if err != nil {
		return err
	}
	slog.Info("reopening closed issue", "issue", issue.GetNumber())
	_, _, err = gh.githubAPI().Issues.Edit(ctx, owner, repo, issue.GetNumber(), &github.IssueRequest{State: new("open")})
	return err
}

// findIssue finds an existing managed issue with exactly the requested title.
func findIssue(ctx context.Context, gh *githubClient, title string) (string, error) {
	owner, repo, err := gh.repoParts()
	if err != nil {
		return "", err
	}
	for page := 1; page <= maxIssuePages; page++ {
		issues, _, err := gh.githubAPI().Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{
			State:  "all",
			Labels: []string{label},
			ListOptions: github.ListOptions{
				PerPage: 100,
				Page:    page,
			},
		})
		if err != nil {
			return "", err
		}
		if len(issues) == 0 {
			return "", nil
		}
		for _, issue := range issues {
			if !issue.IsPullRequest() && issue.GetTitle() == title {
				return strconv.Itoa(issue.GetNumber()), nil
			}
		}
	}
	return "", nil
}

// ensureLabel creates the tracking label when it is missing.
func ensureLabel(ctx context.Context, gh *githubClient) error {
	owner, repo, err := gh.repoParts()
	if err != nil {
		return err
	}
	if _, _, err := gh.githubAPI().Issues.GetLabel(ctx, owner, repo, label); err == nil {
		return nil
	} else {
		var ghErr *github.ErrorResponse
		if !errors.As(err, &ghErr) || ghErr.Response == nil || ghErr.Response.StatusCode != http.StatusNotFound {
			return err
		}
	}
	slog.Info("creating label", "label", label)
	_, _, err = gh.githubAPI().Issues.CreateLabel(ctx, owner, repo, &github.Label{
		Name:        new(label),
		Description: new("Flaky test detected by CI"),
		Color:       new("e11d48"),
	})
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusUnprocessableEntity {
		return nil
	}
	return err
}

// extractHistory reads the hidden JSON history block from an issue body.
func extractHistory(body string) historyData {
	prefix := "<!-- " + historyMarker + " "
	start := strings.Index(body, prefix)
	if start == -1 {
		return historyData{}
	}
	start += len(prefix)
	end := strings.Index(body[start:], " -->")
	if end == -1 {
		return historyData{}
	}
	var h historyData
	_ = json.Unmarshal([]byte(body[start:start+end]), &h)
	return h
}

// updateHistory inserts or replaces today's history row.
func updateHistory(existing historyData, passes, failures int, rate string) historyData {
	today := time.Now().UTC().Format("2006-01-02")
	if existing.FirstSeen == "" {
		return historyData{FirstSeen: today, History: []historyRow{{Date: today, Passes: passes, Failures: failures, Rate: rate}}}
	}
	row := historyRow{Date: today, Passes: passes, Failures: failures, Rate: rate}
	for i := range existing.History {
		if existing.History[i].Date == today {
			existing.History[i] = row
			return existing
		}
	}
	existing.History = append([]historyRow{row}, existing.History...)
	return existing
}

// renderIssueBody produces the full Markdown body for a managed issue.
func renderIssueBody(repo string, r *result, classification string, ratePct int, history historyData) (string, error) {
	statusIcon := "🔴"
	statusLabel := "Consistently failing (never passed in observed artifacts)"
	if classification == "flaky" {
		statusIcon = "🟡"
		statusLabel = "Flaky (passes and fails intermittently)"
	}

	runLinks := "_None recorded._\n"
	if len(r.FailedRunIDs) > 0 {
		ids := make([]int64, 0, len(r.FailedRunIDs))
		for id := range r.FailedRunIDs {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		var b strings.Builder
		for _, id := range ids {
			fmt.Fprintf(&b, "- [Run #%d](https://github.com/%s/actions/runs/%d)\n", id, repo, id)
		}
		runLinks = b.String()
	}

	var historyTable strings.Builder
	historyTable.WriteString("| Date | Passes | Failures | Rate |\n")
	historyTable.WriteString("| --- | --- | --- | --- |\n")
	for _, row := range history.History {
		fmt.Fprintf(&historyTable, "| %s | %d | %d | %s%% |\n", row.Date, row.Passes, row.Failures, row.Rate)
	}

	historyJSON, err := json.Marshal(history)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`## %s %s

**Test:** %s
**Package:** %s
**First seen:** %s

### Failed runs

%s### Failure rate history

%s

### How to investigate

1. Search for this test name in recent [CI workflow runs](https://github.com/%s/actions)
2. Check the test report artifacts for failure details
3. Look for timing-dependent assertions, resource leaks, or shared state

---
_This issue is automatically managed by the [flaky test tracker](https://github.com/%s/actions/workflows/flaky-test-tracker.yml)._

<!-- %s %s -->
`, statusIcon, statusLabel, "`"+r.Test+"`", "`"+r.Package+"`", history.FirstSeen, runLinks, historyTable.String(), repo, repo, historyMarker, historyJSON), nil
}
