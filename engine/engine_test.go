package engine

import (
	"testing"
)

func TestRegister(t *testing.T) {
	if err := Register("dummy1", nil); err != nil {
		t.Fatal(err)
	}

	if err := Register("dummy1", nil); err == nil {
		t.Fatalf("Expecting error, got none")
	}

	eng := newTestEngine(t)

	//Should fail because globan handlers are copied
	//at the engine creation
	if err := eng.Register("dummy1", nil); err == nil {
		t.Fatalf("Expecting error, got none")
	}

	if err := eng.Register("dummy2", nil); err != nil {
		t.Fatal(err)
	}

	if err := eng.Register("dummy2", nil); err == nil {
		t.Fatalf("Expecting error, got none")
	}
}

func TestJob(t *testing.T) {
	eng := newTestEngine(t)
	job1 := eng.Job("dummy1", "--level=awesome")

	if job1.handler != nil {
		t.Fatalf("job1.handler should be empty")
	}

	h := func(j *Job) Status {
		j.Printf("%s\n", j.Name)
		return 42
	}

	eng.Register("dummy2", h)
	job2 := eng.Job("dummy2", "--level=awesome")

	if job2.handler == nil {
		t.Fatalf("job2.handler shouldn't be nil")
	}

	if job2.handler(job2) != 42 {
		t.Fatalf("handler dummy2 was not found in job2")
	}
}
