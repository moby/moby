// +build windows

package client

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
)

// HotAddVhd hot-adds a VHD to a utility VM. This is used in the global one-utility-VM-
// service-VM per host scenario. In order to do a graphdriver `Diff`, we hot-add the
// sandbox to /mnt/<id> so that we can run `exportSandbox` inside the utility VM to
// get a tar-stream of the sandboxes contents back to the daemon.
func (config *Config) HotAddVhd(hostPath string, containerPath string) error {
	logrus.Debugf("opengcs: HotAddVhd: %s: %s", hostPath, containerPath)

	if config.Uvm == nil {
		return fmt.Errorf("cannot hot-add VHD as no utility VM is in configuration")
	}

	modification := &hcsshim.ResourceModificationRequestResponse{
		Resource: "MappedVirtualDisk",
		Data: hcsshim.MappedVirtualDisk{
			HostPath:          hostPath,
			ContainerPath:     containerPath,
			CreateInUtilityVM: true,
			//ReadOnly:          true,
		},
		Request: "Add",
	}
	logrus.Debugf("opengcs: HotAddVhd: %s to %s", hostPath, containerPath)
	if err := config.Uvm.Modify(modification); err != nil {
		return fmt.Errorf("opengcs: HotAddVhd: failed: %s", err)
	}
	logrus.Debugf("opengcs: HotAddVhd: %s added successfully", hostPath)
	return nil
}
