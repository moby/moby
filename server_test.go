package docker

import (
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
	if err := srv.runtime.repositories.Set("utest/docker", "tag2", unitTestImageName, false); err != nil {
		t.Fatal(err)
	}

	images, err := srv.Images(false, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(images) != len(initialImages)+2 {
		t.Errorf("Expected %d images, %d found", len(initialImages)+2, len(images))
	}

	if _, err := srv.ImageDelete("utest/docker:tag2", true); err != nil {
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

func TestContainerTop(t *testing.T) {
	runtime := mkRuntime(t)
	srv := &Server{runtime: runtime}
	defer nuke(runtime)

	c, hostConfig := mkContainer(runtime, []string{"_", "/bin/sh", "-c", "sleep 2"}, t)
	defer runtime.Destroy(c)
	if err := c.Start(hostConfig); err != nil {
		t.Fatal(err)
	}

	// Give some time to the process to start
	c.WaitTimeout(500 * time.Millisecond)

	if !c.State.Running {
		t.Errorf("Container should be running")
	}
	procs, err := srv.ContainerTop(c.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(procs) != 2 {
		t.Fatalf("Expected 2 processes, found %d.", len(procs))
	}

	if procs[0].Cmd != "sh" && procs[0].Cmd != "busybox" {
		t.Fatalf("Expected `busybox` or `sh`, found %s.", procs[0].Cmd)
	}

	if procs[1].Cmd != "sh" && procs[1].Cmd != "busybox" {
		t.Fatalf("Expected `busybox` or `sh`, found %s.", procs[1].Cmd)
	}
}

func TestPools(t *testing.T) {
	runtime := mkRuntime(t)
	srv := &Server{
		runtime:     runtime,
		pullingPool: make(map[string]struct{}),
		pushingPool: make(map[string]struct{}),
	}
	defer nuke(runtime)

	err := srv.poolAdd("pull", "test1")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolAdd("pull", "test2")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolAdd("push", "test1")
	if err == nil || err.Error() != "pull test1 is already in progress" {
		t.Fatalf("Expected `pull test1 is already in progress`")
	}
	err = srv.poolAdd("pull", "test1")
	if err == nil || err.Error() != "pull test1 is already in progress" {
		t.Fatalf("Expected `pull test1 is already in progress`")
	}
	err = srv.poolAdd("wait", "test3")
	if err == nil || err.Error() != "Unkown pool type" {
		t.Fatalf("Expected `Unkown pool type`")
	}

	err = srv.poolRemove("pull", "test2")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolRemove("pull", "test2")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolRemove("pull", "test1")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolRemove("push", "test1")
	if err != nil {
		t.Fatal(err)
	}
	err = srv.poolRemove("wait", "test3")
	if err == nil || err.Error() != "Unkown pool type" {
		t.Fatalf("Expected `Unkown pool type`")
	}
}
