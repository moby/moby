package apparmor

import (
	"io"
	"os"
	"text/template"
)

type data struct {
	Name         string
	Imports      []string
	InnerImports []string
}

const baseTemplate = `
{{range $value := .Imports}}
{{$value}}
{{end}}

profile {{.Name}} flags=(attach_disconnected,mediate_deleted) {
{{range $value := .InnerImports}}
  {{$value}}
{{end}}

  # Globally allows everything to run under this profile. This is fine-tuned
  # later
  # in this profile and can be narrowed depending on the container's use.
  file,
  capability,
  network,

  # the container may never be allowed to mount devpts.  If it does, it
  # will remount the host's devpts.  We could allow it to do it with
  # the newinstance option (but, right now, we don't).
  deny mount fstype=devpts,

  umount,

  # This also needs additional rules to reach outside of the container via
  # DBus, so just let all of DBus within the container.
  dbus,

  # Allow us to ptrace ourselves
  ptrace peer=@{profile_name},

  # ignore DENIED message on / remount
  deny mount options=(ro, remount) -> /,
  deny mount options=(ro, remount, silent) -> /,

  # allow tmpfs mounts everywhere
  mount fstype=tmpfs,

  # allow hugetlbfs mounts everywhere
  mount fstype=hugetlbfs,

  # allow mqueue mounts everywhere
  mount fstype=mqueue,

  # allow fuse mounts everywhere
  mount fstype=fuse.*,

  # deny writes in /proc/sys/fs but allow binfmt_misc to be mounted
  mount fstype=binfmt_misc -> /proc/sys/fs/binfmt_misc/,
  deny @{PROC}/sys/fs/** wklx,

  # allow efivars to be mounted, writing to it will be blocked though
  mount fstype=efivarfs -> /sys/firmware/efi/efivars/,

  # block some other dangerous paths
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,

  # deny writes in /sys except for /sys/fs/cgroup, also allow
  # fusectl, securityfs and debugfs to be mounted there (read-only)
  mount fstype=fusectl -> /sys/fs/fuse/connections/,
  mount fstype=securityfs -> /sys/kernel/security/,
  mount fstype=debugfs -> /sys/kernel/debug/,
  deny mount fstype=debugfs -> /var/lib/ureadahead/debugfs/,
  mount fstype=proc -> /proc/,
  mount fstype=sysfs -> /sys/,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
  mount options=(move) /sys/fs/cgroup/cgmanager/ -> /sys/fs/cgroup/cgmanager.lower/,

  mount options=(rw nosuid nodev noexec remount) -> /sys/,
  mount options=(rw remount) -> /sys/kernel/security/,
  mount options=(rw remount) -> /sys/fs/pstore/,
  mount options=(ro remount) -> /sys/fs/pstore/,

  deny /proc/sys/[^kn]*{,/**} wklx,
  deny /proc/sys/k[^e]*{,/**} wklx,
  deny /proc/sys/ke[^r]*{,/**} wklx,
  deny /proc/sys/ker[^n]*{,/**} wklx,
  deny /proc/sys/kern[^e]*{,/**} wklx,
  deny /proc/sys/kerne[^l]*{,/**} wklx,
  deny /proc/sys/kernel/[^smhd]*{,/**} wklx,
  deny /proc/sys/kernel/d[^o]*{,/**} wklx,
  deny /proc/sys/kernel/do[^m]*{,/**} wklx,
  deny /proc/sys/kernel/dom[^a]*{,/**} wklx,
  deny /proc/sys/kernel/doma[^i]*{,/**} wklx,
  deny /proc/sys/kernel/domai[^n]*{,/**} wklx,
  deny /proc/sys/kernel/domain[^n]*{,/**} wklx,
  deny /proc/sys/kernel/domainn[^a]*{,/**} wklx,
  deny /proc/sys/kernel/domainna[^m]*{,/**} wklx,
  deny /proc/sys/kernel/domainnam[^e]*{,/**} wklx,
  deny /proc/sys/kernel/domainname?*{,/**} wklx,
  deny /proc/sys/kernel/h[^o]*{,/**} wklx,
  deny /proc/sys/kernel/ho[^s]*{,/**} wklx,
  deny /proc/sys/kernel/hos[^t]*{,/**} wklx,
  deny /proc/sys/kernel/host[^n]*{,/**} wklx,
  deny /proc/sys/kernel/hostn[^a]*{,/**} wklx,
  deny /proc/sys/kernel/hostna[^m]*{,/**} wklx,
  deny /proc/sys/kernel/hostnam[^e]*{,/**} wklx,
  deny /proc/sys/kernel/hostname?*{,/**} wklx,
  deny /proc/sys/kernel/m[^s]*{,/**} wklx,
  deny /proc/sys/kernel/ms[^g]*{,/**} wklx,
  deny /proc/sys/kernel/msg*/** wklx,
  deny /proc/sys/kernel/s[^he]*{,/**} wklx,
  deny /proc/sys/kernel/se[^m]*{,/**} wklx,
  deny /proc/sys/kernel/sem*/** wklx,
  deny /proc/sys/kernel/sh[^m]*{,/**} wklx,
  deny /proc/sys/kernel/shm*/** wklx,
  deny /proc/sys/kernel?*{,/**} wklx,
  deny /proc/sys/n[^e]*{,/**} wklx,
  deny /proc/sys/ne[^t]*{,/**} wklx,
  deny /proc/sys/net?*{,/**} wklx,
  deny /sys/[^fdc]*{,/**} wklx,
  deny /sys/c[^l]*{,/**} wklx,
  deny /sys/cl[^a]*{,/**} wklx,
  deny /sys/cla[^s]*{,/**} wklx,
  deny /sys/clas[^s]*{,/**} wklx,
  deny /sys/class/[^n]*{,/**} wklx,
  deny /sys/class/n[^e]*{,/**} wklx,
  deny /sys/class/ne[^t]*{,/**} wklx,
  deny /sys/class/net?*{,/**} wklx,
  deny /sys/class?*{,/**} wklx,
  deny /sys/d[^e]*{,/**} wklx,
  deny /sys/de[^v]*{,/**} wklx,
  deny /sys/dev[^i]*{,/**} wklx,
  deny /sys/devi[^c]*{,/**} wklx,
  deny /sys/devic[^e]*{,/**} wklx,
  deny /sys/device[^s]*{,/**} wklx,
  deny /sys/devices/[^v]*{,/**} wklx,
  deny /sys/devices/v[^i]*{,/**} wklx,
  deny /sys/devices/vi[^r]*{,/**} wklx,
  deny /sys/devices/vir[^t]*{,/**} wklx,
  deny /sys/devices/virt[^u]*{,/**} wklx,
  deny /sys/devices/virtu[^a]*{,/**} wklx,
  deny /sys/devices/virtua[^l]*{,/**} wklx,
  deny /sys/devices/virtual/[^n]*{,/**} wklx,
  deny /sys/devices/virtual/n[^e]*{,/**} wklx,
  deny /sys/devices/virtual/ne[^t]*{,/**} wklx,
  deny /sys/devices/virtual/net?*{,/**} wklx,
  deny /sys/devices/virtual?*{,/**} wklx,
  deny /sys/devices?*{,/**} wklx,
  deny /sys/f[^s]*{,/**} wklx,
  deny /sys/fs/[^c]*{,/**} wklx,
  deny /sys/fs/c[^g]*{,/**} wklx,
  deny /sys/fs/cg[^r]*{,/**} wklx,
  deny /sys/fs/cgr[^o]*{,/**} wklx,
  deny /sys/fs/cgro[^u]*{,/**} wklx,
  deny /sys/fs/cgrou[^p]*{,/**} wklx,
  deny /sys/fs/cgroup?*{,/**} wklx,
  deny /sys/fs?*{,/**} wklx,
}
`

func generateProfile(out io.Writer) error {
	compiled, err := template.New("apparmor_profile").Parse(baseTemplate)
	if err != nil {
		return err
	}
	data := &data{
		Name: "docker-default",
	}
	if tuntablesExists() {
		data.Imports = append(data.Imports, "#include <tunables/global>")
	} else {
		data.Imports = append(data.Imports, "@{PROC}=/proc/")
	}
	if abstrctionsEsists() {
		data.InnerImports = append(data.InnerImports, "#include <abstractions/base>")
	}
	if err := compiled.Execute(out, data); err != nil {
		return err
	}
	return nil
}

// check if the tunables/global exist
func tuntablesExists() bool {
	_, err := os.Stat("/etc/apparmor.d/tunables/global")
	return err == nil
}

// check if abstractions/base exist
func abstrctionsEsists() bool {
	_, err := os.Stat("/etc/apparmor.d/abstractions/base")
	return err == nil
}
