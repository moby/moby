package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// config contains runtime options resolved from flags, environment variables,
// and GitHub CLI defaults.
type config struct {
	repo        string
	writeIssues bool
	verbose     bool
	summaryPath string
}

// parseConfig resolves command-line flags and environment variables.
// REPO overrides auto-detection through gh, WRITE_ISSUES=1 enables mutation,
// and VERBOSE=1 enables artifact discovery diagnostics.
func parseConfig(ctx context.Context, args []string) (config, error) {
	fs := flag.NewFlagSet("analyze-tests", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	writeIssues := fs.Bool("write-issues", false, "create/update GitHub issues")
	verbose := fs.Bool("verbose", false, "print artifact discovery diagnostics")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("unknown argument: %s", fs.Arg(0))
	}

	repo := os.Getenv("REPO")
	if repo == "" {
		var err error
		repo, err = resolveRepo(ctx)
		if err != nil {
			return config{}, err
		}
	}

	return config{
		repo:        repo,
		writeIssues: *writeIssues || os.Getenv("WRITE_ISSUES") == "1",
		verbose:     *verbose || os.Getenv("VERBOSE") == "1",
		summaryPath: firstNonEmpty(os.Getenv("GITHUB_SUMMARY"), os.Getenv("GITHUB_STEP_SUMMARY")),
	}, nil
}

// resolveRepo asks gh for the current repository in owner/name form.
func resolveRepo(ctx context.Context) (string, error) {
	out, err := runCommand(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner")
	if err != nil {
		return "", errors.New("could not detect repo; set REPO= explicitly")
	}
	return strings.TrimSpace(out), nil
}

// resolveToken returns a GitHub API token from environment or gh auth.
// The analyzer can still run unauthenticated, but API rate limits are lower.
func resolveToken(ctx context.Context) string {
	if token := firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	out, err := runCommand(ctx, "gh", "auth", "token")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
