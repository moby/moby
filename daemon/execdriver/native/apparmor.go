// +build linux

package native

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/opencontainers/runc/libcontainer/apparmor"
)

const (
	apparmorProfilePath = "/etc/apparmor.d/docker"
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

  network,
  capability,
  file,
  umount,

  deny @{PROC}/{*,**^[0-9*],sys/kernel/shm*} wkx,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/kcore rwklx,

  deny mount,

  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
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
	if tunablesExists() {
		data.Imports = append(data.Imports, "#include <tunables/global>")
	} else {
		data.Imports = append(data.Imports, "@{PROC}=/proc/")
	}
	if abstractionsExists() {
		data.InnerImports = append(data.InnerImports, "#include <abstractions/base>")
	}
	if err := compiled.Execute(out, data); err != nil {
		return err
	}
	return nil
}

// check if the tunables/global exist
func tunablesExists() bool {
	_, err := os.Stat("/etc/apparmor.d/tunables/global")
	return err == nil
}

// check if abstractions/base exist
func abstractionsExists() bool {
	_, err := os.Stat("/etc/apparmor.d/abstractions/base")
	return err == nil
}

func installAppArmorProfile() error {
	if !apparmor.IsEnabled() {
		return nil
	}

	// Make sure /etc/apparmor.d exists
	if err := os.MkdirAll(path.Dir(apparmorProfilePath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(apparmorProfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := generateProfile(f); err != nil {
		f.Close()
		return err
	}
	f.Close()

	cmd := exec.Command("/sbin/apparmor_parser", "-r", "-W", "docker")
	// to use the parser directly we have to make sure we are in the correct
	// dir with the profile
	cmd.Dir = "/etc/apparmor.d"

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error loading docker apparmor profile: %s (%s)", err, output)
	}
	return nil
}

func hasAppArmorProfileLoaded(profile string) error {
	file, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return err
	}
	r := bufio.NewReader(file)
	for {
		p, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.HasPrefix(p, profile+" ") {
			return nil
		}
	}
}
