package daemon

import (
	"fmt"
	"syscall"
)

func (daemon *Daemon) ImageMount(image, dstMountPath string) (err error) {
	img, err := daemon.repositories.LookupImage(image)
	if err != nil {
		return err
	}

	srcMountPath, err := daemon.driver.Get(img.ID, "")
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			daemon.driver.Put(img.ID)
		}
	}()

	// Bind mount image to user specified path.
	if err = syscall.Mount(srcMountPath, dstMountPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("Bind mount failed:%v", err)
	}

	// Remount read only.
	if err = syscall.Mount(srcMountPath, dstMountPath, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("Remount read only failed:%v", err)
	}
	return nil
}

func (daemon *Daemon) ImageUmount(image, dstMountPath string) error {
	img, err := daemon.repositories.LookupImage(image)
	if err != nil {
		return err
	}

	if err = syscall.Unmount(dstMountPath, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("Unmount failed:%v", err)
	}

	if err = daemon.driver.Put(img.ID); err != nil {
		return err
	}

	return nil
}
