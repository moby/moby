package main

import (
	"fmt"
	"os"
	"strings"
)

// summaryRow is one flaky-test detection result for the GitHub Actions summary.
type summaryRow struct {
	Result         *result
	Classification string
	RatePct        int
}

// writeGitHubSummary appends the analyzer result to the GitHub Actions summary.
func writeGitHubSummary(path string, rows []summaryRow, writeIssues bool) error {
	if path == "" {
		return nil
	}

	var b strings.Builder
	b.WriteString("## Flaky test detection\n\n")
	if writeIssues {
		b.WriteString("Issue updates: enabled.\n\n")
	} else {
		b.WriteString("Issue updates: dry-run; no issues were created or edited.\n\n")
	}

	if len(rows) == 0 {
		b.WriteString("No qualifying flaky tests were detected.\n")
		return appendSummary(path, b.String())
	}

	b.WriteString("| Classification | Test | Package | Passes | Failures | Failure rate |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: |\n")
	for _, row := range rows {
		r := row.Result
		fmt.Fprintf(&b, "| %s | `%s` | `%s` | %d | %d | %d%% |\n",
			strings.ToUpper(row.Classification), r.Test, r.Package, r.Passes, r.Failures, row.RatePct)
	}

	return appendSummary(path, b.String())
}

func appendSummary(path, content string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
