package lmctfy

import (
	"fmt"
	"github.com/dotcloud/docker/runtime/execdriver"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	DriverName    = "lmctfy"
	LmctfyBinary  = "lmctfy"
	CreaperBinary = "lmctfy-creaper"
)

type driver struct {
}

func init() {
	// This method gets invoked from docker init.
	execdriver.RegisterInitFunc(DriverName, func(args *execdriver.InitArgs) error {
		log := log.New(os.Stderr, "", log.Lshortfile)
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
		if err := syscall.Exec(path, args.Args, os.Environ()); err != nil {
			errorMsg := fmt.Errorf("dockerinit unable to execute %s - %s", path, err)
			log.Println(errorMsg)
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

func addFlag(args *[]string, flagName, value string) {
	*args = append(*args, flagName, value)
}

func buildDockerInitCmd(c *execdriver.Command, pipes *execdriver.Pipes) []string {
	args := []string{"./" + path.Base(c.InitPath)}
	if c.User != "" {
		addFlag(&args, "-u", c.User)
	}
	if c.Network != nil && c.Network.Interface != nil {
		addFlag(&args, "-mtu", strconv.Itoa(c.Network.Mtu))
		addFlag(&args, "-g", c.Network.Interface.Gateway)
		addFlag(&args, "-i", fmt.Sprintf("%s/%d", c.Network.Interface.IPAddress, c.Network.Interface.IPPrefixLen))
	}
	if c.WorkingDir != "" {
		addFlag(&args, "-w", c.WorkingDir)
	}
	if c.Privileged {
		addFlag(&args, "-privileged", "")
	}
	addFlag(&args, "-driver", DriverName)

	if c.Console != "" {
		addFlag(&args, "-console", c.Console)
	}
	if c.ConfigPath != "" {
		addFlag(&args, "-root", c.ConfigPath)
	}
	args = append(args, c.Entrypoint+" "+strings.Join(c.Arguments, " "))
	return args
}

func buildLmctfyConfig(c *execdriver.Command) string {
	var output []string
	if c.Resources == nil {
		return ""
	}
	// Use proto buf here
	var memoryArgs []string
	if c.Resources.Memory > 0 {
		memoryArgs = append(memoryArgs, fmt.Sprintf("limit: %d", c.Resources.Memory))
	}
	if c.Resources.MemorySwap > 0 {
		memoryArgs = append(memoryArgs, fmt.Sprintf("swap_limit: %d", c.Resources.MemorySwap))
	}
	if len(memoryArgs) > 0 {
		output = append(output, fmt.Sprintf("memory: { %s }", strings.Join(memoryArgs, " ")))
	}
	if c.Resources.CpuShares > 0 {
		output = append(output, fmt.Sprintf("cpu: {limit: %d}", c.Resources.CpuShares))
	}
	return strings.Join(output, " ")
}

func buildMountCommand(source, destination string, rw, private bool) []string {
	cmd := []string{fmt.Sprintf("mount --rbind %s %s &&", source, destination)}
	if !rw {
		cmd = append(cmd, fmt.Sprintf("mount --rbind %s %s -o remount,ro &&", destination, destination))
	}
	var mountType string
	if private {
		mountType = "--make-rprivate"
	} else {
		mountType = "--make-rslave"
	}
	cmd = append(cmd, fmt.Sprintf("mount %s %s &&", mountType, destination))
	return cmd
}

func buildMountWrapper(c *execdriver.Command) []string {
	cmd := []string{}
	for _, mount := range c.Mounts {
		absDestinationPath := filepath.Join(c.Rootfs, mount.Destination)
		cmd = append(cmd, buildMountCommand(mount.Source, absDestinationPath, mount.Writable, mount.Private)...)
	}
	cmd = append(cmd, fmt.Sprintf("mount --move $(cat /proc/mounts | grep shm | awk '{print $2}') %s &&", filepath.Join(c.Rootfs, "dev/shm")))
	cmd = append(cmd, fmt.Sprintf("mount --move /sys %s &&", filepath.Join(c.Rootfs, "sys")))
	cmd = append(cmd, fmt.Sprintf("mount --move /proc %s &&", filepath.Join(c.Rootfs, "proc")))
	return cmd
}

func buildPivotRootWrapper(c *execdriver.Command, origCmd []string) []string {
	cmd := []string{}
	cmd = append(cmd, buildMountWrapper(c)...)
	cmd = append(cmd, fmt.Sprintf("cd %s &&", c.Rootfs))
	cmd = append(cmd, "mkdir old-root &&")
	cmd = append(cmd, "pivot_root . old-root &&")
	cmd = append(cmd, "cd / &&")
	cmd = append(cmd, "umount -l old-root &&")
	cmd = append(cmd, "exec")
	cmd = append(cmd, "chroot . "+strings.Join(origCmd, " "))
	return cmd
}

func buildNetworkConfig(veth_id string, network *execdriver.Network) string {
	if network == nil || network.Interface == nil {
		return ""
	}
	veth_pair_str := fmt.Sprintf("veth_pair: { outside: \"veth%s\" inside: \"eth0\" }", veth_id)
	virtual_ip_str := fmt.Sprintf("virtual_ip: { ip: \"%s\" netmask: \"%d\" gateway: \"%s\" mtu:%d }",
		network.Interface.IPAddress, network.Interface.IPPrefixLen, network.Interface.Gateway, network.Mtu)
	return fmt.Sprintf("network: { %s %s }", veth_pair_str, virtual_ip_str)
}

func buildCreaperCmd(c *execdriver.Command, pipes *execdriver.Pipes) []string {
	dockerCmd := buildDockerInitCmd(c, pipes)
	realCmd := buildPivotRootWrapper(c, dockerCmd)
	veth_id := c.ID
	if len(veth_id) > 10 {
		veth_id = veth_id[0:10]
	}
	lmctfy_command := fmt.Sprintf("%s virtual_host: { %s }", buildLmctfyConfig(c), buildNetworkConfig(veth_id, c.Network))
	cmd := []string{
		CreaperBinary,
	}
	if c.Network != nil && c.Network.Interface != nil && c.Network.Interface.Bridge != "" {
		cmd = append(cmd, fmt.Sprintf("--networkSetup=brctl addif docker0 veth%s;\n /sbin/ifconfig veth%s up", veth_id, veth_id))
	}
	cmd = append(cmd, c.ID)
	cmd = append(cmd, lmctfy_command)
	cmd = append(cmd, strings.Join(realCmd, " "))
	return cmd
}

func setupExecCmd(c *execdriver.Command, pipes *execdriver.Pipes) error {
	var err error
	cmd := buildCreaperCmd(c, pipes)
	c.Path, err = exec.LookPath(cmd[0])
	if err != nil {
		return err
	}
	c.Args = append([]string{cmd[0]}, cmd[1:]...)
	return nil
}

func (d *driver) Run(c *execdriver.Command, pipes *execdriver.Pipes, startCallback execdriver.StartCallback) (int, error) {
	var err error
	if err = execdriver.SetTerminal(c, pipes); err != nil {
		return -1, err
	}
	message := make(chan error)
	if err = setupExecCmd(c, pipes); err != nil {
		return -1, err
	}
	go func() {
		if err := c.Run(); err != nil {
			message <- fmt.Errorf("container creation failed. error: %s\ncommand:%s", err, c.Args)
		} else {
			message <- nil
		}
	}()

	if startCallback != nil {
		startCallback(c)
	}

	err = <-message
	return 0, err
}

func removeContainer(id string) error {
	if output, err := exec.Command(LmctfyBinary, "destroy", id).CombinedOutput(); err != nil {
		return fmt.Errorf("Err: lmctfy create failed with: %s and output: %s", err, output)
	}
	return nil
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
