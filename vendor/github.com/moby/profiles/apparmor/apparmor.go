//go:build linux

package apparmor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"
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

// macroExists checks if the passed macro exists.
func macroExists(m string) bool {
	_, err := os.Stat(path.Join(profileDirectory, m))
	return err == nil
}

// InstallDefault generates a default profile in a temp directory determined by
// os.TempDir(), then loads the profile into the kernel using 'apparmor_parser'.
func InstallDefault(name string) error {
	// Figure out the daemon profile.
	daemonProfile := "unconfined"
	if currentProfile, err := os.ReadFile("/proc/self/attr/current"); err == nil {
		// Normally profiles are suffixed by " (enforcing)" or similar. AppArmor
		// profiles cannot contain spaces so this doesn't restrict daemon profile
		// names.
		if profile, _, _ := strings.Cut(string(currentProfile), " "); profile != "" {
			daemonProfile = profile
		}
	}

	// Install to a temporary directory.
	tmpFile, err := os.CreateTemp("", name)
	if err != nil {
		return err
	}

	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	p := profileData{
		Name:          name,
		DaemonProfile: daemonProfile,
	}
	if err := p.generateDefault(tmpFile); err != nil {
		return err
	}

	return loadProfile(tmpFile.Name())
}

// IsLoaded checks if a profile with the given name has been loaded into the
// kernel.
func IsLoaded(name string) (bool, error) {
	return isLoaded(name, "/sys/kernel/security/apparmor/profiles")
}

func isLoaded(name string, fileName string) (bool, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if prefix, _, ok := strings.Cut(scanner.Text(), " "); ok && prefix == name {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

// loadProfile runs `apparmor_parser -Kr` on a specified apparmor profile to
// replace the profile. The `-K` is necessary to make sure that apparmor_parser
// doesn't try to write to a read-only filesystem.
func loadProfile(profilePath string) error {
	c := exec.Command("apparmor_parser", "-Kr", profilePath)
	c.Dir = ""

	if output, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("running '%s' failed with output: %s\nerror: %v", c, output, err)
	}

	return nil
}
