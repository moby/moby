package engine

import (
	"testing"
)

var globalTestID string

func newTestEngine(t *testing.T) *Engine {
	eng, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func mkJob(t *testing.T, name string, args ...string) *Job {
	return newTestEngine(t).Job(name, args...)
}
