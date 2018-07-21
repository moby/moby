// +build linux

package apparmor

import (
	"os"
	"testing"
)

func TestInstallDefault(t *testing.T) {
	const profile = "test-apparmor-default"
	const aapath = "/sys/kernel/security/apparmor/"

	if _, err := os.Stat(aapath); err != nil {
		t.Skip("AppArmor isn't available in this environment")
	}

	// removes `profile`
	removeProfile := func() error {
		path := aapath + ".remove"

		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.WriteString(profile)
		return err
	}

	// makes sure `profile` is loaded according to `state`
	checkLoaded := func(state bool) {
		loaded, err := IsLoaded(profile)
		if err != nil {
			t.Fatalf("Error searching AppArmor profile '%s': %v", profile, err)
		}
		if state != loaded {
			if state {
				t.Fatalf("AppArmor profile '%s' isn't loaded but should", profile)
			} else {
				t.Fatalf("AppArmor profile '%s' is loaded but shouldn't", profile)
			}
		}
	}

	// test installing the profile
	if err := InstallDefault(profile); err != nil {
		t.Fatalf("Couldn't install AppArmor profile '%s': %v", profile, err)
	}
	checkLoaded(true)

	// remove the profile and check again
	if err := removeProfile(); err != nil {
		t.Fatalf("Couldn't remove AppArmor profile '%s': %v", profile, err)
	}
	checkLoaded(false)
}
