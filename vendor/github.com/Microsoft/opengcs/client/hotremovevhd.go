// +build windows

package client

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
)

// HotRemoveVhd hot-removes a VHD from a utility VM. This is used in the global one-utility-VM-
// service-VM per host scenario.
func (config *Config) HotRemoveVhd(hostPath string) error {
	logrus.Debugf("opengcs: HotRemoveVhd: %s", hostPath)

	if config.Uvm == nil {
		return fmt.Errorf("cannot hot-add VHD as no utility VM is in configuration")
	}

	modification := &hcsshim.ResourceModificationRequestResponse{
		Resource: "MappedVirtualDisk",
		Data: hcsshim.MappedVirtualDisk{
			HostPath:          hostPath,
			CreateInUtilityVM: true,
		},
		Request: "Remove",
	}
	if err := config.Uvm.Modify(modification); err != nil {
		return fmt.Errorf("failed modifying utility VM for hot-remove %s: %s", hostPath, err)
	}
	logrus.Debugf("opengcs: HotRemoveVhd: %s removed successfully", hostPath)
	return nil
}
