package apparmor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
)

const (
	DefaultProfilePath = "/etc/apparmor.d/docker"
)

func InstallDefaultProfile(backupPath string) error {
	if !IsEnabled() {
		return nil
	}

	// If the profile already exists, check if we already have a backup
	// if not, do the backup and override it. (docker 0.10 upgrade changed the apparmor profile)
	// see gh#5049, apparmor blocks signals in ubuntu 14.04
	if _, err := os.Stat(DefaultProfilePath); err == nil {
		if _, err := os.Stat(backupPath); err == nil {
			// If both the profile and the backup are present, do nothing
			return nil
		}
		// Make sure the directory exists
		if err := os.MkdirAll(path.Dir(backupPath), 0755); err != nil {
			return err
		}

		// Create the backup file
		f, err := os.Create(backupPath)
		if err != nil {
			return err
		}
		defer f.Close()

		src, err := os.Open(DefaultProfilePath)
		if err != nil {
			return err
		}
		defer src.Close()

		if _, err := io.Copy(f, src); err != nil {
			return err
		}
	}

	// Make sure /etc/apparmor.d exists
	if err := os.MkdirAll(path.Dir(DefaultProfilePath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(DefaultProfilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := generateProfile(f); err != nil {
		f.Close()
		return err
	}
	f.Close()

	cmd := exec.Command("/sbin/apparmor_parser", "-r", "-W", "docker")
	// to use the parser directly we have to make sure we are in the correct
	// dir with the profile
	cmd.Dir = "/etc/apparmor.d"

	output, err := cmd.CombinedOutput()
	if err != nil && !os.IsNotExist(err) {
		if e, ok := err.(*exec.Error); ok {
			// keeping with the current profile load code, if the parser does not
			// exist then just return
			if e.Err == exec.ErrNotFound || os.IsNotExist(e.Err) {
				return nil
			}
		}
		return fmt.Errorf("Error loading docker profile: %s (%s)", err, output)
	}
	return nil
}
