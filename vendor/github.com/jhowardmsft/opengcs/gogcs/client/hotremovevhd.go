// +build windows

package client

// TODO @jhowardmsft - This will move to Microsoft/opengcs soon

import (
	"fmt"

	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
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
		return fmt.Errorf("opengcs: HotRemoveVhd: %s failed: %s", hostPath, err)
	}
	logrus.Debugf("opengcs: HotRemoveVhd: %s removed successfully", hostPath)
	return nil
}
