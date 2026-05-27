package main

import "strings"

// filterProblemResults returns tests that look flaky enough to track.
// A test must have at least one pass, at least one failure, and either fail on
// master or fail across at least two pull-request heads.
func filterProblemResults(results map[string]*result) map[string]*result {
	deduped := map[string]*result{}
	for _, r := range results {
		key := r.Package + "|" + r.Test
		d := deduped[key]
		if d == nil {
			d = &result{Package: r.Package, Test: r.Test, FailedRunIDs: map[int64]struct{}{}, FailedPRHeads: map[string]struct{}{}}
			deduped[key] = d
		}
		d.Passes += r.Passes
		d.Failures += r.Failures
		for runID := range r.FailedRunIDs {
			d.FailedRunIDs[runID] = struct{}{}
		}
		for prHead := range r.FailedPRHeads {
			d.FailedPRHeads[prHead] = struct{}{}
		}
		d.FailedOnMaster = d.FailedOnMaster || r.FailedOnMaster
	}

	problem := map[string]*result{}
	var tests []string
	for _, r := range deduped {
		if r.Failures > 0 {
			tests = append(tests, r.Test)
		}
	}
	for key, r := range deduped {
		if r.Failures == 0 || r.Passes == 0 || !hasQualifyingFailureSource(r) || hasFailingSubtest(r.Test, tests) {
			continue
		}
		problem[key] = r
	}
	return problem
}

// hasQualifyingFailureSource reports whether failures came from trusted enough
// sources to avoid filing issues for one-off pull-request breakage.
func hasQualifyingFailureSource(r *result) bool {
	return r.FailedOnMaster || len(r.FailedPRHeads) >= 2
}

// hasFailingSubtest reports whether test has a failing descendant subtest.
// Parent test failures are skipped when a more specific subtest also failed.
func hasFailingSubtest(test string, tests []string) bool {
	prefix := test + "/"
	for _, candidate := range tests {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}
