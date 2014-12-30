// +build !test_execdriver_lxc

package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRunCapDropInvalid(t *testing.T) {
	defer deleteAllContainers()
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-drop=CHPASS invalid")
}

func TestRunCapDropCannotMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=MKNOD", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=MKNOD cannot mknod")
}

func TestRunCapDropCannotMknodLowerCase(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=mknod", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=mknod cannot mknod lowercase")
}

func TestRunCapDropALLCannotMknod(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-drop=ALL", "busybox", "sh", "-c", "mknod /tmp/sda b 8 0 && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-drop=ALL cannot mknod")
}

func TestRunCapAddInvalid(t *testing.T) {
	defer deleteAllContainers()

	cmd := exec.Command(dockerBinary, "run", "--cap-add=CHPASS", "busybox", "ls")
	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	logDone("run - test --cap-add=CHPASS invalid")
}

func TestRunCapAddALLCanDownInterface(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "--cap-add=ALL", "busybox", "sh", "-c", "ip link set eth0 down && echo ok")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual != "ok" {
		t.Fatalf("expected output ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test --cap-add=ALL can set eth0 down")
}

func TestRunUnPrivilegedCannotMount(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "sh", "-c", "mount -t tmpfs none /tmp && echo ok")

	out, _, err := runCommandWithOutput(cmd)
	if err == nil {
		t.Fatal(err, out)
	}

	if actual := strings.Trim(out, "\r\n"); actual == "ok" {
		t.Fatalf("expected output not ok received %s", actual)
	}
	deleteAllContainers()

	logDone("run - test un-privileged cannot mount")
}

func TestRunSysNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/sys/kernel/profiling")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("sys should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - sys not writable in non privileged container")
}

func TestRunProcNotWritableInNonPrivilegedContainers(t *testing.T) {
	cmd := exec.Command(dockerBinary, "run", "busybox", "touch", "/proc/sysrq-trigger")
	if code, err := runCommand(cmd); err == nil || code == 0 {
		t.Fatal("proc should not be writable in a non privileged container")
	}

	deleteAllContainers()

	logDone("run - proc not writable in non privileged container")
}

func TestRunNonLocalMacAddress(t *testing.T) {
	defer deleteAllContainers()
	addr := "00:16:3E:08:00:50"

	cmd := exec.Command(dockerBinary, "run", "--mac-address", addr, "busybox", "ifconfig")
	if out, _, err := runCommandWithOutput(cmd); err != nil || !strings.Contains(out, addr) {
		t.Fatalf("Output should have contained %q: %s, %v", addr, out, err)
	}

	logDone("run - use non-local mac-address")
}
