package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRemoveContainerWithRemovedVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "losemyvolumes", "-v", "/tmp/testing:/test", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove("/tmp/testing"); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "losemyvolumes")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - removed volume")
}

func TestRemoveContainerWithVolume(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--name", "foo", "-v", "/srv", "busybox", "true")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	cmd = exec.Command(dockerBinary, "rm", "-v", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - volume")
}

func TestRemoveRunningContainer(t *testing.T) {
	createRunningContainer(t, "foo")

	// Test cannot remove running container
	cmd := exec.Command(dockerBinary, "rm", "foo")
	if _, err := runCommand(cmd); err == nil {
		t.Fatalf("Expected error, can't rm a running container")
	}

	deleteAllContainers()

	logDone("rm - running container")
}

func TestForceRemoveRunningContainer(t *testing.T) {
	createRunningContainer(t, "foo")

	// Stop then remove with -s
	cmd := exec.Command(dockerBinary, "rm", "-f", "foo")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}

	deleteAllContainers()

	logDone("rm - running container with --force=true")
}

func TestContainerOrphaning(t *testing.T) {
	dockerfile1 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]`
	img := "test-container-orphaning"
	dockerfile2 := `FROM busybox:latest
	ENTRYPOINT ["/bin/true"]
	MAINTAINER Integration Tests`

	// build first dockerfile
	img1, err := buildImage(img, dockerfile1, true)
	if err != nil {
		t.Fatalf("Could not build image %s: %v", img, err)
	}
	// run container on first image
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", img)); err != nil {
		t.Fatalf("Could not run image %s: %v: %s", img, err, out)
	}
	// rebuild dockerfile with a small addition at the end
	if _, err := buildImage(img, dockerfile2, true); err != nil {
		t.Fatalf("Could not rebuild image %s: %v", img, err)
	}
	// try to remove the image, should error out.
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rmi", img)); err == nil {
		t.Fatalf("Expected to error out removing the image, but succeeded: %s", out)
	}
	// check if we deleted the first image
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "images", "-q", "--no-trunc"))
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !strings.Contains(out, img1) {
		t.Fatalf("Orphaned container (could not find '%s' in docker images): %s", img1, out)
	}

	deleteAllContainers()

	logDone("rm - container orphaning")
}

func TestDeleteTagWithExistingContainers(t *testing.T) {
	container := "test-delete-tag"
	newtag := "busybox:newtag"
	bb := "busybox:latest"
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "tag", bb, newtag)); err != nil {
		t.Fatalf("Could not tag busybox: %v: %s", err, out)
	}
	if out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "run", "--name", container, bb, "/bin/true")); err != nil {
		t.Fatalf("Could not run busybox: %v: %s", err, out)
	}
	out, _, err := runCommandWithOutput(exec.Command(dockerBinary, "rmi", newtag))
	if err != nil {
		t.Fatalf("Could not remove tag %s: %v: %s", newtag, err, out)
	}
	if d := strings.Count(out, "Untagged: "); d != 1 {
		t.Fatalf("Expected 1 untagged entry got %d: %q", d, out)
	}

	deleteAllContainers()

	logDone("rm - delete tag with existing containers")

}

func createRunningContainer(t *testing.T, name string) {
	cmd := exec.Command(dockerBinary, "run", "-dt", "--name", name, "busybox", "top")
	if _, err := runCommand(cmd); err != nil {
		t.Fatal(err)
	}
}
