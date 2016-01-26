// +build linux

package apparmor

import (
	"bufio"
	"io"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/docker/docker/pkg/aaparser"
)

var (
	// profileDirectory is the file store for apparmor profiles and macros.
	profileDirectory = "/etc/apparmor.d"
	// defaultProfilePath is the default path for the apparmor profile to be saved.
	defaultProfilePath = path.Join(profileDirectory, "docker")
)

// profileData holds information about the given profile for generation.
type profileData struct {
	// Name is profile name.
	Name string
	// ExecPath is the path to the docker binary.
	ExecPath string
	// Imports defines the apparmor functions to import, before defining the profile.
	Imports []string
	// InnerImports defines the apparmor functions to import in the profile.
	InnerImports []string
	// MajorVersion is the apparmor_parser major version.
	MajorVersion int
	// MinorVersion is the apparmor_parser minor version.
	MinorVersion int
}

// generateDefault creates an apparmor profile from ProfileData.
func (p *profileData) generateDefault(out io.Writer) error {
	compiled, err := template.New("apparmor_profile").Parse(baseTemplate)
	if err != nil {
		return err
	}
	if macroExists("tunables/global") {
		p.Imports = append(p.Imports, "#include <tunables/global>")
	} else {
		p.Imports = append(p.Imports, "@{PROC}=/proc/")
	}
	if macroExists("abstractions/base") {
		p.InnerImports = append(p.InnerImports, "#include <abstractions/base>")
	}
	if err := compiled.Execute(out, p); err != nil {
		return err
	}
	return nil
}

// macrosExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}

// InstallDefault generates a default profile and installs it in the
// ProfileDirectory with `apparmor_parser`.
func InstallDefault(name string) error {
	// Make sure the path where they want to save the profile exists
	if err := os.MkdirAll(profileDirectory, 0755); err != nil {
		return err
	}

	p := profileData{
		Name: name,
	}

	f, err := os.OpenFile(defaultProfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := p.generateDefault(f); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if err := aaparser.LoadProfile(defaultProfilePath); err != nil {
		return err
	}

	return nil
}

// IsLoaded checks if a passed profile as been loaded into the kernel.
func IsLoaded(name string) error {
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
		if strings.HasPrefix(p, name+" ") {
			return nil
		}
	}
}
