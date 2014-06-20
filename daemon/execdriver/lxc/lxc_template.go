package lxc

import (
	"strings"
	"text/template"

	"github.com/docker/libcontainer/label"
	"github.com/dotcloud/docker/daemon/execdriver"
)

const LxcTemplate = `
{{if .Network.Interface}}
# network configuration
lxc.network.type = veth
lxc.network.link = {{.Network.Interface.Bridge}}
lxc.network.name = eth0
lxc.network.mtu = {{.Network.Mtu}}
{{else if .Network.HostNetworking}}
lxc.network.type = none
{{else}}
# network is disabled (-n=false)
lxc.network.type = empty
lxc.network.flags = up
lxc.network.mtu = {{.Network.Mtu}}
{{end}}

# root filesystem
{{$ROOTFS := .Rootfs}}
lxc.rootfs = {{$ROOTFS}}

# use a dedicated pts for the container (and limit the number of pseudo terminal
# available)
lxc.pts = 1024

# disable the main console
lxc.console = none
{{if .ProcessLabel}}
lxc.se_context = {{ .ProcessLabel}}
{{end}}
{{$MOUNTLABEL := .MountLabel}}

# no controlling tty at all
lxc.tty = 1

{{if .Privileged}}
lxc.cgroup.devices.allow = a
{{else}}
# no implicit access to devices
lxc.cgroup.devices.deny = a
#Allow the devices passed to us in the AllowedDevices list.
{{range $allowedDevice := .AllowedDevices}}
lxc.cgroup.devices.allow = {{$allowedDevice.GetCgroupAllowString}}
{{end}}
{{end}}

# standard mount point
# Use mnt.putold as per https://bugs.launchpad.net/ubuntu/+source/lxc/+bug/986385
lxc.pivotdir = lxc_putold

# NOTICE: These mounts must be applied within the namespace

# WARNING: mounting procfs and/or sysfs read-write is a known attack vector.
# See e.g. http://blog.zx2c4.com/749 and http://bit.ly/T9CkqJ
# We mount them read-write here, but later, dockerinit will call the Restrict() function to remount them read-only.
# We cannot mount them directly read-only, because that would prevent loading AppArmor profiles.
lxc.mount.entry = proc {{escapeFstabSpaces $ROOTFS}}/proc proc nosuid,nodev,noexec 0 0
lxc.mount.entry = sysfs {{escapeFstabSpaces $ROOTFS}}/sys sysfs nosuid,nodev,noexec 0 0

{{if .Tty}}
lxc.mount.entry = {{.Console}} {{escapeFstabSpaces $ROOTFS}}/dev/console none bind,rw 0 0
{{end}}

lxc.mount.entry = devpts {{escapeFstabSpaces $ROOTFS}}/dev/pts devpts {{formatMountLabel "newinstance,ptmxmode=0666,nosuid,noexec" $MOUNTLABEL}} 0 0
lxc.mount.entry = shm {{escapeFstabSpaces $ROOTFS}}/dev/shm tmpfs {{formatMountLabel "size=65536k,nosuid,nodev,noexec" $MOUNTLABEL}} 0 0

{{range $value := .Mounts}}
{{if $value.Writable}}
lxc.mount.entry = {{$value.Source}} {{escapeFstabSpaces $ROOTFS}}/{{escapeFstabSpaces $value.Destination}} none bind,rw 0 0
{{else}}
lxc.mount.entry = {{$value.Source}} {{escapeFstabSpaces $ROOTFS}}/{{escapeFstabSpaces $value.Destination}} none bind,ro 0 0
{{end}}
{{end}}

{{if .Privileged}}
{{if .AppArmor}}
lxc.aa_profile = unconfined
{{else}}
# Let AppArmor normal confinement take place (i.e., not unconfined)
{{end}}
{{end}}

# limits
{{if .Resources}}
{{if .Resources.Memory}}
lxc.cgroup.memory.limit_in_bytes = {{.Resources.Memory}}
lxc.cgroup.memory.soft_limit_in_bytes = {{.Resources.Memory}}
{{with $memSwap := getMemorySwap .Resources}}
lxc.cgroup.memory.memsw.limit_in_bytes = {{$memSwap}}
{{end}}
{{end}}
{{if .Resources.CpuShares}}
lxc.cgroup.cpu.shares = {{.Resources.CpuShares}}
{{end}}
{{if .Resources.Cpuset}}
lxc.cgroup.cpuset.cpus = {{.Resources.Cpuset}}
{{end}}
{{end}}

{{if .Config.lxc}}
{{range $value := .Config.lxc}}
lxc.{{$value}}
{{end}}
{{end}}
`

var LxcTemplateCompiled *template.Template

// Escape spaces in strings according to the fstab documentation, which is the
// format for "lxc.mount.entry" lines in lxc.conf. See also "man 5 fstab".
func escapeFstabSpaces(field string) string {
	return strings.Replace(field, " ", "\\040", -1)
}

func getMemorySwap(v *execdriver.Resources) int64 {
	// By default, MemorySwap is set to twice the size of RAM.
	// If you want to omit MemorySwap, set it to `-1'.
	if v.MemorySwap < 0 {
		return 0
	}
	return v.Memory * 2
}

func getLabel(c map[string][]string, name string) string {
	label := c["label"]
	for _, l := range label {
		parts := strings.SplitN(l, "=", 2)
		if strings.TrimSpace(parts[0]) == name {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func init() {
	var err error
	funcMap := template.FuncMap{
		"getMemorySwap":     getMemorySwap,
		"escapeFstabSpaces": escapeFstabSpaces,
		"formatMountLabel":  label.FormatMountLabel,
	}
	LxcTemplateCompiled, err = template.New("lxc").Funcs(funcMap).Parse(LxcTemplate)
	if err != nil {
		panic(err)
	}
}
