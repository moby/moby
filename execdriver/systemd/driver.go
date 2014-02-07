package systemd

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const DriverName = "systemd"

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		if err := setupHostname(args); err != nil {
			return err
		}

		/*		if err := setupNetworking(args); err != nil {
					return err
				}
		*/
		if err := setupCapabilities(args); err != nil {
			return err
		}

		if err := setupWorkingDirectory(args); err != nil {
			return err
		}

		if err := changeUser(args); err != nil {
			return err
		}

		path, err := exec.LookPath(args.Args[0])
		if err != nil {
			log.Printf("Unable to locate %v", args.Args[0])
			os.Exit(127)
		}
		if err := syscall.Exec(path, args.Args, os.Environ()); err != nil {
			return fmt.Errorf("dockerinit unable to execute %s - %s", path, err)
		}
		panic("Unreachable")
	})
}

type driver struct {
	root       string // root path for the driver to use
	sharedRoot bool
}

func NewDriver() (*driver, error) {
	return &driver{}, nil
}

func (d *driver) Run(c *execdriver.Command, startCallback execdriver.StartCallback) (int, error) {

	params := []string{
		"systemd-nspawn",
		"-M", c.ID,
		"-D", c.Rootfs,
	}

	params = append(params, c.InitPath)
	params = append(params, "-driver", DriverName)

	if c.Network != nil {
		params = append(params,
			"-g", c.Network.Gateway,
			"-i", fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen),
			"-mtu", strconv.Itoa(c.Network.Mtu),
		)
	}

	if c.User != "" {
		params = append(params, "-u", c.User)
	}

	if c.Privileged {
		params = append(params, "-privileged")
	}

	if c.WorkingDir != "" {
		params = append(params, "-w", c.WorkingDir)
	}

	params = append(params, "--", c.Entrypoint)
	params = append(params, c.Arguments...)

	var (
		name = params[0]
		arg  = params[1:]
	)
	aname, err := exec.LookPath(name)
	if err != nil {
		aname = name
	}
	c.Path = aname
	c.Args = append([]string{name}, arg...)

	if err := c.Start(); err != nil {
		return -1, err
	}

	var (
		waitErr  error
		waitLock = make(chan struct{})
	)
	go func() {
		if err := c.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok { // Do not propagate the error if it's simply a status code != 0
				waitErr = err
			}
		}
		close(waitLock)
	}()

	if startCallback != nil {
		startCallback(c)
	}

	<-waitLock

	return getExitCode(c), waitErr
}

/// Return the exit code of the process
// if the process has not exited -1 will be returned
func getExitCode(c *execdriver.Command) int {
	if c.ProcessState == nil {
		return -1
	}
	return c.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}

func (d *driver) Kill(c *execdriver.Command, sig int) error {
	return c.Process.Kill()
}

func (d *driver) Restore(c *execdriver.Command) error {
	panic("Restore Not Implemented")
}

func (d *driver) version() string {
	version := ""
	if output, err := exec.Command("systemd-nspawn", "--version").CombinedOutput(); err == nil {
		outputStr := string(output)
		if len(strings.SplitN(outputStr, " ", 2)) == 2 {
			version = strings.TrimSpace(strings.SplitN(outputStr, " ", 2)[1])
		}
	}
	return version
}

func (d *driver) getInfo(id string) ([]byte, error) {
	panic("get Info Not implemented")
}

type info struct {
	ID     string
	driver *driver
}

func (i *info) IsRunning() bool {
	fmt.Printf("Info Not implemented")
	return false
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		ID:     id,
		driver: d,
	}
}

func (d *driver) Name() string {
	version := d.version()
	return fmt.Sprintf("%s-%s", DriverName, version)
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	return nil, fmt.Errorf("GetPidsForContainer Not supported")
}
