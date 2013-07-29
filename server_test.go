package docker

import (
	"github.com/dotcloud/docker/utils"
	"testing"
	"time"
)

func TestContainerTagImageDelete(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	initialImages, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := srv.runtime.repositories.Set("utest", "tag1", unitTestImageName, false); err != nil {
		t.Fatal(err)
	}
	if err := srv.runtime.repositories.Set("utest:5000/docker", "tag2", unitTestImageName, false); err != nil {
		t.Fatal(err)
	}

	images, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages)+2 {
		t.Errorf("Expected %d images, %d found", len(initialImages)+2, len(images))
	}

	if _, err := srv.ImageDelete("utest:5000/docker:tag2", true); err != nil {
		t.Fatal(err)
	}

	images, err = srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages)+1 {
		t.Errorf("Expected %d images, %d found", len(initialImages)+1, len(images))
	}

	if _, err := srv.ImageDelete("utest:tag1", true); err != nil {
		t.Fatal(err)
	}

	images, err = srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages) {
		t.Errorf("Expected %d image, %d found", len(initialImages), len(images))
	}
}

func TestCreateRm(t *testing.T) {
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	config, _, _, err := ParseRun([]string{GetTestImage(runtime).ID, "echo test"}, nil)
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
	runtime := mkRuntime(t)
	defer nuke(runtime)

	srv := &Server{runtime: runtime}

	config, hostConfig, _, err := ParseRun([]string{GetTestImage(runtime).ID, "/bin/cat"}, nil)
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

	err = srv.ContainerStart(id, hostConfig)
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

	err = srv.ContainerStart(id, hostConfig)
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

func TestRunWithTooLowMemoryLimit(t *testing.T) {
	var err error
	runtime := mkRuntime(t)
	srv := &Server{runtime: runtime}
	defer nuke(runtime)
	// Try to create a container with a memory limit of 1 byte less than the minimum allowed limit.
	_, err = srv.ContainerCreate(
		&Config{
			Image:     GetTestImage(runtime).ID,
			Memory:    524287,
			CpuShares: 1000,
			Cmd:       []string{"/bin/cat"},
		},
	)
	if err == nil {
		t.Errorf("Memory limit is smaller than the allowed limit. Container creation should've failed!")
	}

}

func TestLogEvent(t *testing.T) {
	runtime := mkRuntime(t)
	srv := &Server{
		runtime:   runtime,
		events:    make([]utils.JSONMessage, 0, 64),
		listeners: make(map[string]chan utils.JSONMessage),
	}

	srv.LogEvent("fakeaction", "fakeid")

	listener := make(chan utils.JSONMessage)
	srv.Lock()
	srv.listeners["test"] = listener
	srv.Unlock()

	srv.LogEvent("fakeaction2", "fakeid")

	if len(srv.events) != 2 {
		t.Fatalf("Expected 2 events, found %d", len(srv.events))
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		srv.LogEvent("fakeaction3", "fakeid")
		time.Sleep(200 * time.Millisecond)
		srv.LogEvent("fakeaction4", "fakeid")
	}()

	setTimeout(t, "Listening for events timed out", 2*time.Second, func() {
		for i := 2; i < 4; i++ {
			event := <-listener
			if event != srv.events[i] {
				t.Fatalf("Event received it different than expected")
			}
		}
	})

}
