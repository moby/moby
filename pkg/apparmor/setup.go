package apparmor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

const (
	DefaultProfilePath = "/etc/apparmor.d/docker"
)

const DefaultProfile = `
#include <tunables/global>
profile docker-default flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>
  network,
  capability,
  file,
  umount,

  mount fstype=tmpfs,
  mount fstype=mqueue,
  mount fstype=fuse.*,
  mount fstype=binfmt_misc -> /proc/sys/fs/binfmt_misc/,
  mount fstype=efivarfs -> /sys/firmware/efi/efivars/,
  mount fstype=fusectl -> /sys/fs/fuse/connections/,
  mount fstype=securityfs -> /sys/kernel/security/,
  mount fstype=debugfs -> /sys/kernel/debug/,
  mount fstype=proc -> /proc/,
  mount fstype=sysfs -> /sys/,

  deny @{PROC}/sys/fs/** wklx,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/sys/kernel/[^s][^h][^m]* wklx,
  deny @{PROC}/sys/kernel/*/** wklx,

  deny mount options=(ro, remount) -> /,
  deny mount fstype=debugfs -> /var/lib/ureadahead/debugfs/,
  deny mount fstype=devpts,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
}
`

func InstallDefaultProfile(backupPath string) error {
	if !IsEnabled() {
		return nil
	}

	// If the profile already exists, check if we already have a backup
	// if not, do the backup and override it. (docker 0.10 upgrade changed the apparmor profile)
	// see gh#5049, apparmor blocks signals in ubuntu 14.04
	if _, err := os.Stat(DefaultProfilePath); err == nil {
		if _, err := os.Stat(backupPath); err == nil {
			// If both the profile and the backup are present, do nothing
			return nil
		}
		// Make sure the directory exists
		if err := os.MkdirAll(path.Dir(backupPath), 0755); err != nil {
			return err
		}

		// Create the backup file
		f, err := os.Create(backupPath)
		if err != nil {
			return err
		}
		defer f.Close()

		src, err := os.Open(DefaultProfilePath)
		if err != nil {
			return err
		}
		defer src.Close()

		if _, err := io.Copy(f, src); err != nil {
			return err
		}
	}

	// Make sure /etc/apparmor.d exists
	if err := os.MkdirAll(path.Dir(DefaultProfilePath), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(DefaultProfilePath, []byte(DefaultProfile), 0644); err != nil {
		return err
	}

	output, err := exec.Command("/sbin/apparmor_parser", "-r", "-W", "docker").CombinedOutput()
	if err != nil && !os.IsNotExist(err) {
		if e, ok := err.(*exec.Error); ok {
			// keeping with the current profile load code, if the parser does not exist then
			// just return
			if e.Err == exec.ErrNotFound || os.IsNotExist(e.Err) {
				return nil
			}
		}
		return fmt.Errorf("Error loading docker profile: %s (%s)", err, output)
	}
	return nil
}
