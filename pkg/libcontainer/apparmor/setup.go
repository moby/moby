package apparmor

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

const DefaultProfilePath = "/etc/apparmor.d/docker"
const DefaultProfile = `
# AppArmor profile from lxc for containers.
@{HOME}=@{HOMEDIRS}/*/ /root/
@{HOMEDIRS}=/home/
#@{HOMEDIRS}+=
@{multiarch}=*-linux-gnu*
@{PROC}=/proc/

profile docker-default flags=(attach_disconnected,mediate_deleted) {
  network,
  capability,
  file,
  umount,

  # ignore DENIED message on / remount
  deny mount options=(ro, remount) -> /,

  # allow tmpfs mounts everywhere
  mount fstype=tmpfs,

  # allow mqueue mounts everywhere
  mount fstype=mqueue,

  # allow fuse mounts everywhere
  mount fstype=fuse.*,

  # allow bind mount of /lib/init/fstab for lxcguest
  mount options=(rw, bind) /lib/init/fstab.lxc/ -> /lib/init/fstab/,

  # deny writes in /proc/sys/fs but allow binfmt_misc to be mounted
  mount fstype=binfmt_misc -> /proc/sys/fs/binfmt_misc/,
  deny @{PROC}/sys/fs/** wklx,

  # allow efivars to be mounted, writing to it will be blocked though
  mount fstype=efivarfs -> /sys/firmware/efi/efivars/,

  # block some other dangerous paths
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/sys/kernel/[^s][^h][^m]* wklx,
  deny @{PROC}/sys/kernel/*/** wklx,

  # deny writes in /sys except for /sys/fs/cgroup, also allow
  # fusectl, securityfs and debugfs to be mounted there (read-only)
  mount fstype=fusectl -> /sys/fs/fuse/connections/,
  mount fstype=securityfs -> /sys/kernel/security/,
  mount fstype=debugfs -> /sys/kernel/debug/,
  deny mount fstype=debugfs -> /var/lib/ureadahead/debugfs/,
  mount fstype=proc -> /proc/,
  mount fstype=sysfs -> /sys/,
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
  mount options=(move) /sys/fs/cgroup/cgmanager/ -> /sys/fs/cgroup/cgmanager.lower/,

  # the container may never be allowed to mount devpts.  If it does, it
  # will remount the host's devpts.  We could allow it to do it with
  # the newinstance option (but, right now, we don't).
  deny mount fstype=devpts,
}
`

func InstallDefaultProfile() error {
	if !IsEnabled() {
		return nil
	}

	// If the profile already exists, let it be.
	if _, err := os.Stat(DefaultProfilePath); err == nil {
		return nil
	}

	// Make sure /etc/apparmor.d exists
	if err := os.MkdirAll(path.Dir(DefaultProfilePath), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(DefaultProfilePath, []byte(DefaultProfile), 0644); err != nil {
		return err
	}

	output, err := exec.Command("/lib/init/apparmor-profile-load", "docker").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error loading docker profile: %s (%s)", err, output)
	}
	return nil
}
