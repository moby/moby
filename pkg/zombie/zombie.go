package zombie

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// MonitoredPids will keep track of all the processIDs that need to be killed.
var MonitoredPids map[int]string

// Executor will execute arbitrary commands.
func Executor(exe string) (err error) {

	// simple command start. it's a string, so it should execute simply.
	cmd := exec.Command("sh", "-c", exe)
	if err != nil {
		log.Println(err)
		return err
	}

	err = cmd.Start()
	if err != nil {
		log.Println(err)
	}

	// and now we wait.
	err = cmd.Wait()

	//if there is one.
	return err
}

// SignalHandler will wait for a keyboard or system signal before trying to kill
//
func SignalHandler(signal chan os.Signal, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	for sig := range signal {
		switch sig {
		case syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL:
			// exits with no error code.
			return

		case syscall.SIGCHLD:
			//this is the call that will exit the children processes.

			//information on a child process.
			var (
				status syscall.WaitStatus
				usage  syscall.Rusage
			)

			// this calls the child process of the processID via a WNOHANG
			// but does nothing until we determine the state and act accordingly
			processID, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &usage)
			if err != nil {
				log.Println(err)
			}

			if _, ok := MonitoredPids[processID]; ok {
				// assuming the process is ok, delete the processID from the
				// monitored processes.
				delete(MonitoredPids, processID)

				// loop through the monitoredPids to make sure all the processes
				// are dead and return.
				if len(MonitoredPids) == 0 {
					return
				}

			}
		}
	}
}

// Exit provices the arrayed structure for the childReaper exit.
type Exit struct {
	Process int
	Status  int
}

// ChildReaper is designed to be called at the end execution, when all children
// processes need to be cleaned. This prevents zombie processes from being
// left behind when a container exits.
func ChildReaper() (exits []Exit, err error) {
	//start the loop to reap any abandonded children.
	for {
		var (
			status syscall.WaitStatus
			usage  syscall.Rusage
		)

		// make the `wait4` call and check for errors. if the error equals
		// ECHILD (no child process), return empty exit status.
		processID, err := syscall.Wait4(-1, &status, syscall.WNOHANG, &usage)
		if err != nil {
			if err == syscall.ECHILD {
				return exits, nil
			}
		}

		// shows which processes have exited and with what status.
		exits = append(exits, Exit{
			processID, status.ExitStatus(),
		})
	}
}
