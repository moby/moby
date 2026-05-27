package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// configureLogging installs the process-wide logger used by the analyzer.
func configureLogging() {
	githubActions := os.Getenv("GITHUB_ACTIONS") == "true"
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	if githubActions {
		handler = &GitHubActionsHandler{Handler: handler}
	}
	slog.SetDefault(slog.New(handler))
}

// GitHubActionsHandler emits GitHub Actions workflow annotations for log records.
type GitHubActionsHandler struct {
	slog.Handler
}

// Handle writes a workflow annotation before delegating to the wrapped handler.
func (h *GitHubActionsHandler) Handle(ctx context.Context, r slog.Record) error {
	level := r.Level
	msg := r.Message

	switch {
	case level >= slog.LevelError:
		fmt.Printf("::error::%s\n", msg)
	case level >= slog.LevelWarn:
		fmt.Printf("::warning::%s\n", msg)
	default:
		fmt.Printf("::notice::%s\n", msg)
	}

	r.Time = time.Time{}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs preserves GitHub Actions annotations for derived loggers.
func (h *GitHubActionsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &GitHubActionsHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup preserves GitHub Actions annotations for derived loggers.
func (h *GitHubActionsHandler) WithGroup(name string) slog.Handler {
	return &GitHubActionsHandler{Handler: h.Handler.WithGroup(name)}
}

// sortedKeys returns the map keys in deterministic string order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// runCommand executes a subprocess and returns stdout.
// Stderr is included in returned errors to make CI failures actionable.
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// firstNonEmpty returns the first non-empty value in order.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
