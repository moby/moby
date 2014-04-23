package engine

import (
	"testing"
)

var globalTestID string

func mkJob(t *testing.T, name string, args ...string) *Job {
	return New().Job(name, args...)
}
