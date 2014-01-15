package lxc

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"github.com/dotcloud/docker/utils"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const DriverName = "lxc"

func init() {
	execdriver.RegisterDockerInitFct(DriverName, func(args *execdriver.DockerInitArgs) error {
		if err := setupHostname(args); err != nil {
			return err
		}

		if err := setupNetworking(args); err != nil {
			return err
		}

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
			panic(err)
		}
		panic("Unreachable")
	})
}

type driver struct {
	root       string // root path for the driver to use
	apparmor   bool
	sharedRoot bool
}

func NewDriver(root string, apparmor bool) (*driver, error) {
	// setup unconfined symlink
	if err := linkLxcStart(root); err != nil {
		return nil, err
	}
	return &driver{
		apparmor:   apparmor,
		root:       root,
		sharedRoot: rootIsShared(),
	}, nil
}

func (d *driver) Name() string {
	return DriverName
}

func (d *driver) Run(c *execdriver.Process, startCallback execdriver.StartCallback) (int, error) {
	params := []string{
		"lxc-start",
		"-n", c.ID,
		"-f", c.ConfigPath,
		"--",
		c.InitPath,
		"-driver",
		d.Name(),
	}

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
		if d.apparmor {
			params[0] = path.Join(d.root, "lxc-start-unconfined")

		}
		params = append(params, "-privileged")
	}

	if c.WorkingDir != "" {
		params = append(params, "-w", c.WorkingDir)
	}

	if d.sharedRoot {
		// lxc-start really needs / to be non-shared, or all kinds of stuff break
		// when lxc-start unmount things and those unmounts propagate to the main
		// mount namespace.
		// What we really want is to clone into a new namespace and then
		// mount / MS_REC|MS_SLAVE, but since we can't really clone or fork
		// without exec in go we have to do this horrible shell hack...
		shellString :=
			"mount --make-rslave /; exec " +
				utils.ShellQuoteArguments(params)

		params = []string{
			"unshare", "-m", "--", "/bin/sh", "-c", shellString,
		}
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
			waitErr = err
		}
		close(waitLock)
	}()

	// Poll lxc for RUNNING status
	if err := d.waitForStart(c, waitLock); err != nil {
		return -1, err
	}

	if startCallback != nil {
		startCallback(c)
	}

	<-waitLock

	return c.GetExitCode(), waitErr
}

func (d *driver) Kill(c *execdriver.Process, sig int) error {
	return d.kill(c, sig)
}

func (d *driver) Wait(id string) error {
	for {
		output, err := exec.Command("lxc-info", "-n", id).CombinedOutput()
		if err != nil {
			return err
		}
		if !strings.Contains(string(output), "RUNNING") {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (d *driver) Version() string {
	version := ""
	if output, err := exec.Command("lxc-version").CombinedOutput(); err == nil {
		outputStr := string(output)
		if len(strings.SplitN(outputStr, ":", 2)) == 2 {
			version = strings.TrimSpace(strings.SplitN(outputStr, ":", 2)[1])
		}
	}
	return version
}

func (d *driver) kill(c *execdriver.Process, sig int) error {
	output, err := exec.Command("lxc-kill", "-n", c.ID, strconv.Itoa(sig)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Err: %s Output: %s", err, output)
	}
	return nil
}

func (d *driver) waitForStart(c *execdriver.Process, waitLock chan struct{}) error {
	var (
		err    error
		output []byte
	)
	// We wait for the container to be fully running.
	// Timeout after 5 seconds. In case of broken pipe, just retry.
	// Note: The container can run and finish correctly before
	// the end of this loop
	for now := time.Now(); time.Since(now) < 5*time.Second; {
		select {
		case <-waitLock:
			// If the process dies while waiting for it, just return
			return nil
			if c.ProcessState != nil && c.ProcessState.Exited() {
				return nil
			}
		default:
		}

		output, err = d.getInfo(c.ID)
		if err != nil {
			output, err = d.getInfo(c.ID)
			if err != nil {
				return err
			}
		}
		if strings.Contains(string(output), "RUNNING") {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return execdriver.ErrNotRunning
}

func (d *driver) getInfo(id string) ([]byte, error) {
	return exec.Command("lxc-info", "-s", "-n", id).CombinedOutput()
}

type info struct {
	ID     string
	driver *driver
}

func (i *info) IsRunning() bool {
	var running bool

	output, err := i.driver.getInfo(i.ID)
	if err != nil {
		panic(err)
	}
	if strings.Contains(string(output), "RUNNING") {
		running = true
	}
	return running
}

func (d *driver) Info(id string) execdriver.Info {

	return &info{
		ID:     id,
		driver: d,
	}
}

func linkLxcStart(root string) error {
	sourcePath, err := exec.LookPath("lxc-start")
	if err != nil {
		return err
	}
	targetPath := path.Join(root, "lxc-start-unconfined")

	if _, err := os.Lstat(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		if err := os.Remove(targetPath); err != nil {
			return err
		}
	}
	return os.Symlink(sourcePath, targetPath)
}

// TODO: This can be moved to the mountinfo reader in the mount pkg
func rootIsShared() bool {
	if data, err := ioutil.ReadFile("/proc/self/mountinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			cols := strings.Split(line, " ")
			if len(cols) >= 6 && cols[4] == "/" {
				return strings.HasPrefix(cols[6], "shared")
			}
		}
	}

	// No idea, probably safe to assume so
	return true
}
