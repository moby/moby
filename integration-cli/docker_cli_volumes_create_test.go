package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVolumeCreate(t *testing.T) {
	defer deleteAllVolumes()
	cmd := exec.Command(dockerBinary, "volume", "create", "--name", "dark_helmet")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}
	cmd = exec.Command(dockerBinary, "volume", "inspect", "--format", "{{ .Path }}", "dark_helmet")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}
	path := strings.TrimSpace(out)
	if os.Stat(path); err != nil && os.IsNotExist(err) {
		t.Fatalf("expected %s to exist", path)
	}

	logDone("volumes create - volume created")
}

func TestVolumeCreateBindMount(t *testing.T) {
	defer deleteAllVolumes()
	tmpDir, err := ioutil.TempDir(os.TempDir(), "volumescreate-testbindmount")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	cmd := exec.Command(dockerBinary, "volume", "create", "--name", "princess_vespa", "--path", tmpDir)
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}
	cmd = exec.Command(dockerBinary, "volume", "inspect", "--format", "{{ .IsBindMount }}", "princess_vespa")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "true") {
		t.Fatal("Exepected IsBindMount to be true, got: %s", out)
	}

	logDone("volumes create - bind mount")
}

func TestVolumeCreateMode(t *testing.T) {
	defer deleteAllVolumes()
	// mode with normal volume
	cmd := exec.Command(dockerBinary, "volume", "create", "--name", "dot_matrix", "--mode", "ro")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}
	cmd = exec.Command(dockerBinary, "volume", "inspect", "--format", "{{ .Writable }}", "dot_matrix")
	if out, _, err := runCommandWithOutput(cmd); err != nil || !strings.Contains(out, "false") {
		t.Fatal(err, "Failed to set mode:", out)
	}

	// mode with bind-mount
	tmpDir, err := ioutil.TempDir(os.TempDir(), "volumescreate-modetest2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	cmd = exec.Command(dockerBinary, "volume", "create", "--name", "king_roland", "--path", tmpDir, "--mode", "ro")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}
	cmd = exec.Command(dockerBinary, "volume", "inspect", "--format", "{{ .Writable }}:{{ .IsBindMount }}", "king_roland")
	if out, _, err := runCommandWithOutput(cmd); err != nil || !strings.Contains(out, "false:true") {
		t.Fatal(err, "Failed to set mode:", out)
	}

	logDone("volumes create - mode is set")
}

func TestVolumeCreateUniquePaths(t *testing.T) {
	defer deleteAllVolumes()
	tmpDir, err := ioutil.TempDir(os.TempDir(), "volumescreate-onevolume")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dockerBinary, "volume", "create", "--path", tmpDir)
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	cmd = exec.Command(dockerBinary, "volume", "create", "--path", tmpDir)
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "Volume exists") {
		t.Fatalf("Expected creating 2nd volume with same path to fail\n%q", out)
	}

	logDone("volumes create - paths are unique")
}

func TestVolumeCreateUniqueNames(t *testing.T) {
	defer deleteAllVolumes()

	cmd := exec.Command(dockerBinary, "volume", "create", "--name", "lone_starr")
	if out, _, err := runCommandWithOutput(cmd); err != nil {
		t.Fatal(err, out)
	}

	cmd = exec.Command(dockerBinary, "volume", "create", "--name", "lone_starr")
	if out, _, err := runCommandWithOutput(cmd); err == nil || !strings.Contains(out, "Volume exists") {
		t.Fatalf("Expected creating 2nd volume with same name to fail\n%q", out)
	}

	logDone("volumes create - names are unique")
}
