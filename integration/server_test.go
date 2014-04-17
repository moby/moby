package docker

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/runconfig"
	"github.com/dotcloud/docker/server"
	"strings"
	"testing"
	"time"
)

func TestCreateRm(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := runconfig.Parse([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	job = eng.Job("container_delete", id)
	job.SetenvBool("removeVolume", true)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 0 {
		t.Errorf("Expected 0 container, %v found", len(outs.Data))
	}

}

func TestCreateNumberHostname(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := runconfig.Parse([]string{"-h", "web.0", unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	createTestContainer(eng, config, t)
}

func TestCreateNumberUsername(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := runconfig.Parse([]string{"-u", "1002", unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	createTestContainer(eng, config, t)
}

func TestCreateRmVolumes(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, hostConfig, _, err := runconfig.Parse([]string{"-v", "/srv", unitTestImageID, "echo", "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	job = eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("stop", id)
	job.SetenvInt("t", 1)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("container_delete", id)
	job.SetenvBool("removeVolume", true)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 0 {
		t.Errorf("Expected 0 container, %v found", len(outs.Data))
	}
}

func TestCreateRmRunning(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, hostConfig, _, err := runconfig.Parse([]string{"--name", "foo", unitTestImageID, "sleep 300"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("containers")
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	// Test cannot remove running container
	job = eng.Job("container_delete", id)
	job.SetenvBool("forceRemove", false)
	if err := job.Run(); err == nil {
		t.Fatal("Expected container delete to fail")
	}

	job = eng.Job("containers")
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	// Test can force removal of running container
	job = eng.Job("container_delete", id)
	job.SetenvBool("forceRemove", true)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 0 {
		t.Errorf("Expected 0 container, %v found", len(outs.Data))
	}
}

func TestCommit(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, _, _, err := runconfig.Parse([]string{unitTestImageID, "/bin/cat"}, nil)
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
	runtime := mkDaemonFromEngine(eng, t)
	defer runtime.Nuke()

	container1, _, _ := mkContainer(runtime, []string{"-e", "FOO=bar", unitTestImageID, "echo test > /tmp/foo"}, t)
	defer runtime.Destroy(container1)

	config, _, _, err := runconfig.Parse([]string{container1.ID, "cat /tmp/foo"}, nil)
	if err != nil {
		t.Error(err)
	}

	job := eng.Job("commit", container1.ID)
	job.Setenv("repo", "testrepo")
	job.Setenv("tag", "testtag")
	job.SetenvJson("config", config)
	var newId string
	job.Stdout.AddString(&newId)
	if err := job.Run(); err != nil {
		t.Error(err)
	}

	container2, _, _ := mkContainer(runtime, []string{newId}, t)
	defer runtime.Destroy(container2)

	job = eng.Job("inspect", container1.Name, "container")
	baseContainer, _ := job.Stdout.AddEnv()
	if err := job.Run(); err != nil {
		t.Error(err)
	}

	job = eng.Job("inspect", container2.Name, "container")
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
	srv := mkServerFromEngine(eng, t)
	runtime := mkDaemonFromEngine(eng, t)
	defer runtime.Nuke()

	config, hostConfig, _, err := runconfig.Parse([]string{"-i", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	job = eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	job = eng.Job("kill", id)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	eng = newTestEngine(t, false, eng.Root())
	srv = mkServerFromEngine(eng, t)

	job = srv.Eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	setTimeout(t, "Waiting on stopped container timedout", 5*time.Second, func() {
		job = srv.Eng.Job("wait", outs.Data[0].Get("Id"))
		var statusStr string
		job.Stdout.AddString(&statusStr)
		if err := job.Run(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCreateStartRestartStopStartKillRm(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config, hostConfig, _, err := runconfig.Parse([]string{"-i", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := srv.Eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Errorf("Expected 1 container, %v found", len(outs.Data))
	}

	job = eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("restart", id)
	job.SetenvInt("t", 15)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("stop", id)
	job.SetenvInt("t", 15)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("kill", id).Run(); err != nil {
		t.Fatal(err)
	}

	// FIXME: this failed once with a race condition ("Unable to remove filesystem for xxx: directory not empty")
	job = eng.Job("container_delete", id)
	job.SetenvBool("removeVolume", true)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	job = srv.Eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err = job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 0 {
		t.Errorf("Expected 0 container, %v found", len(outs.Data))
	}
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
	var id string
	job.Stdout.AddString(&id)
	if err := job.Run(); err == nil {
		t.Errorf("Memory limit is smaller than the allowed limit. Container creation should've failed!")
	}
}

func TestRmi(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	initialImages := getAllImages(eng, t)

	config, hostConfig, _, err := runconfig.Parse([]string{unitTestImageID, "echo", "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	containerID := createTestContainer(eng, config, t)

	//To remove
	job := eng.Job("start", containerID)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("wait", containerID).Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("commit", containerID)
	job.Setenv("repo", "test")
	var imageID string
	job.Stdout.AddString(&imageID)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("tag", imageID, "test", "0.1").Run(); err != nil {
		t.Fatal(err)
	}

	containerID = createTestContainer(eng, config, t)

	//To remove
	job = eng.Job("start", containerID)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if err := eng.Job("wait", containerID).Run(); err != nil {
		t.Fatal(err)
	}

	job = eng.Job("commit", containerID)
	job.Setenv("repo", "test")
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	images := getAllImages(eng, t)

	if images.Len()-initialImages.Len() != 2 {
		t.Fatalf("Expected 2 new images, found %d.", images.Len()-initialImages.Len())
	}

	if err = srv.DeleteImage(imageID, engine.NewTable("", 0), true, false, false); err != nil {
		t.Fatal(err)
	}

	images = getAllImages(eng, t)

	if images.Len()-initialImages.Len() != 1 {
		t.Fatalf("Expected 1 new image, found %d.", images.Len()-initialImages.Len())
	}

	for _, image := range images.Data {
		if strings.Contains(unitTestImageID, image.Get("Id")) {
			continue
		}
		if image.GetList("RepoTags")[0] == "<none>:<none>" {
			t.Fatalf("Expected tagged image, got untagged one.")
		}
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

	if len(images.Data[0].GetList("RepoTags")) != 2 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "utest")

	if len(images.Data[0].GetList("RepoTags")) != 1 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "utest*")

	if len(images.Data[0].GetList("RepoTags")) != 1 {
		t.Fatal("incorrect number of matches returned")
	}

	images = getImages(eng, t, false, "*5000*/*")

	if len(images.Data[0].GetList("RepoTags")) != 1 {
		t.Fatal("incorrect number of matches returned")
	}
}

// FIXE: 'insert' is deprecated and should be removed in a future version.
func TestImageInsert(t *testing.T) {
	eng := NewTestEngine(t)
	defer mkDaemonFromEngine(eng, t).Nuke()
	srv := mkServerFromEngine(eng, t)

	// bad image name fails
	if err := srv.Eng.Job("insert", "foo", "https://www.docker.io/static/img/docker-top-logo.png", "/foo").Run(); err == nil {
		t.Fatal("expected an error and got none")
	}

	// bad url fails
	if err := srv.Eng.Job("insert", unitTestImageID, "http://bad_host_name_that_will_totally_fail.com/", "/foo").Run(); err == nil {
		t.Fatal("expected an error and got none")
	}

	// success returns nil
	if err := srv.Eng.Job("insert", unitTestImageID, "https://www.docker.io/static/img/docker-top-logo.png", "/foo").Run(); err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}
}

func TestListContainers(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	defer mkDaemonFromEngine(eng, t).Nuke()

	config := runconfig.Config{
		Image:     unitTestImageID,
		Cmd:       []string{"/bin/sh", "-c", "cat"},
		OpenStdin: true,
	}

	firstID := createTestContainer(eng, &config, t)
	secondID := createTestContainer(eng, &config, t)
	thirdID := createTestContainer(eng, &config, t)
	fourthID := createTestContainer(eng, &config, t)
	defer func() {
		containerKill(eng, firstID, t)
		containerKill(eng, secondID, t)
		containerKill(eng, fourthID, t)
		containerWait(eng, firstID, t)
		containerWait(eng, secondID, t)
		containerWait(eng, fourthID, t)
	}()

	startContainer(eng, firstID, t)
	startContainer(eng, secondID, t)
	startContainer(eng, fourthID, t)

	// all
	if !assertContainerList(srv, true, -1, "", "", []string{fourthID, thirdID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// running
	if !assertContainerList(srv, false, -1, "", "", []string{fourthID, secondID, firstID}) {
		t.Error("Container list is not in the correct order")
	}

	// from here 'all' flag is ignored

	// limit
	expected := []string{fourthID, thirdID}
	if !assertContainerList(srv, true, 2, "", "", expected) ||
		!assertContainerList(srv, false, 2, "", "", expected) {
		t.Error("Container list is not in the correct order")
	}

	// since
	expected = []string{fourthID, thirdID, secondID}
	if !assertContainerList(srv, true, -1, firstID, "", expected) ||
		!assertContainerList(srv, false, -1, firstID, "", expected) {
		t.Error("Container list is not in the correct order")
	}

	// before
	expected = []string{secondID, firstID}
	if !assertContainerList(srv, true, -1, "", thirdID, expected) ||
		!assertContainerList(srv, false, -1, "", thirdID, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before
	expected = []string{thirdID, secondID}
	if !assertContainerList(srv, true, -1, firstID, fourthID, expected) ||
		!assertContainerList(srv, false, -1, firstID, fourthID, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & limit
	expected = []string{fourthID, thirdID}
	if !assertContainerList(srv, true, 2, firstID, "", expected) ||
		!assertContainerList(srv, false, 2, firstID, "", expected) {
		t.Error("Container list is not in the correct order")
	}

	// before & limit
	expected = []string{thirdID}
	if !assertContainerList(srv, true, 1, "", fourthID, expected) ||
		!assertContainerList(srv, false, 1, "", fourthID, expected) {
		t.Error("Container list is not in the correct order")
	}

	// since & before & limit
	expected = []string{thirdID}
	if !assertContainerList(srv, true, 1, firstID, fourthID, expected) ||
		!assertContainerList(srv, false, 1, firstID, fourthID, expected) {
		t.Error("Container list is not in the correct order")
	}
}

func assertContainerList(srv *server.Server, all bool, limit int, since, before string, expected []string) bool {
	job := srv.Eng.Job("containers")
	job.SetenvBool("all", all)
	job.SetenvInt("limit", limit)
	job.Setenv("since", since)
	job.Setenv("before", before)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		return false
	}
	if err := job.Run(); err != nil {
		return false
	}
	if len(outs.Data) != len(expected) {
		return false
	}
	for i := 0; i < len(outs.Data); i++ {
		if outs.Data[i].Get("Id") != expected[i] {
			return false
		}
	}
	return true
}

// Regression test for being able to untag an image with an existing
// container
func TestDeleteTagWithExistingContainers(t *testing.T) {
	eng := NewTestEngine(t)
	defer nuke(mkDaemonFromEngine(eng, t))

	srv := mkServerFromEngine(eng, t)

	// Tag the image
	if err := eng.Job("tag", unitTestImageID, "utest", "tag1").Run(); err != nil {
		t.Fatal(err)
	}

	// Create a container from the image
	config, _, _, err := runconfig.Parse([]string{unitTestImageID, "echo test"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createNamedTestContainer(eng, config, t, "testingtags")
	if id == "" {
		t.Fatal("No id returned")
	}

	job := srv.Eng.Job("containers")
	job.SetenvBool("all", true)
	outs, err := job.Stdout.AddListTable()
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	if len(outs.Data) != 1 {
		t.Fatalf("Expected 1 container got %d", len(outs.Data))
	}

	// Try to remove the tag
	imgs := engine.NewTable("", 0)
	if err := srv.DeleteImage("utest:tag1", imgs, true, false, false); err != nil {
		t.Fatal(err)
	}

	if len(imgs.Data) != 1 {
		t.Fatalf("Should only have deleted one untag %d", len(imgs.Data))
	}

	if untag := imgs.Data[0].Get("Untagged"); untag != "utest:tag1" {
		t.Fatalf("Expected %s got %s", unitTestImageID, untag)
	}
}
