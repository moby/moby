package lmctfy

import (
	"fmt"
	"github.com/dotcloud/docker/runtime/execdriver"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const (
	DriverName      = "lmctfy"
	LmctfyBinary    = "lmctfy"
	CreaperBinary   = "lmctfy-creaper"
	CpuSharesPerCpu = 1024
)

type driver struct {
}

func init() {
	// This method gets invoked from docker init.
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		log := log.New(os.Stderr, "", log.Lshortfile)
		if err := setupEnv(args); err != nil {
			log.Println(err)
			return err
		}
		if err := setupHostname(args); err != nil {
			log.Println(err)
			return err
		}

		if err := setupCapabilities(args); err != nil {
			log.Println(err)
			return err
		}

		if err := setupWorkingDirectory(args); err != nil {
			log.Println(err)
			return err
		}

		if err := changeUser(args); err != nil {
			log.Println(err)
			return err
		}

		if len(args.Args) == 0 {
			log.Printf("Input Args missing. Error!")
			os.Exit(127)
		}
		path, err := exec.LookPath(args.Args[0])
		if err != nil {
			log.Printf("Unable to locate %v", args.Args[0])
			os.Exit(127)
		}
		if err := syscall.Exec(path, args.Args, args.Env); err != nil {
			errorMsg := fmt.Errorf("dockerinit unable to execute %s - %s", path, err)
			return errorMsg
		}
		panic("Unreachable")
	})
}

func NewDriver() (*driver, error) {
	if output, err := exec.Command(LmctfyBinary, "init", "").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Err: lmctfy init failed with: %s and output: %s", err, output)
	}
	return &driver{}, nil
}

func (d *driver) Name() string {
	return DriverName
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	var err error
	if err = performFsSetup(c); err != nil {
		return -1, err
	}
	if err = setupExecCmd(c, pipes); err != nil {
		return -1, err
	}
	if err = c.Start(); err != nil {
		return -1, err
	}
	if startCallback != nil {
		startCallback(c)
	}
	if err = c.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	status := c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	return status, err
}

func removeContainer(id string) error {
	if output, err := exec.Command(LmctfyBinary, "destroy", "-f", id).CombinedOutput(); err != nil {
		return fmt.Errorf("Err: lmctfy create failed with: %s and output: %s", err, output)
	}
	return nil
}

// Return the exit code of the process
// if the process has not exited -1 will be returned
func getExitCode(c *execdriver.Command) int {
	if c.ProcessState == nil {
		return -1
	}
	return c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}

func (d *driver) Kill(c *execdriver.Command, sig int) error {
	pids, err := JustGetPidsForContainer(c.ID)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		if err := syscall.Kill(pid, syscall.Signal(sig)); err != nil {
			return err
		}
	}
	return nil
}

func (d *driver) Terminate(c *execdriver.Command) error {
	return d.Kill(c, 9)
}

type info struct {
	id string
}

func (i *info) IsRunning() bool {
	if pids, err := JustGetPidsForContainer(i.id); err != nil {
		return false
	} else {
		return len(pids) > 0
	}
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{id}
}

func JustGetPidsForContainer(id string) ([]int, error) {
	pids := []int{}
	output, err := exec.Command(LmctfyBinary, "-v", "list", "tids", id).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Err: lmctfy list pids failed with: %s and output: %s", err, output)
	}
	tid_strings := strings.Split(string(output), "\n")
	for _, tid_string := range tid_strings[0 : len(tid_strings)-1] {
		tid, err := strconv.ParseInt(tid_string, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Err %s: Couldn't parse a pid: %s for a container %s. Whole output: %s", err, tid_string, id, output)
		}
		pids = append(pids, int(tid))
	}
	return pids, nil
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	return JustGetPidsForContainer(id)
}
