// +build windows

package client

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

// HotAddVhd hot-adds a VHD to a utility VM. This is used in the global one-utility-VM-
// service-VM per host scenario. In order to do a graphdriver `Diff`, we hot-add the
// sandbox to /mnt/<id> so that we can run `exportSandbox` inside the utility VM to
// get a tar-stream of the sandboxes contents back to the daemon.
func (config *Config) HotAddVhd(hostPath string, containerPath string, readOnly bool, mount bool) error {
	logrus.Debugf("opengcs: HotAddVhd: %s: %s", hostPath, containerPath)

	if config.Uvm == nil {
		return fmt.Errorf("cannot hot-add VHD as no utility VM is in configuration")
	}

	defer config.DebugGCS()

	modification := &hcsshim.ResourceModificationRequestResponse{
		Resource: "MappedVirtualDisk",
		Data: hcsshim.MappedVirtualDisk{
			HostPath:          hostPath,
			ContainerPath:     containerPath,
			CreateInUtilityVM: true,
			ReadOnly:          readOnly,
			AttachOnly:        !mount,
		},
		Request: "Add",
	}

	if err := config.Uvm.Modify(modification); err != nil {
		return fmt.Errorf("failed to modify utility VM configuration for hot-add: %s", err)
	}
	logrus.Debugf("opengcs: HotAddVhd: %s added successfully", hostPath)
	return nil
}
