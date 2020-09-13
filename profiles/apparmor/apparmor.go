// +build linux

package apparmor // import "github.com/docker/docker/profiles/apparmor"

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/docker/docker/pkg/aaparser"
)

//go:generate go run generate.go

var (
	// profileDirectory is the file store for apparmor profiles and macros.
	profileDirectory = "/etc/apparmor.d"
)

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
	// Version is the {major, minor, patch} version of apparmor_parser as a single number.
	Version int
}

// macrosExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}

// execute evaluates the given profile using the provided profileData, filling
// in any fields with auto-detected data.
func (p *profileData) execute(tmpl *template.Template, out io.Writer) error {
	// Figure out the daemon profile name.
	currentProfile, err := ioutil.ReadFile("/proc/self/attr/current")
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

	// Fill the imports with the correct system defaults.
	if macroExists("tunables/global") {
		p.Imports = append(p.Imports, "#include <tunables/global>")
	} else {
		p.Imports = append(p.Imports, "@{PROC}=/proc/")
	}
	if macroExists("abstractions/base") {
		p.InnerImports = append(p.InnerImports, "#include <abstractions/base>")
	}

	// Fill the apparmor_parser version.
	ver, err := aaparser.GetVersion()
	if err != nil {
		return err
	}
	p.Version = ver

	return tmpl.Execute(out, p)
}

// InstallCustom installs a custom apparmor profile with the given name. The
// final profile is not saved to disk, because writing profiles to
// /etc/apparmor.d/ can lead to containers becoming unconfined on host package
// upgrades.
func InstallCustom(tmpl *template.Template, name string) error {
	p := profileData{
		Name: name,
	}

	// Install to a temporary directory.
	f, err := ioutil.TempFile("", name)
	if err != nil {
		return err
	}
	profilePath := f.Name()

	defer f.Close()
	defer os.Remove(profilePath)

	if err := p.execute(tmpl, f); err != nil {
		return err
	}

	return aaparser.LoadProfile(profilePath)
}

// InstallDefault is a wrapper around InstallCustom which uses the built-in
// default AppArmor profile.
func InstallDefault(name string) error {
	defaultTemplate, err := template.New("apparmor_profile").Parse(baseTemplate)
	if err != nil {
		return err
	}
	return InstallCustom(defaultTemplate, name)
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
