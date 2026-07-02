//go:build linux

package daemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/v2/daemon/config"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"
)

func TestSetupAppArmorProfile(t *testing.T) {
	profile := fs.NewFile(t, "profile", fs.WithContent(`profile "{{.Name}}" {}`))

	d := &Daemon{}
	err := d.setupAppArmorProfile(&config.Config{AppArmorProfile: profile.Path()})
	assert.NilError(t, err)
	assert.Equal(t, d.appArmorProfilePath, profile.Path())
	assert.Assert(t, d.appArmorProfile != nil)
}

func TestSetupAppArmorProfileErrors(t *testing.T) {
	d := &Daemon{}
	err := d.setupAppArmorProfile(&config.Config{AppArmorProfile: filepath.Join(t.TempDir(), "missing")})
	assert.ErrorContains(t, err, "opening AppArmor profile")

	profile := fs.NewFile(t, "profile", fs.WithContent(`profile "{{.Name}" {}`))
	err = d.setupAppArmorProfile(&config.Config{AppArmorProfile: profile.Path()})
	assert.ErrorContains(t, err, "parsing AppArmor profile")
}

func TestGenerateCustomAppArmorProfile(t *testing.T) {
	profile := fs.NewFile(t, "profile", fs.WithContent(`{{range .Imports}}{{.}}
{{end}}profile "{{.Name}}" {
  signal (receive) peer="{{.DaemonProfile}}",
}`))

	d := &Daemon{}
	err := d.setupAppArmorProfile(&config.Config{AppArmorProfile: profile.Path()})
	assert.NilError(t, err)

	generated, err := d.generateCustomAppArmorProfile()
	assert.NilError(t, err)
	out := generated.String()
	assert.Assert(t, strings.Contains(out, `profile "docker-default"`))
	assert.Assert(t, strings.Contains(out, `@{PROC}=/proc/`))
}

func TestGenerateCustomAppArmorProfileMissingKey(t *testing.T) {
	profile := fs.NewFile(t, "profile", fs.WithContent(`profile "{{.Unknown}}" {}`))

	d := &Daemon{}
	err := d.setupAppArmorProfile(&config.Config{AppArmorProfile: profile.Path()})
	assert.NilError(t, err)

	_, err = d.generateCustomAppArmorProfile()
	assert.ErrorContains(t, err, `can't evaluate field Unknown`)
}
