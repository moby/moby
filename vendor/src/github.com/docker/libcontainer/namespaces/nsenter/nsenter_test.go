package nsenter

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
)

func TestNsenterAlivePid(t *testing.T) {
	args := []string{"nsenter-exec", "--nspid", fmt.Sprintf("%d", os.Getpid())}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe %v", err)
	}

	cmd := &exec.Cmd{
		Path:       os.Args[0],
		Args:       args,
		ExtraFiles: []*os.File{r},
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("nsenter failed to start %v", err)
	}
	r.Close()

	// unblock the child process
	if _, err := w.WriteString("1"); err != nil {
		t.Fatalf("parent failed to write synchronization data %v", err)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("nsenter exits with a non-zero exit status")
	}
}

func TestNsenterInvalidPid(t *testing.T) {
	args := []string{"nsenter-exec", "--nspid", "-1"}

	cmd := &exec.Cmd{
		Path: os.Args[0],
		Args: args,
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("nsenter exits with a zero exit status")
	}
}

func TestNsenterDeadPid(t *testing.T) {

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGCHLD)
	dead_cmd := exec.Command("true")
	if err := dead_cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer dead_cmd.Wait()
	<-c // dead_cmd is zombie

	args := []string{"nsenter-exec", "--nspid", fmt.Sprintf("%d", dead_cmd.Process.Pid)}

	cmd := &exec.Cmd{
		Path: os.Args[0],
		Args: args,
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("nsenter exits with a zero exit status")
	}
}

func init() {
	if strings.HasPrefix(os.Args[0], "nsenter-") {
		os.Exit(0)
	}
	return
}
