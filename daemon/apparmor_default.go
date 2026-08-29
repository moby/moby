//go:build linux

package daemon

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/rootless"
	aaprofile "github.com/moby/profiles/apparmor"
)

// Define constants for native driver
const (
	unconfinedAppArmorProfile = "unconfined"
	defaultAppArmorProfile    = "docker-default"
)

// DefaultApparmorProfile returns the name of the default apparmor profile
func DefaultApparmorProfile() string {
	if appArmorSupported() {
		return defaultAppArmorProfile
	}
	return ""
}

type appArmorProfileData struct {
	Abi           string
	Name          string
	DaemonProfile string
	Imports       []string
	InnerImports  []string
}

func (daemon *Daemon) loadDefaultAppArmorProfileIfMissing() error {
	if !defaultAppArmorProfileSupported() {
		return nil
	}

	loaded, err := aaprofile.IsLoaded(defaultAppArmorProfile)
	if err != nil {
		return fmt.Errorf("Could not check if %s AppArmor profile was loaded: %s", defaultAppArmorProfile, err)
	}
	if loaded {
		return nil
	}

	return daemon.installDefaultAppArmorProfile()
}

func (daemon *Daemon) installDefaultAppArmorProfile() error {
	if !defaultAppArmorProfileSupported() {
		return nil
	}

	if daemon.appArmorProfile == nil {
		if err := aaprofile.InstallDefault(defaultAppArmorProfile); err != nil {
			return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded: %s", defaultAppArmorProfile, err)
		}
		return nil
	}

	if err := daemon.installCustomAppArmorProfile(); err != nil {
		return fmt.Errorf("AppArmor enabled on system but the %s profile could not be loaded from %s: %s", defaultAppArmorProfile, daemon.appArmorProfilePath, err)
	}
	return nil
}

func (daemon *Daemon) installCustomAppArmorProfile() error {
	profile, err := daemon.generateCustomAppArmorProfile()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(context.Background(), "apparmor_parser", "-Kr")
	cmd.Stdin = profile
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running '%s' failed with output: %s\nerror: %w", cmd, out, err)
	}
	return nil
}

func (daemon *Daemon) generateCustomAppArmorProfile() (*bytes.Buffer, error) {
	data := appArmorProfileData{
		Name:          defaultAppArmorProfile,
		DaemonProfile: daemonAppArmorProfile(),
	}

	const abi = "abi/3.0"
	if appArmorMacroExists(abi) {
		data.Abi = abi
	}
	if appArmorMacroExists("tunables/global") {
		data.Imports = append(data.Imports, "#include <tunables/global>")
	} else {
		data.Imports = append(data.Imports, "@{PROC}=/proc/")
	}
	if appArmorMacroExists("abstractions/base") {
		data.InnerImports = append(data.InnerImports, "#include <abstractions/base>")
	}

	var profile bytes.Buffer
	if err := daemon.appArmorProfile.Execute(&profile, data); err != nil {
		return nil, err
	}
	return &profile, nil
}

func daemonAppArmorProfile() string {
	currentProfile, err := os.ReadFile("/proc/self/attr/current")
	if err != nil {
		log.G(context.TODO()).Warnf("Could not get daemon AppArmor profile: %s", err)
		return "unconfined"
	}
	profile, _, _ := strings.Cut(strings.TrimSpace(string(currentProfile)), " (")
	if profile == "" {
		return "unconfined"
	}
	return profile
}

func appArmorMacroExists(m string) bool {
	_, err := os.Stat(filepath.Join("/etc/apparmor.d", m))
	return err == nil
}

func defaultAppArmorProfileSupported() bool {
	hostSupports := appArmorSupported()
	if hostSupports {
		if detachedNetNS, _ := rootless.DetachedNetNS(); detachedNetNS != "" {
			// "open /sys/kernel/security/apparmor/profiles: permission denied"
			// (because sysfs is netns-scoped)
			hostSupports = false
		}
	}

	return hostSupports
}
