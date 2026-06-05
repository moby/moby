//go:build linux

package daemon

import (
	"fmt"
	"os"
	"text/template"

	"github.com/moby/moby/v2/daemon/config"
)

func (daemon *Daemon) setupAppArmorProfile(cfg *config.Config) error {
	if cfg.AppArmorProfile == "" {
		return nil
	}

	daemon.appArmorProfilePath = cfg.AppArmorProfile
	b, err := os.ReadFile(daemon.appArmorProfilePath)
	if err != nil {
		return fmt.Errorf("opening AppArmor profile (%s) failed: %v", daemon.appArmorProfilePath, err)
	}
	tmpl, err := template.New("apparmor_profile").Option("missingkey=error").Parse(string(b))
	if err != nil {
		return fmt.Errorf("parsing AppArmor profile (%s) failed: %v", daemon.appArmorProfilePath, err)
	}
	daemon.appArmorProfile = tmpl
	return nil
}
