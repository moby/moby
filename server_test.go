package docker

import (
	"testing"
)

func TestCreateRm(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	config, _, err := ParseRun([]string{GetTestImage(runtime).Id, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id, err := srv.ContainerCreate(config)
	if err != nil {
		t.Fatal(err)
	}

	if len(runtime.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(runtime.List()))
	}

	if err = srv.ContainerDestroy(id, true); err != nil {
		t.Fatal(err)
	}

	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 container, %v found", len(runtime.List()))
	}

}

func TestCreateStartRestartStopStartKillRm(t *testing.T) {
	runtime, err := newTestRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	config, _, err := ParseRun([]string{GetTestImage(runtime).Id, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id, err := srv.ContainerCreate(config)
	if err != nil {
		t.Fatal(err)
	}

	if len(runtime.List()) != 1 {
		t.Errorf("Expected 1 container, %v found", len(runtime.List()))
	}

	err = srv.ContainerStart(id)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.ContainerRestart(id, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.ContainerStop(id, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.ContainerStart(id)
	if err != nil {
		t.Fatal(err)
	}

	err = srv.ContainerKill(id)
	if err != nil {
		t.Fatal(err)
	}

	// FIXME: this failed once with a race condition ("Unable to remove filesystem for xxx: directory not empty")
	if err = srv.ContainerDestroy(id, true); err != nil {
		t.Fatal(err)
	}

	if len(runtime.List()) != 0 {
		t.Errorf("Expected 0 container, %v found", len(runtime.List()))
	}

}
