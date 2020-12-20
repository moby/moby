package vfs // import "github.com/docker/docker/daemon/graphdriver/vfs"

import (
	"github.com/docker/docker/quota"
	"github.com/sirupsen/logrus"
)

//nolint:structcheck
type driverQuota struct {
	quotaCtl *quota.Control
	quotaOpt quota.Quota
}

func setupDriverQuota(driver *Driver) {
	if quotaCtl, err := quota.NewControl(driver.home); err == nil {
		driver.quotaCtl = quotaCtl
	} else if err != quota.ErrQuotaNotSupported {
		logrus.Warnf("Unable to setup quota: %v\n", err)
	}
}

func (d *Driver) setQuotaOpt(size uint64) error {
	d.quotaOpt.Size = size
	return nil
}

func (d *Driver) getQuotaOpt() uint64 {
	return d.quotaOpt.Size
}

func (d *Driver) setupQuota(dir string, size uint64) error {
	return d.quotaCtl.SetQuota(dir, quota.Quota{Size: size})
}

func (d *Driver) quotaSupported() bool {
	return d.quotaCtl != nil
}
