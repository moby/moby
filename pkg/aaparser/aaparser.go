// Package aaparser is a convenience package interacting with `apparmor_parser`.
package aaparser // import "github.com/docker/docker/pkg/aaparser"

import (
	"fmt"
	"os/exec"
	"strings"
)

// LoadProfile runs `apparmor_parser -Kr` on a specified apparmor profile to
// replace the profile. The `-K` is necessary to make sure that apparmor_parser
// doesn't try to write to a read-only filesystem.
func LoadProfile(profilePath string) error {
	c := exec.Command("apparmor_parser", "-Kr", profilePath)
	c.Dir = ""

	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running `%s %s` failed with output: %s\nerror: %v", c.Path, strings.Join(c.Args, " "), output, err)
	}
	return nil
}
