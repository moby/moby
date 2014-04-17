package lxc

import (
	"github.com/dotcloud/docker/daemon/execdriver"
	"github.com/dotcloud/docker/pkg/label"
	"strings"
	"text/template"
)

const LxcTemplate = `
{{if .Network.Interface}}
# network configuration
lxc.network.type = veth
lxc.network.link = {{.Network.Interface.Bridge}}
lxc.network.name = eth0
{{else}}
# network is disabled (-n=false)
lxc.network.type = empty
lxc.network.flags = up
{{end}}
lxc.network.mtu = {{.Network.Mtu}}

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

# but allow mknod for any device
lxc.cgroup.devices.allow = c *:* m
lxc.cgroup.devices.allow = b *:* m

# /dev/null and zero
lxc.cgroup.devices.allow = c 1:3 rwm
lxc.cgroup.devices.allow = c 1:5 rwm

# consoles
lxc.cgroup.devices.allow = c 5:1 rwm
lxc.cgroup.devices.allow = c 5:0 rwm
lxc.cgroup.devices.allow = c 4:0 rwm
lxc.cgroup.devices.allow = c 4:1 rwm

# /dev/urandom,/dev/random
lxc.cgroup.devices.allow = c 1:9 rwm
lxc.cgroup.devices.allow = c 1:8 rwm

# /dev/pts/ - pts namespaces are "coming soon"
lxc.cgroup.devices.allow = c 136:* rwm
lxc.cgroup.devices.allow = c 5:2 rwm

# tuntap
lxc.cgroup.devices.allow = c 10:200 rwm

# fuse
#lxc.cgroup.devices.allow = c 10:229 rwm

# rtc
#lxc.cgroup.devices.allow = c 254:0 rwm
{{end}}

# standard mount point
# Use mnt.putold as per https://bugs.launchpad.net/ubuntu/+source/lxc/+bug/986385
lxc.pivotdir = lxc_putold

# NOTICE: These mounts must be applied within the namespace

#  WARNING: procfs is a known attack vector and should probably be disabled
#           if your userspace allows it. eg. see http://blog.zx2c4.com/749
lxc.mount.entry = proc {{escapeFstabSpaces $ROOTFS}}/proc proc nosuid,nodev,noexec 0 0

# WARNING: sysfs is a known attack vector and should probably be disabled
# if your userspace allows it. eg. see http://bit.ly/T9CkqJ
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
#lxc.aa_profile = unconfined
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
