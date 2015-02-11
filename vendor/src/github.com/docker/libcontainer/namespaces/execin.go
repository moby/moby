// +build linux

package namespaces

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/mount"
	"github.com/docker/libcontainer/system"
	"github.com/docker/libcontainer/utils"
)

// ExecIn reexec's the initPath with the argv 0 rewrite to "nsenter" so that it is able to run the
// setns code in a single threaded environment joining the existing containers' namespaces.
func ExecIn(container *libcontainer.Config, state *libcontainer.State, userArgs []string, initPath, action string,
	stdin io.Reader, stdout, stderr io.Writer, console string, startCallback func(*exec.Cmd)) (int, error) {

	args := []string{fmt.Sprintf("nsenter-%s", action), "--nspid", strconv.Itoa(state.InitPid)}

	if console != "" {
		args = append(args, "--console", console)
	}

	cmd := &exec.Cmd{
		Path: initPath,
		Args: append(args, append([]string{"--"}, userArgs...)...),
	}

	if filepath.Base(initPath) == initPath {
		if lp, err := exec.LookPath(initPath); err == nil {
			cmd.Path = lp
		}
	}

	parent, child, err := newInitPipe()
	if err != nil {
		return -1, err
	}
	defer parent.Close()

	// Note: these are only used in non-tty mode
	// if there is a tty for the container it will be opened within the namespace and the
	// fds will be duped to stdin, stdiout, and stderr
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.ExtraFiles = []*os.File{child}

	if err := cmd.Start(); err != nil {
		child.Close()
		return -1, err
	}
	child.Close()

	terminate := func(terr error) (int, error) {
		// TODO: log the errors for kill and wait
		cmd.Process.Kill()
		cmd.Wait()
		return -1, terr
	}

	// Enter cgroups.
	if err := EnterCgroups(state, cmd.Process.Pid); err != nil {
		return terminate(err)
	}

	// finish cgroups' setup, unblock the child process.
	if _, err := parent.WriteString("1"); err != nil {
		return terminate(err)
	}

	if err := json.NewEncoder(parent).Encode(container); err != nil {
		return terminate(err)
	}

	if startCallback != nil {
		startCallback(cmd)
	}

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}
	return cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// Finalize expects that the setns calls have been setup and that is has joined an
// existing namespace
func FinalizeSetns(container *libcontainer.Config, args []string) error {
	// clear the current processes env and replace it with the environment defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}

	if err := setupRlimits(container); err != nil {
		return fmt.Errorf("setup rlimits %s", err)
	}

	if err := FinalizeNamespace(container); err != nil {
		return err
	}

	if err := apparmor.ApplyProfile(container.AppArmorProfile); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", container.AppArmorProfile, err)
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	if err := system.Execv(args[0], args[0:], os.Environ()); err != nil {
		return err
	}

	panic("unreachable")
}

// SetupContainer is run to setup mounts and networking related operations
// for a user namespace enabled process as a user namespace root doesn't
// have permissions to perform these operations.
// The setup process joins all the namespaces of user namespace enabled init
// except the user namespace, so it run as root in the root user namespace
// to perform these operations.
func SetupContainer(container *libcontainer.Config, dataPath, uncleanRootfs, consolePath string) error {
	rootfs, err := utils.ResolveRootfs(uncleanRootfs)
	if err != nil {
		return err
	}

	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}

	state, err := libcontainer.GetState(dataPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to read state: %s", err)
	}

	cloneFlags := GetNamespaceFlags(container.Namespaces)

	if (cloneFlags & syscall.CLONE_NEWNET) == 0 {
		if len(container.Networks) != 0 || len(container.Routes) != 0 {
			return fmt.Errorf("unable to apply network parameters without network namespace")
		}
	} else {
		if err := setupNetwork(container, &state.NetworkState); err != nil {
			return fmt.Errorf("setup networking %s", err)
		}
		if err := setupRoute(container); err != nil {
			return fmt.Errorf("setup route %s", err)
		}
	}

	label.Init()

	hostRootUid, err := GetHostRootUid(container)
	if err != nil {
		return fmt.Errorf("failed to get hostRootUid %s", err)
	}

	hostRootGid, err := GetHostRootGid(container)
	if err != nil {
		return fmt.Errorf("failed to get hostRootGid %s", err)
	}

	// InitializeMountNamespace() can be executed only for a new mount namespace
	if (cloneFlags & syscall.CLONE_NEWNS) == 0 {
		if container.MountConfig != nil {
			return fmt.Errorf("mount config is set without mount namespace")
		}
	} else if err := mount.InitializeMountNamespace(rootfs,
		consolePath,
		container.RestrictSys,
		hostRootUid,
		hostRootGid,
		(*mount.MountConfig)(container.MountConfig)); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}

	return nil
}

func EnterCgroups(state *libcontainer.State, pid int) error {
	return cgroups.EnterPid(state.CgroupPaths, pid)
}
