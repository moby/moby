package lmctfy

import (
	"fmt"
	"github.com/dotcloud/docker/runtime/execdriver"
	"path"
	"os/exec"
	"strings"
)

func buildUserCmd(c *execdriver.Command) []string {
	cmd := []string{c.Entrypoint}
	for _, arg := range c.Arguments {
		cmd = append(cmd, arg)
	}
	return cmd
}

func addFlag(args *[]string, flagName, value string) {
	*args = append(*args, flagName, value)
}

func buildDockerInitCmd(c *execdriver.Command) []string {
	args := []string{"./" + path.Base(c.InitPath)}
	if c.User != "" {
		addFlag(&args, "-u", c.User)
	}
	if c.WorkingDir != "" {
		addFlag(&args, "-w", c.WorkingDir)
	}
	if c.Privileged {
		addFlag(&args, "-privileged", "")
	}
	addFlag(&args, "-driver", DriverName)
	if c.ConfigPath != "" {
		addFlag(&args, "-root", c.ConfigPath)
	}
	args = append(args, buildUserCmd(c)...)
	return args
}

func getCustomInit(c *execdriver.Command) string {
	customInit := "init: { init_argv: '"
	customInit += strings.Join(buildDockerInitCmd(c), "' init_argv: '")
	customInit += "' "
	if c.Console != "" {
		runSpec := fmt.Sprintf("run_spec: { console: { slave_pty: '%s' } }", path.Base(c.Console))
		customInit += runSpec
	}
	return customInit + "}"
}

func getNetworkConfig(c *execdriver.Command) string {
	if c.Network == nil || c.Network.Interface == nil {
		return ""
	}
	vethId := c.ID
	if len(vethId) > 10 {
		vethId = vethId[0:10]
	}
	vethPair := fmt.Sprintf("veth_pair: { outside: 'veth%s' inside: 'eth0' }", vethId)
	bridge := fmt.Sprintf("bridge: {name: 'docker0'}")
	virtualIp := fmt.Sprintf("virtual_ip: { ip: '%s' netmask: '%d' gateway: '%s' mtu:%d }",
		c.Network.Interface.IPAddress, c.Network.Interface.IPPrefixLen, c.Network.Interface.Gateway, c.Network.Mtu)

	return fmt.Sprintf("network: { connection: {%s %s} %s}", vethPair, bridge, virtualIp)
}

func getVirtualHostSpec(c *execdriver.Command) string {
	return fmt.Sprintf("virtual_host: { %s %s }", getCustomInit(c), getNetworkConfig(c))
}

func getLmctfyFilesystemConfig(c *execdriver.Command) string {
	var output []string
	for _, mount := range(c.Mounts) {
		mountEntry := fmt.Sprintf("source: '%s' target: '%s' ", mount.Source, mount.Destination)
		if mount.Private {
			mountEntry += "private: true "
		}
		if !mount.Writable {
			mountEntry += "read_only: true "
		}
		output = append(output, mountEntry)
	}
	if c.Console != "" {
		mountEntry := fmt.Sprintf("source: '%s' target: '%s' private: true", c.Console, "dev/console")
		output = append(output, mountEntry)
	}
	rootfsPath := fmt.Sprintf("rootfs: '%s'", c.Rootfs) 
	return fmt.Sprintf("filesystem: { %s mounts: { mount: { %s } } }", rootfsPath, strings.Join(output, " } mount: { "))
}

func getContainerConfig(c *execdriver.Command) string {
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
		output = append(output, fmt.Sprintf("cpu: {limit: %d}", c.Resources.CpuShares/CpuSharesPerCpu))
	}
	return fmt.Sprintf("%s %s", getLmctfyFilesystemConfig(c), strings.Join(output, " "))
}

func getLmctfyConfig(c *execdriver.Command) string {
	return fmt.Sprintf("%s %s", getContainerConfig(c), getVirtualHostSpec(c))
}

func getCreaperCmd(c *execdriver.Command) []string {	
	cmd := []string{
		CreaperBinary,
	}
	cmd = append(cmd, c.ID)
	return append(cmd, getLmctfyConfig(c))
}

func setupExecCmd(c *execdriver.Command, pipes *execdriver.Pipes) error {
	term, err := setupTerminal(c, pipes)
	if err != nil {
		return err
	}
	c.Terminal = term
	cmd := getCreaperCmd(c)
	c.Path, err = exec.LookPath(cmd[0])
	if err != nil {
		return err
	}
	c.Args = append([]string{cmd[0]}, cmd[1:]...)
	c.Cmd.Env = c.Env
	if err := term.Attach(&c.Cmd); err != nil {
		return err
	}
	return nil
}
