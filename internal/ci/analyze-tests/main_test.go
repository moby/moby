package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseConfigVerboseFlagAndEnv(t *testing.T) {
	t.Setenv("REPO", "owner/repo")
	t.Setenv("VERBOSE", "")

	cfg, err := parseConfig(context.Background(), []string{"--verbose"})
	assert.NilError(t, err)
	assert.Assert(t, cfg.verbose)

	t.Setenv("VERBOSE", "1")
	cfg, err = parseConfig(context.Background(), nil)
	assert.NilError(t, err)
	assert.Assert(t, cfg.verbose)
}

func TestParseConfigSummaryPath(t *testing.T) {
	t.Setenv("REPO", "owner/repo")
	t.Setenv("GITHUB_SUMMARY", "custom-summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", "step-summary.md")

	cfg, err := parseConfig(context.Background(), nil)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(cfg.summaryPath, "custom-summary.md"))
}

func TestWriteGitHubSummaryDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.md")
	err := writeGitHubSummary(path, []summaryRow{{
		Result: &result{
			Passes:   2,
			Failures: 1,
			Package:  "pkg",
			Test:     "TestFlaky",
		},
		Classification: "flaky",
		RatePct:        33,
	}}, false)
	assert.NilError(t, err)

	content, err := os.ReadFile(path)
	assert.NilError(t, err)
	summary := string(content)
	assert.Assert(t, strings.Contains(summary, "Issue updates: dry-run; no issues were created or edited."))
	assert.Assert(t, strings.Contains(summary, "| FLAKY | `TestFlaky` | `pkg` | 2 | 1 | 33% |"))
}

func TestWriteGitHubSummaryNoRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.md")
	err := writeGitHubSummary(path, nil, false)
	assert.NilError(t, err)

	content, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(content), "No qualifying flaky tests were detected."))
}

func TestParseReportFileReturnsRelevantEvents(t *testing.T) {
	report := strings.NewReader(strings.Join([]string{
		`{"Action":"run","Package":"pkg","Test":"TestIgnored"}`,
		`{"Action":"pass","Package":"pkg","Test":"TestPass"}`,
		`{"Action":"fail","Test":"TestFailUnknownPackage"}`,
		`{"Action":"fail","Package":"pkg"}`,
	}, "\n"))

	events, err := parseReportFile(report)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(events, 2))
	assert.Assert(t, is.DeepEqual(events[0], testEvent{
		Action:  "pass",
		Package: "pkg",
		Test:    "TestPass",
	}))
	assert.Assert(t, is.DeepEqual(events[1], testEvent{
		Action:  "fail",
		Package: "unknown",
		Test:    "TestFailUnknownPackage",
	}))
}

func TestParseReportFileReturnsMalformedJSONError(t *testing.T) {
	_, err := parseReportFile(strings.NewReader(`{"Action":"pass"`))
	assert.Assert(t, err != nil)
}

func TestGitHubErrorAddsUnauthorizedContext(t *testing.T) {
	endpoint, err := url.Parse("https://api.github.com/repos/owner/repo/actions/artifacts")
	assert.NilError(t, err)
	ghErr := &github.ErrorResponse{
		Response: &http.Response{
			Status:     "401 Unauthorized",
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Request:    &http.Request{URL: endpoint},
		},
	}
	ghErr.Response.Header.Set("X-GitHub-Request-Id", "abc123")

	err = (&githubClient{repo: "owner/repo"}).githubError("list workflow artifacts", ghErr)
	msg := err.Error()
	assert.Assert(t, strings.Contains(msg, "list workflow artifacts for repo owner/repo failed"))
	assert.Assert(t, strings.Contains(msg, "status=401 Unauthorized"))
	assert.Assert(t, strings.Contains(msg, "endpoint=https://api.github.com/repos/owner/repo/actions/artifacts"))
	assert.Assert(t, strings.Contains(msg, "authenticated=false"))
	assert.Assert(t, strings.Contains(msg, "github_request_id=abc123"))
	assert.Assert(t, strings.Contains(msg, "hint=check GH_TOKEN/GITHUB_TOKEN or gh auth token"))
}

func TestGitHubActionsHandlerWritesAnnotations(t *testing.T) {
	read, write, err := os.Pipe()
	assert.NilError(t, err)
	stdout := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = stdout })

	var logs bytes.Buffer
	handler := &GitHubActionsHandler{
		Handler: slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}),
	}
	logger := slog.New(handler)
	logger.Info("starting")
	logger.Warn("careful")
	logger.Error("failed")

	assert.NilError(t, write.Close())
	out, err := io.ReadAll(read)
	assert.NilError(t, err)
	assert.NilError(t, read.Close())

	assert.Assert(t, strings.Contains(string(out), "::notice::starting"))
	assert.Assert(t, strings.Contains(string(out), "::warning::careful"))
	assert.Assert(t, strings.Contains(string(out), "::error::failed"))
	assert.Assert(t, strings.Contains(logs.String(), "msg=starting"))
}

func TestGitHubActionsLoggingOmitsTime(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(&GitHubActionsHandler{
		Handler: slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}),
	})

	logger.Info("starting")

	assert.Assert(t, !strings.Contains(logs.String(), "time="))
	assert.Assert(t, strings.Contains(logs.String(), "level=INFO"))
	assert.Assert(t, strings.Contains(logs.String(), "msg=starting"))
}

func TestFilterArtifactsReportsSkipReasons(t *testing.T) {
	createdAt := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	artifacts := []artifact{
		newArtifact("test-results-unit", createdAt, false, 100),
		newArtifact("test-results-expired", createdAt, true, 101),
		newArtifact("test-results-missing-run", createdAt, false, 0),
		newArtifact("test-reports-unit", createdAt, false, 103),
	}

	selected, stats := filterArtifacts(artifacts)
	assert.Assert(t, is.Len(selected, 1))
	assert.Assert(t, is.Equal(selected[0].Name, "test-results-unit"))
	assert.Assert(t, is.DeepEqual(stats, artifactFilterStats{
		Seen:                 4,
		Selected:             1,
		Expired:              1,
		MissingWorkflowRunID: 1,
		WrongPrefix:          1,
	}))
}

func TestNormalizeArtifactsKeepsGitHubFields(t *testing.T) {
	createdAt := time.Date(2026, 5, 27, 11, 6, 27, 0, time.UTC)
	artifacts := normalizeArtifacts([]*github.Artifact{{
		ID:        github.Ptr[int64](123),
		Name:      new("test-results-unit"),
		CreatedAt: &github.Timestamp{Time: createdAt},
		Expired:   new(false),
		WorkflowRun: &github.ArtifactWorkflowRun{
			ID:               github.Ptr[int64](456),
			HeadBranch:       new("feature"),
			HeadRepositoryID: github.Ptr[int64](789),
		},
	}})

	assert.Assert(t, is.Len(artifacts, 1))
	a := artifacts[0]
	assert.Assert(t, is.Equal(a.ID, int64(123)))
	assert.Assert(t, is.Equal(a.Name, "test-results-unit"))
	assert.Assert(t, is.Equal(a.WorkflowRun.ID, int64(456)))
	assert.Assert(t, is.Equal(a.WorkflowRun.HeadBranch, "feature"))
	assert.Assert(t, is.Equal(a.WorkflowRun.HeadRepositoryID, int64(789)))
	assert.Assert(t, a.CreatedAt.Equal(createdAt))
}

func newArtifact(name string, createdAt time.Time, expired bool, runID int64) artifact {
	a := artifact{Name: name, CreatedAt: createdAt, Expired: expired}
	a.WorkflowRun.ID = runID
	return a
}

func TestFilterProblemResultsAggregatesAndDropsParents(t *testing.T) {
	results := map[string]*result{
		"pkg|TestParent": {
			Passes:        1,
			Failures:      1,
			FailedRunIDs:  map[int64]struct{}{101: {}},
			FailedPRHeads: map[string]struct{}{"1:pr-a": {}, "1:pr-b": {}},
			Package:       "pkg",
			Test:          "TestParent",
		},
		"pkg|TestParent/TestLeaf": {
			Passes:        2,
			Failures:      1,
			FailedRunIDs:  map[int64]struct{}{101: {}},
			FailedPRHeads: map[string]struct{}{"1:pr-a": {}, "1:pr-b": {}},
			Package:       "pkg",
			Test:          "TestParent/TestLeaf",
		},
		"pkg|TestPassing": {
			Passes:        3,
			FailedRunIDs:  map[int64]struct{}{},
			FailedPRHeads: map[string]struct{}{},
			Package:       "pkg",
			Test:          "TestPassing",
		},
	}

	problems := filterProblemResults(results)
	assert.Assert(t, is.Len(problems, 1))

	leaf := problems["pkg|TestParent/TestLeaf"]
	assert.Assert(t, leaf != nil)
	assert.Assert(t, is.Equal(leaf.Passes, 2))
	assert.Assert(t, is.Equal(leaf.Failures, 1))
	_, ok := leaf.FailedRunIDs[101]
	assert.Assert(t, ok)
}

func TestFilterProblemResultsRequiresMasterOrTwoPRs(t *testing.T) {
	results := map[string]*result{
		"pkg|TestSinglePR": {
			Passes:        1,
			Failures:      1,
			FailedRunIDs:  map[int64]struct{}{101: {}},
			FailedPRHeads: map[string]struct{}{"1:pr-a": {}},
			Package:       "pkg",
			Test:          "TestSinglePR",
		},
		"pkg|TestTwoPRs": {
			Passes:        1,
			Failures:      2,
			FailedRunIDs:  map[int64]struct{}{201: {}, 202: {}},
			FailedPRHeads: map[string]struct{}{"1:pr-a": {}, "1:pr-b": {}},
			Package:       "pkg",
			Test:          "TestTwoPRs",
		},
		"pkg|TestMaster": {
			Passes:         1,
			Failures:       1,
			FailedRunIDs:   map[int64]struct{}{301: {}},
			FailedPRHeads:  map[string]struct{}{},
			FailedOnMaster: true,
			Package:        "pkg",
			Test:           "TestMaster",
		},
		"pkg|TestOnlyFails": {
			Failures:      2,
			FailedRunIDs:  map[int64]struct{}{401: {}, 402: {}},
			FailedPRHeads: map[string]struct{}{"1:pr-a": {}, "1:pr-b": {}},
			Package:       "pkg",
			Test:          "TestOnlyFails",
		},
	}

	problems := filterProblemResults(results)
	assert.Assert(t, is.Nil(problems["pkg|TestSinglePR"]))
	assert.Assert(t, problems["pkg|TestTwoPRs"] != nil)
	assert.Assert(t, problems["pkg|TestMaster"] != nil)
	assert.Assert(t, is.Nil(problems["pkg|TestOnlyFails"]))
}

func TestUpdateHistoryReplacesToday(t *testing.T) {
	existing := updateHistory(historyData{}, 1, 2, "66")
	updated := updateHistory(existing, 3, 4, "57")

	assert.Assert(t, is.Len(updated.History, 1))
	row := updated.History[0]
	assert.Assert(t, is.Equal(row.Passes, 3))
	assert.Assert(t, is.Equal(row.Failures, 4))
	assert.Assert(t, is.Equal(row.Rate, "57"))
	assert.Assert(t, updated.FirstSeen != "")
}
