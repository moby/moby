package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	cacheDir             = ".flaky-cache"
	defaultBranch        = "master"
	maxParallelArtifacts = 8
)

// testEvent is one pass or fail record for a single Go test.
type testEvent struct {
	Action  string
	Package string
	Test    string
	RunID   int64
	PRHead  string
	Master  bool
}

// result aggregates all observed pass and fail events for one test.
type result struct {
	Passes         int
	Failures       int
	FailedRunIDs   map[int64]struct{}
	FailedPRHeads  map[string]struct{}
	FailedOnMaster bool
	Package        string
	Test           string
}

// aggregateArtifacts downloads, parses, and aggregates test events from all
// selected artifacts.
func aggregateArtifacts(ctx context.Context, gh *githubClient, artifacts []artifact) (map[string]*result, error) {
	jobs := make(chan int)
	events := make(chan []testEvent)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for range maxParallelArtifacts {
		wg.Go(func() {
			for i := range jobs {
				a := artifacts[i]
				slog.Info("processing artifact", "artifact", a.Name, "run_id", a.WorkflowRun.ID, "index", i+1, "total", len(artifacts))
				es, err := processArtifact(ctx, gh, a)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				if len(es) > 0 {
					events <- es
				}
			}
		})
	}

	go func() {
		for i := range artifacts {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(events)
	}()

	results := map[string]*result{}
	for es := range events {
		for _, e := range es {
			key := e.Package + "|" + e.Test
			r := results[key]
			if r == nil {
				r = &result{Package: e.Package, Test: e.Test, FailedRunIDs: map[int64]struct{}{}, FailedPRHeads: map[string]struct{}{}}
				results[key] = r
			}
			switch e.Action {
			case "pass":
				r.Passes++
			case "fail":
				r.Failures++
				r.FailedRunIDs[e.RunID] = struct{}{}
				if e.Master {
					r.FailedOnMaster = true
				} else if e.PRHead != "" {
					r.FailedPRHeads[e.PRHead] = struct{}{}
				}
			}
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// processArtifact ensures an artifact is available locally and annotates parsed
// test events with workflow-run metadata.
func processArtifact(ctx context.Context, gh *githubClient, a artifact) ([]testEvent, error) {
	dest := filepath.Join(cacheDir, strconv.FormatInt(a.WorkflowRun.ID, 10), a.Name)
	if _, err := os.Stat(dest); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := downloadAndExtract(ctx, gh, a, dest); err != nil {
			_ = os.RemoveAll(dest)
			return nil, err
		}
	}
	events, err := parseReports(dest)
	if err != nil {
		return nil, err
	}
	for i := range events {
		events[i].RunID = a.WorkflowRun.ID
		events[i].Master = a.WorkflowRun.HeadBranch == defaultBranch
		events[i].PRHead = prHead(a)
	}
	return events, nil
}

// prHead returns a stable pull-request head identifier for cross-fork branch
// names, or an empty string for master and unknown heads.
func prHead(a artifact) string {
	if a.WorkflowRun.HeadBranch == "" || a.WorkflowRun.HeadBranch == defaultBranch {
		return ""
	}
	return fmt.Sprintf("%d:%s", a.WorkflowRun.HeadRepositoryID, a.WorkflowRun.HeadBranch)
}
