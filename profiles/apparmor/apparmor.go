//go:build linux
// +build linux

package apparmor // import "github.com/docker/docker/profiles/apparmor"

import (
	"bufio"
	"io"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/docker/docker/pkg/aaparser"
)

// profileDirectory is the file store for apparmor profiles and macros.
const profileDirectory = "/etc/apparmor.d"

// profileData holds information about the given profile for generation.
type profileData struct {
	// Name is profile name.
	Name string
	// DaemonProfile is the profile name of our daemon.
	DaemonProfile string
	// Imports defines the apparmor functions to import, before defining the profile.
	Imports []string
	// InnerImports defines the apparmor functions to import in the profile.
	InnerImports []string
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

	return compiled.Execute(out, p)
}

// macrosExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}

// InstallDefault generates a default profile in a temp directory determined by
// os.TempDir(), then loads the profile into the kernel using 'apparmor_parser'.
func InstallDefault(name string) error {
	p := profileData{
		Name: name,
	}

	// Figure out the daemon profile.
	currentProfile, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		// If we couldn't get the daemon profile, assume we are running
		// unconfined which is generally the default.
		currentProfile = nil
	}
	daemonProfile := string(currentProfile)
	// Normally profiles are suffixed by " (enforcing)" or similar. AppArmor
	// profiles cannot contain spaces so this doesn't restrict daemon profile
	// names.
	if parts := strings.SplitN(daemonProfile, " ", 2); len(parts) >= 1 {
		daemonProfile = parts[0]
	}
	if daemonProfile == "" {
		daemonProfile = "unconfined"
	}
	p.DaemonProfile = daemonProfile

	// Install to a temporary directory.
	f, err := os.CreateTemp("", name)
	if err != nil {
		return err
	}
	profilePath := f.Name()

	defer f.Close()
	defer os.Remove(profilePath)

	if err := p.generateDefault(f); err != nil {
		return err
	}

	return aaparser.LoadProfile(profilePath)
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
func IsLoaded(name string) (bool, error) {
	file, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return false, err
	}
	defer file.Close()

	r := bufio.NewReader(file)
	for {
		p, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if strings.HasPrefix(p, name+" ") {
			return true, nil
		}
	}

	return false, nil
}
