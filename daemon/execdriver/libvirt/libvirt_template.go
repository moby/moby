// +build linux
// +build !dockerinit

package libvirt

import (
	"github.com/docker/docker/daemon/execdriver"
	"github.com/docker/docker/utils"
	"gopkg.in/alexzorin/libvirt-go.v2"
	"html/template"
)

const LibvirtLxcTemplate = `
<domain type='lxc'>
  <name>{{TruncateID .ID}}</name>
{{with .Resources.Memory}}
  <memory unit='b'>{{.}}</memory>
{{else}}
  <memory>500000</memory>
{{end}}
  <os>
    <type>exe</type>
    <init>{{.ProcessConfig.Entrypoint}}</init>
{{range .ProcessConfig.Arguments}}
    <initarg>{{.}}</initarg>
{{end}}
  </os>
  <vcpu>1</vcpu>
{{with .Resources.CpuShares}}
  <cputune>
    <shares>{{.}}</shares>
  </cputune>
{{end}}
{{if .Resources.Memory}}
  <memtune>
    <hard_limit unit='bytes'>{{.Resources.Memory}}</hard_limit>
    <soft_limit unit='bytes'>{{.Resources.Memory}}</soft_limit>
    <swap_hard_limit unit='bytes'>{{.Resources.MemorySwap}}</swap_hard_limit>
  </memtune>
{{end}}
  <features>
{{if isPrivNetwork .Network}}
    <privnet/>
{{end}}
  </features>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <clock offset='utc'/>
  <devices>
    <filesystem type='mount'>
      <source dir='{{.Rootfs}}'/>
      <target dir='/'/>
    </filesystem>
{{range $value := .Mounts}}
    <filesystem type='mount'>
      <source dir='{{$value.Source}}'/>
      <target dir='{{$value.Destination}}'/>
{{if not $value.Writable}}
      <readonly/>
{{end}}
    </filesystem>
{{end}}
{{if .Network.Interface}}
    <interface type='bridge'>
      <source bridge='{{.Network.Interface.Bridge}}'/>
    </interface>
{{end}}
    <console type='pty'/>
  </devices>
{{if not .ProcessConfig.Privileged}}
{{if .AppArmor}}
  <seclabel type='dynamic' model='apparmor'/>
{{else if .ProcessLabel}}
  <seclabel type='static' model='selinux'>
    <label>{{.ProcessLabel}}</label>
  </seclabel>
{{end}}
{{end}}
</domain>

`

var LibvirtLxcTemplateCompiled *template.Template

func getMemory(v *execdriver.Resources) uint64 {
	if v != nil && v.Memory > 0 {
		return uint64(v.Memory / 1024)
	} else {
		return libvirt.VIR_DOMAIN_MEMORY_PARAM_UNLIMITED
	}
}

func isPrivNetwork(n *execdriver.Network) bool {
	return !n.HostNetworking && (n.Interface == nil)
}

func getTemplate() (*template.Template, error) {
	funcMap := template.FuncMap{
		"getMemory":     getMemory,
		"isPrivNetwork": isPrivNetwork,
		"TruncateID":    utils.TruncateID,
	}
	return template.New("libvirt-lxc").Funcs(funcMap).Parse(LibvirtLxcTemplate)
}
