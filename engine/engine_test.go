package engine

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
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

	eng := newTestEngine(t)

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
	defer unregister("dummy2")
	job2 := eng.Job("dummy2", "--level=awesome")

	if job2.handler == nil {
		t.Fatalf("job2.handler shouldn't be nil")
	}

	if job2.handler(job2) != 42 {
		t.Fatalf("handler dummy2 was not found in job2")
	}
}

func TestEngineCommands(t *testing.T) {
	eng := newTestEngine(t)
	defer os.RemoveAll(eng.Root())
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

func TestEngineRoot(t *testing.T) {
	tmp, err := ioutil.TempDir("", "docker-test-TestEngineCreateDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	// We expect Root to resolve to an absolute path.
	// FIXME: this should not be necessary.
	// Until the above FIXME is implemented, let's check for the
	// current behavior.
	tmp, err = filepath.EvalSymlinks(tmp)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err = filepath.Abs(tmp)
	if err != nil {
		t.Fatal(err)
	}
	dir := path.Join(tmp, "dir")
	eng, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if !st.IsDir() {
		t.Fatalf("engine.New() created something other than a directory at %s", dir)
	}
	if r := eng.Root(); r != dir {
		t.Fatalf("Expected: %v\nReceived: %v", dir, r)
	}
}

func TestEngineString(t *testing.T) {
	eng1 := newTestEngine(t)
	defer os.RemoveAll(eng1.Root())
	eng2 := newTestEngine(t)
	defer os.RemoveAll(eng2.Root())
	s1 := eng1.String()
	s2 := eng2.String()
	if eng1 == eng2 {
		t.Fatalf("Different engines should have different names (%v == %v)", s1, s2)
	}
}

func TestEngineLogf(t *testing.T) {
	eng := newTestEngine(t)
	defer os.RemoveAll(eng.Root())
	input := "Test log line"
	if n, err := eng.Logf("%s\n", input); err != nil {
		t.Fatal(err)
	} else if n < len(input) {
		t.Fatalf("Test: Logf() should print at least as much as the input\ninput=%d\nprinted=%d", len(input), n)
	}
}
