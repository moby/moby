package zombie

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
)

func TestExecutor(t *testing.T) {
	// shell loop to test multiple step applications.
	testString := "for i in {1..5}; do echo $i; done"
	err := Executor(testString)
	if err != nil {
		t.Error(err)
	}

	// sleep test to test arbitrary waiting.
	testSleep := "sleep 1"
	err = Executor(testSleep)
	if err != nil {
		t.Error(err)
	}
}

// TestChildReaper tests `cat /dev/random` to demonstrate a long running command
// that will never exit. It creates the processs, then detaches the process as a
// child process. This demonstrates a process that creates a detached child,
// then calls `ChildReaper()` to clean it up.
func TestChildReaper(t *testing.T) {
	cmdToRun := "/bin/cat"
	args := []string{"/dev/random"}
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}

	fork, err := os.StartProcess(cmdToRun, args, procAttr)
	if err != nil {
		t.Error(err)
	}
	log.Println("started fork:", fork.Pid)

	err = fork.Release()
	if err != nil {
		t.Error(err)
	}
	log.Println("releasing fork:", fork.Pid)

	exit, err := ChildReaper()
	if err != nil {
		t.Error(err)
	}

	// 	// In the test results, you will see:
	// 	// started fork: processID
	// 	// releasing fork: -1
	// 	// [{0 0} {0 0} ... {processID, ExitStatus}
	// 	//log.Println(exit)
	for pid, status := range exit {
		if exit[pid].Status != 0 {
			t.Error(pid, "HAS BAD EXIT STATUS:", status)
		}
	}
	log.Println("all exits are good!")
}

func TestSignalhandler(t *testing.T) {
	testString := "sleep 1"
	MonitoredPids = make(map[int]string)

	signals := make(chan os.Signal, 1024)
	signal.Notify(signals, syscall.SIGCHLD, syscall.SIGTERM, syscall.SIGINT)

	waitGroup := sync.WaitGroup{}

	waitGroup.Add(1)
	go SignalHandler(signals, &waitGroup)

	for i := 0; i < 3; i++ {
		if err := Executor(testString); err != nil {
			t.Error(err)
		}
	}

	log.Println("sending SIGCHLD.")
	syscall.Kill(syscall.Getpid(), syscall.SIGCHLD)
	log.Println("processes have exited.")
}
