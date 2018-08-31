package wclayer

import (
	"fmt"

	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/sirupsen/logrus"
)

// GrantVmAccess adds access to a file for a given VM
func GrantVmAccess(vmid string, filepath string) error {
	title := fmt.Sprintf("hcsshim::GrantVmAccess id:%s path:%s ", vmid, filepath)
	logrus.Debugf(title)

	err := grantVmAccess(vmid, filepath)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s", filepath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title + " - succeeded")
	return nil
}
