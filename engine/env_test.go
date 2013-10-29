package engine

import (
	"testing"
)

func TestNewJob(t *testing.T) {
	job := mkJob(t, "dummy", "--level=awesome")
	if job.Name != "dummy" {
		t.Fatalf("Wrong job name: %s", job.Name)
	}
	if len(job.Args) != 1 {
		t.Fatalf("Wrong number of job arguments: %d", len(job.Args))
	}
	if job.Args[0] != "--level=awesome" {
		t.Fatalf("Wrong job arguments: %s", job.Args[0])
	}
}

func TestSetenv(t *testing.T) {
	job := mkJob(t, "dummy")
	job.Setenv("foo", "bar")
	if val := job.Getenv("foo"); val != "bar" {
		t.Fatalf("Getenv returns incorrect value: %s", val)
	}
	if val := job.Getenv("nonexistent"); val != "" {
		t.Fatalf("Getenv returns incorrect value: %s", val)
	}
}
