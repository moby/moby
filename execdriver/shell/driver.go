package shell

import (
	"fmt"
	"github.com/dotcloud/docker/execdriver"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const DriverName = "shell"

const (
	CommandInit  = "init"
	CommandStart = "start"
)

type driver struct {
	root    string
	options string
	args    []string
	pids    map[string]int
	sync.RWMutex
}

type info struct {
	id     string
	driver *driver
}

func (d *driver) do_wait(cmd *exec.Cmd, exit_chan chan int) {
	for {
		// NOTE: The Wait() function will also take
		// care of closing stdin, stdout & stderr pipes
		// once the process exits.
		err := cmd.Wait()

		if err != nil {
			_, ok := err.(*exec.ExitError)
			if ok {
				// Wut? It seems we can't actually get the
				// exit code when we use the exec interface.
				// Woah, that sucks. For the moment, we'll
				// use 1 as a stand-in for all error codes.
				exit_chan <- 1
				break
			} else {
				// A different problem?
				// Maybe I/O related? We need to make sure
				// the process is cleaned up, so wait until
				// we catch the exit properly.
				continue
			}
		} else {
			// Success.
			exit_chan <- 0
			break
		}
	}
}

func (d *driver) do_exec(
	command string,
	args []string,
	workDir string,
	env []string,
	stdin io.ReadCloser,
	stdout io.Writer,
	stderr io.Writer) (int, chan int, error) {

	new_args := d.args[:]
	new_args = append(new_args, command)
	new_args = append(new_args, args...)

	cmd := exec.Command(new_args[0], new_args[1:]...)
	cmd.Dir = workDir
	cmd.Env = env

	var err error
	var cmd_stdin io.WriteCloser
	var cmd_stdout io.ReadCloser
	var cmd_stderr io.ReadCloser

	if stdin != nil {
		cmd_stdin, err = cmd.StdinPipe()
		if err != nil {
			return -1, nil, err
		}
		go io.Copy(cmd_stdin, stdin)
	}

	if stdout != nil {
		cmd_stdout, err = cmd.StdoutPipe()
		if err != nil {
			if cmd_stdin != nil {
				cmd_stdin.Close()
			}
			return -1, nil, err
		}
		go io.Copy(stdout, cmd_stdout)
	}

	if stderr != nil {
		cmd_stderr, err = cmd.StderrPipe()
		if err != nil {
			if cmd_stdin != nil {
				cmd_stdin.Close()
			}
			if cmd_stdout != nil {
				cmd_stdout.Close()
			}
			return -1, nil, err
		}
	}

	err = cmd.Start()
	if err != nil {
		if cmd_stdin != nil {
			cmd_stdin.Close()
		}
		if cmd_stdout != nil {
			cmd_stdout.Close()
		}
		if cmd_stderr != nil {
			cmd_stderr.Close()
		}
		return -1, nil, err
	}

	// Make sure we don't leave zombies.
	exit_chan := make(chan int, 1)
	go d.do_wait(cmd, exit_chan)

	// Done.
	return cmd.Process.Pid, exit_chan, err
}

func init() {
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		// This function is only called when we are run from dockerinit,
		// during which we are potentially already inside a container.
		// We simply execute the function as given.
		path, err := exec.LookPath(args.Args[0])
		if err != nil {
			return err
		}

		// Move into our working directory.
		if args.WorkDir != "" {
			err = os.Chdir(args.WorkDir)
			if err != nil {
				return err
			}
		}

		// Set our user.
		if args.User != "" {
			user, err := user.Lookup(args.User)
			if err != nil {
				return err
			}
			userid, err := strconv.ParseInt(user.Uid, 0, 64)
			if err != nil {
				return err
			}
			err = syscall.Setuid(int(userid))
			if err != nil {
				return err
			}
		}

		// Execute the new process.
		return syscall.Exec(path, args.Args, args.Env)
	})
}

func NewDriver(root, options string) (*driver, error) {
	args := strings.Split(options, " ")
	if len(args) == 0 {
		return nil, fmt.Errorf("no arguments provided?")
	}

	// Our driver.
	d := &driver{
		root:    root,
		options: options,
		args:    args,
		pids:    make(map[string]int),
	}

	// Run our external init.
	_, exit_chan, err := d.do_exec(CommandInit, []string{}, root, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	// Wait for init to finish.
	exit_code := <-exit_chan
	if exit_code != 0 {
		return nil, fmt.Errorf("shell process returned '%d' for init", exit_code)
	}

	return d, nil
}

func (d *driver) Name() string {
	return DriverName
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {

	// Allocate our terminal.
	err := execdriver.SetTerminal(c, pipes)
	if err != nil {
		return -1, err
	}

	// Export our configuration.
	extra_env := make([]string, 0, 0)
	extra_env = append(extra_env, fmt.Sprintf("DOCKER_ID=%s", c.ID))
	extra_env = append(extra_env, fmt.Sprintf("DOCKER_ROOTFS=%s", c.Rootfs))
	extra_env = append(extra_env, c.Config...)

	args := make([]string, 0, 0)
	args = append(args, c.InitPath)
	args = append(args, "-driver", DriverName)
	args = append(args, "-options", d.options)

	if c.Network != nil {
		extra_env = append(extra_env, fmt.Sprintf("DOCKER_BRIDGE=%s", c.Network.Bridge))
		extra_env = append(extra_env, fmt.Sprintf("DOCKER_MTU=%d", c.Network.Mtu))
		args = append(args, "-g", c.Network.Gateway)
		args = append(args, "-i", fmt.Sprintf("%s/%d", c.Network.IPAddress, c.Network.IPPrefixLen))
		args = append(args, "-mtu", strconv.Itoa(c.Network.Mtu))
	}
	if c.User != "" {
		args = append(args, "-u", c.User)
	}
	if c.WorkingDir != "" {
		args = append(args, "-w", c.WorkingDir)
	}

	args = append(args, "--")
	args = append(args, c.Entrypoint)
	args = append(args, c.Arguments...)

	// Start our process.
	pid, exit_chan, err := d.do_exec(
		CommandStart,
		args,
		d.root,
		extra_env,
		pipes.Stdin,
		pipes.Stdout,
		pipes.Stderr)
	if err != nil {
		return pid, err
	}

	// Remember that this is running.
	d.RWMutex.Lock()
	d.pids[c.ID] = pid
	d.RWMutex.Unlock()

	// Save our pid, and notify of startup.
	c.ContainerPid = pid
	if startCallback != nil {
		startCallback(c)
	}

	// Wait for exit.
	exit_code := <-exit_chan

	// We're not running.
	d.RWMutex.Lock()
	delete(d.pids, c.ID)
	d.RWMutex.Unlock()

	return exit_code, err
}

func (d *driver) Kill(c *execdriver.Command, sig int) error {
	return syscall.Kill(c.ContainerPid, syscall.Signal(sig))
}

func (d *driver) Info(id string) execdriver.Info {
	return &info{
		id:     id,
		driver: d,
	}
}

func (d *driver) GetPidsForContainer(id string) ([]int, error) {
	d.RWMutex.RLock()
	defer d.RWMutex.RUnlock()
	pids := make([]int, 0, 0)
	for _, pid := range d.pids {
		pids = append(pids, pid)
	}
	return pids, nil
}

func (i *info) IsRunning() bool {
	i.driver.RWMutex.RLock()
	defer i.driver.RWMutex.RUnlock()
	_, ok := i.driver.pids[i.id]
	return ok
}
