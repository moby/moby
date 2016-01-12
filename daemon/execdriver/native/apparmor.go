// +build linux

package native

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/docker/docker/pkg/aaparser"
	"github.com/opencontainers/runc/libcontainer/apparmor"
)

const (
	apparmorProfilePath = "/etc/apparmor.d/docker"
)

type data struct {
	Name         string
	ExecPath     string
	Imports      []string
	InnerImports []string
	MajorVersion int
	MinorVersion int
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

  deny @{PROC}/* w,   # deny write for all files directly in /proc (not in a subdir)
  # deny write to files not in /proc/<number>/** or /proc/sys/**
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9]*}/** w,
  deny @{PROC}/sys/[^k]** w,  # deny /proc/sys except /proc/sys/k* (effectively /proc/sys/kernel)
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,  # deny everything except shm* in /proc/sys/kernel/
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

{{if ge .MajorVersion 2}}{{if ge .MinorVersion 8}}
  # suppress ptrace denials when using 'docker ps' or using 'ps' inside a container
  ptrace (trace,read) peer=docker-default,
{{end}}{{end}}
{{if ge .MajorVersion 2}}{{if ge .MinorVersion 9}}
  # docker daemon confinement requires explict allow rule for signal
  signal (receive) set=(kill,term) peer={{.ExecPath}},
{{end}}{{end}}
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
	data.MajorVersion, data.MinorVersion, err = aaparser.GetVersion()
	if err != nil {
		return err
	}
	data.ExecPath, err = exec.LookPath("docker")
	if err != nil {
		return err
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

	if err := aaparser.LoadProfile(apparmorProfilePath); err != nil {
		return err
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
