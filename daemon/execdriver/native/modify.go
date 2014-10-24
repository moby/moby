// +build linux

package native

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/reexec"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	"github.com/docker/libcontainer/devices"
	"github.com/docker/libcontainer/namespaces"
)

const modifyMknodCommandName = "nsenter-modify-mknod"
const modifyUnlinkCommandName = "nsenter-modify-unlink"

func init() {
	reexec.Register(modifyMknodCommandName, nsenterModifyMknod)
	reexec.Register(modifyUnlinkCommandName, nsenterModifyUnlink)
}

func nsenterModifyMknod() {
	runtime.LockOSThread()

	args := findUserArgs()
	path := args[0]
	mode, _ := strconv.ParseUint(args[1], 10, 32)
	dev, _ := strconv.Atoi(args[2])
	syscall.Umask(0)

	// I'm not checking for an error here because it's not being reported
	// to the main process
	syscall.Mknod(path, uint32(mode), dev)
}

func nsenterModifyUnlink() {
	runtime.LockOSThread()

	args := findUserArgs()
	path := args[0]
	syscall.Unlink(path)
}

func (d *driver) ModifyDeviceAdd(c *execdriver.Command, device *devices.Device) error {

	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}

	// Update allowed devices in cgroups
	active.container.Cgroups.AllowedDevices = append(active.container.Cgroups.AllowedDevices, device)

	// Apply changes to live container.
	if systemd.UseSystemd() {
		if err := systemd.ApplyDevices(active.container.Cgroups, c.ProcessConfig.Process.Pid); err != nil {
			return err
		}
	} else {
		if err := fs.ApplyDevices(active.container.Cgroups, c.ProcessConfig.Process.Pid); err != nil {
			return err
		}
	}

	deviceNumber := devices.Mkdev(device.MajorNumber, device.MinorNumber)
	args := []string{device.Path, strconv.FormatUint(uint64(device.FileMode), 10), strconv.Itoa(deviceNumber)}

	state, err := libcontainer.GetState(filepath.Join(d.root, c.ID))
	if err != nil {
		return fmt.Errorf("State unavailable for container with ID %s. The container may have been cleaned up already. Error: %s", c.ID, err)
	}

	_, err = namespaces.ExecIn(active.container, state, args, os.Args[0], "modify-mknod", nil, nil, nil, "", nil)
	if err != nil {
		return fmt.Errorf("Unable to execute docker binary for container with ID %s. Error: %s", c.ID, err)
	}
	return nil
}

func (d *driver) ModifyDeviceRemove(c *execdriver.Command, device *devices.Device) error {
	active := d.activeContainers[c.ID]
	if active == nil {
		return fmt.Errorf("active container for %s does not exist", c.ID)
	}

	// Update allowed devices in cgroups
	active.container.Cgroups.AllowedDevices = append(active.container.Cgroups.AllowedDevices, device)
	new_devices := []*devices.Device{}
	for i := 0; i < len(active.container.Cgroups.AllowedDevices); i++ {
		if active.container.Cgroups.AllowedDevices[i].Path != device.Path {
			new_devices = append(new_devices, active.container.Cgroups.AllowedDevices[i])
		}
	}
	active.container.Cgroups.AllowedDevices = new_devices

	// Apply changes to live container.
	if systemd.UseSystemd() {
		if err := systemd.ApplyDevices(active.container.Cgroups, c.ProcessConfig.Process.Pid); err != nil {
			return err
		}
	} else {
		if err := fs.ApplyDevices(active.container.Cgroups, c.ProcessConfig.Process.Pid); err != nil {
			return err
		}
	}

	args := []string{device.Path}

	state, err := libcontainer.GetState(filepath.Join(d.root, c.ID))
	if err != nil {
		return fmt.Errorf("State unavailable for container with ID %s. The container may have been cleaned up already. Error: %s", c.ID, err)
	}

	_, err = namespaces.ExecIn(active.container, state, args, os.Args[0], "modify-unlink", nil, nil, nil, "", nil)
	if err != nil {
		return fmt.Errorf("Unable to execute docker binary for container with ID %s. Error: %s", c.ID, err)
	}
	return nil
}
