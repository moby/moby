package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/ioutils"
)

func TestRegister(t *testing.T) {
	if err := Register("dummy1", nil); err != nil {
		t.Fatal(err)
	}

	if err := Register("dummy1", nil); err == nil {
		t.Fatalf("Expecting error, got none")
	}
	// Register is global so let's cleanup to avoid conflicts
	defer unregister("dummy1")

	eng := New()

	//Should fail because global handlers are copied
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
	defer unregister("dummy2")
}

func TestJob(t *testing.T) {
	eng := New()
	job1 := eng.Job("dummy1", "--level=awesome")

	if job1.handler != nil {
		t.Fatalf("job1.handler should be empty")
	}

	h := func(j *Job) Status {
		j.Printf("%s\n", j.Name)
		return 42
	}

	eng.Register("dummy2", h)
	defer unregister("dummy2")
	job2 := eng.Job("dummy2", "--level=awesome")

	if job2.handler == nil {
		t.Fatalf("job2.handler shouldn't be nil")
	}

	if job2.handler(job2) != 42 {
		t.Fatalf("handler dummy2 was not found in job2")
	}
}

func TestEngineShutdown(t *testing.T) {
	eng := New()
	if eng.IsShutdown() {
		t.Fatalf("Engine should not show as shutdown")
	}
	eng.Shutdown()
	if !eng.IsShutdown() {
		t.Fatalf("Engine should show as shutdown")
	}
}

func TestEngineCommands(t *testing.T) {
	eng := New()
	handler := func(job *Job) Status { return StatusOK }
	eng.Register("foo", handler)
	eng.Register("bar", handler)
	eng.Register("echo", handler)
	eng.Register("die", handler)
	var output bytes.Buffer
	commands := eng.Job("commands")
	commands.Stdout.Add(&output)
	commands.Run()
	expected := "bar\ncommands\ndie\necho\nfoo\n"
	if result := output.String(); result != expected {
		t.Fatalf("Unexpected output:\nExpected = %v\nResult   = %v\n", expected, result)
	}
}

func TestEngineString(t *testing.T) {
	eng1 := New()
	eng2 := New()
	s1 := eng1.String()
	s2 := eng2.String()
	if eng1 == eng2 {
		t.Fatalf("Different engines should have different names (%v == %v)", s1, s2)
	}
}

func TestParseJob(t *testing.T) {
	eng := New()
	// Verify that the resulting job calls to the right place
	var called bool
	eng.Register("echo", func(job *Job) Status {
		called = true
		return StatusOK
	})
	input := "echo DEBUG=1 hello world VERBOSITY=42"
	job, err := eng.ParseJob(input)
	if err != nil {
		t.Fatal(err)
	}
	if job.Name != "echo" {
		t.Fatalf("Invalid job name: %v", job.Name)
	}
	if strings.Join(job.Args, ":::") != "hello:::world" {
		t.Fatalf("Invalid job args: %v", job.Args)
	}
	if job.Env().Get("DEBUG") != "1" {
		t.Fatalf("Invalid job env: %v", job.Env)
	}
	if job.Env().Get("VERBOSITY") != "42" {
		t.Fatalf("Invalid job env: %v", job.Env)
	}
	if len(job.Env().Map()) != 2 {
		t.Fatalf("Invalid job env: %v", job.Env)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatalf("Job was not called")
	}
}

func TestCatchallEmptyName(t *testing.T) {
	eng := New()
	var called bool
	eng.RegisterCatchall(func(job *Job) Status {
		called = true
		return StatusOK
	})
	err := eng.Job("").Run()
	if err == nil {
		t.Fatalf("Engine.Job(\"\").Run() should return an error")
	}
	if called {
		t.Fatalf("Engine.Job(\"\").Run() should return an error")
	}
}

// Ensure that a job within a job both using the same underlying standard
// output writer does not close the output of the outer job when the inner
// job's stdout is wrapped with a NopCloser. When not wrapped, it should
// close the outer job's output.
func TestNestedJobSharedOutput(t *testing.T) {
	var (
		outerHandler Handler
		innerHandler Handler
		wrapOutput   bool
	)

	outerHandler = func(job *Job) Status {
		job.Stdout.Write([]byte("outer1"))

		innerJob := job.Eng.Job("innerJob")

		if wrapOutput {
			innerJob.Stdout.Add(ioutils.NopWriteCloser(job.Stdout))
		} else {
			innerJob.Stdout.Add(job.Stdout)
		}

		if err := innerJob.Run(); err != nil {
			t.Fatal(err)
		}

		// If wrapOutput was *false* this write will do nothing.
		// FIXME (jlhawn): It should cause an error to write to
		// closed output.
		job.Stdout.Write([]byte(" outer2"))

		return StatusOK
	}

	innerHandler = func(job *Job) Status {
		job.Stdout.Write([]byte(" inner"))

		return StatusOK
	}

	eng := New()
	eng.Register("outerJob", outerHandler)
	eng.Register("innerJob", innerHandler)

	// wrapOutput starts *false* so the expected
	// output of running the outer job will be:
	//
	//     "outer1 inner"
	//
	outBuf := new(bytes.Buffer)
	outerJob := eng.Job("outerJob")
	outerJob.Stdout.Add(outBuf)

	if err := outerJob.Run(); err != nil {
		t.Fatal(err)
	}

	expectedOutput := "outer1 inner"
	if outBuf.String() != expectedOutput {
		t.Fatalf("expected job output to be %q, got %q", expectedOutput, outBuf.String())
	}

	// Set wrapOutput to true so that the expected
	// output of running the outer job will be:
	//
	//     "outer1 inner outer2"
	//
	wrapOutput = true
	outBuf.Reset()
	outerJob = eng.Job("outerJob")
	outerJob.Stdout.Add(outBuf)

	if err := outerJob.Run(); err != nil {
		t.Fatal(err)
	}

	expectedOutput = "outer1 inner outer2"
	if outBuf.String() != expectedOutput {
		t.Fatalf("expected job output to be %q, got %q", expectedOutput, outBuf.String())
	}
}
