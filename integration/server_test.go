package docker

import (
	"bytes"
	"testing"
	"time"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/engine"
)

func TestCreateNumberHostname(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := parseRun([]string{"-h", "web.0", unitTestImageID, "echo test"})
	if err != nil {
		t.Fatal(err)
	}

	createTestContainer(eng, config, t)
}

func TestCommit(t *testing.T) {
	eng := NewTestEngine(t)
	b := &builder.BuilderJob{Engine: eng}
	b.Install()
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := parseRun([]string{unitTestImageID, "/bin/cat"})
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("commit", id)
	job.Setenv("repo", "testrepo")
	job.Setenv("tag", "testtag")
	job.SetenvJson("config", config)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestMergeConfigOnCommit(t *testing.T) {
	eng := NewTestEngine(t)
	b := &builder.BuilderJob{Engine: eng}
	b.Install()
	runtime := mkDaemonFromEngine(eng, t)
	defer runtime.Nuke()

	container1, _, _ := mkContainer(runtime, []string{"-e", "FOO=bar", unitTestImageID, "echo test > /tmp/foo"}, t)
	defer runtime.Rm(container1)

	config, _, _, err := parseRun([]string{container1.ID, "cat /tmp/foo"})
	if err != nil {
		t.Error(err)
	}

	job := eng.Job("commit", container1.ID)
	job.Setenv("repo", "testrepo")
	job.Setenv("tag", "testtag")
	job.SetenvJson("config", config)
	var outputBuffer = bytes.NewBuffer(nil)
	job.Stdout.Add(outputBuffer)
	if err := job.Run(); err != nil {
		t.Error(err)
	}

	container2, _, _ := mkContainer(runtime, []string{engine.Tail(outputBuffer, 1)}, t)
	defer runtime.Rm(container2)

	job = eng.Job("container_inspect", container1.Name)
	baseContainer, _ := job.Stdout.AddEnv()
	if err := job.Run(); err != nil {
		t.Error(err)
	}

	job = eng.Job("container_inspect", container2.Name)
	commitContainer, _ := job.Stdout.AddEnv()
	if err := job.Run(); err != nil {
		t.Error(err)
	}

	baseConfig := baseContainer.GetSubEnv("Config")
	commitConfig := commitContainer.GetSubEnv("Config")

	if commitConfig.Get("Env") != baseConfig.Get("Env") {
		t.Fatalf("Env config in committed container should be %v, was %v",
			baseConfig.Get("Env"), commitConfig.Get("Env"))
	}

	if baseConfig.Get("Cmd") != "[\"echo test \\u003e /tmp/foo\"]" {
		t.Fatalf("Cmd in base container should be [\"echo test \\u003e /tmp/foo\"], was %s",
			baseConfig.Get("Cmd"))
	}

	if commitConfig.Get("Cmd") != "[\"cat /tmp/foo\"]" {
		t.Fatalf("Cmd in committed container should be [\"cat /tmp/foo\"], was %s",
			commitConfig.Get("Cmd"))
	}
}

func TestRestartKillWait(t *testing.T) {
	eng := NewTestEngine(t)
	runtime := mkDaemonFromEngine(eng, t)
	defer runtime.Nuke()

	config, hostConfig, _, err := parseRun([]string{"-i", unitTestImageID, "/bin/cat"})
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	containers, err := runtime.Containers(&daemon.ContainersConfig{All: true})

	if err != nil {
		t.Errorf("Error getting containers1: %q", err)
	}

	if len(containers) != 1 {
		t.Errorf("Expected 1 container, %v found", len(containers))
	}

	job := eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if err := runtime.ContainerKill(id, 0); err != nil {
		t.Fatal(err)
	}

	eng = newTestEngine(t, false, runtime.Config().Root)
	runtime = mkDaemonFromEngine(eng, t)

	containers, err = runtime.Containers(&daemon.ContainersConfig{All: true})

	if err != nil {
		t.Errorf("Error getting containers1: %q", err)
	}
	if len(containers) != 1 {
		t.Errorf("Expected 1 container, %v found", len(containers))
	}

	setTimeout(t, "Waiting on stopped container timedout", 5*time.Second, func() {
		job = eng.Job("wait", containers[0].ID)
		if err := job.Run(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunWithTooLowMemoryLimit(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	// Try to create a container with a memory limit of 1 byte less than the minimum allowed limit.
	job := eng.Job("create")
	job.Setenv("Image", unitTestImageID)
	job.Setenv("Memory", "524287")
	job.Setenv("CpuShares", "1000")
	job.SetenvList("Cmd", []string{"/bin/cat"})
	if err := job.Run(); err == nil {
		t.Errorf("Memory limit is smaller than the allowed limit. Container creation should've failed!")
	}
}

func TestImagesFilter(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	if err := eng.Job("tag", unitTestImageName, "utest", "tag1").Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("tag", unitTestImageName, "utest/docker", "tag2").Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("tag", unitTestImageName, "utest:5000/docker", "tag3").Run(); err != nil {
		t.Fatal(err)
	}

	images := getImages(eng, t, false, "utest*/*")

	if len(images[0].RepoTags) != 2 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "utest")

	if len(images[0].RepoTags) != 1 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "utest*")

	if len(images[0].RepoTags) != 1 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "*5000*/*")

	if len(images[0].RepoTags) != 1 {
		t.Fatal("incorrect number of matches returned")
	}
}
