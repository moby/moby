package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

func main() {
	configureLogging()
	if err := run(context.Background(), os.Args[1:]); err != nil {
		slog.Error("analyzer failed", "error", err)
		os.Exit(1)
	}
}

// run executes the analyzer once.
// It fetches GitHub Actions test-result artifacts, aggregates pass/fail events,
// and creates or updates tracking issues for qualifying flaky tests.
func run(ctx context.Context, args []string) error {
	cfg, err := parseConfig(ctx, args)
	if err != nil {
		return err
	}

	if !cfg.writeIssues {
		slog.Info("dry-run mode", "hint", "pass --write-issues or set WRITE_ISSUES=1 to create/update issues")
	}

	slog.Info("repository selected", "repo", cfg.repo)
	slog.Info("fetching test result artifacts", "limit", maxArtifactPages*artifactPageSize)
	if cfg.verbose {
		slog.Info("verbose artifact diagnostics enabled")
	}

	gh := &githubClient{repo: cfg.repo, token: resolveToken(ctx), client: http.DefaultClient, verbose: cfg.verbose}
	artifacts, err := gh.fetchArtifacts(ctx)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		slog.Info("no test result artifacts found")
		return writeGitHubSummary(cfg.summaryPath, nil, cfg.writeIssues)
	}

	slog.Info("found test result artifacts", "count", len(artifacts))
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}

	results, err := aggregateArtifacts(ctx, gh, artifacts)
	if err != nil {
		return err
	}

	problems := filterProblemResults(results)
	if len(problems) == 0 {
		slog.Info("no test failures found")
		return writeGitHubSummary(cfg.summaryPath, nil, cfg.writeIssues)
	}

	slog.Info("tests with failures", "count", len(problems))

	flakyCount := 0
	failingCount := 0
	var summaryRows []summaryRow
	for _, key := range sortedKeys(problems) {
		r := problems[key]
		total := r.Passes + r.Failures
		ratePct := r.Failures * 100 / total
		classification := "failing"
		if r.Passes > 0 {
			classification = "flaky"
			flakyCount++
		} else {
			failingCount++
		}

		slog.Info("test failure summary", "classification", strings.ToUpper(classification), "test", r.Test, "failures", r.Failures, "total", total, "rate_percent", ratePct)
		summaryRows = append(summaryRows, summaryRow{Result: r, Classification: classification, RatePct: ratePct})
		if err := upsertIssue(ctx, gh, cfg.writeIssues, r, classification, ratePct); err != nil {
			return err
		}
	}

	slog.Info("done", "flaky", flakyCount, "consistently_failing", failingCount)
	return writeGitHubSummary(cfg.summaryPath, summaryRows, cfg.writeIssues)
}
