// +build linux

package vfs

import "github.com/docker/docker/daemon/graphdriver/quota"

type driverQuota struct {
	quotaCtl *quota.Control
}

func setupDriverQuota(driver *Driver) error {
	if quotaCtl, err := quota.NewControl(driver.home); err == nil {
		driver.quotaCtl = quotaCtl
	} else if err != quota.ErrQuotaNotSupported {
		return err
	}

	return nil
}

func (d *Driver) setupQuota(dir string, size uint64) error {
	return d.quotaCtl.SetQuota(dir, quota.Quota{Size: size})
}

func (d *Driver) quotaSupported() bool {
	return d.quotaCtl != nil
}
