// +build linux

package manifest

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/homedir"
)

// ensureHomeIfIAmStatic ensure $HOME to be set if dockerversion.IAmStatic is "true".
// In a static binary, os/user.Current() leads to segfault due to a glibc issue that won't be fixed
// in the foreseeable future. (golang/go#13470, https://sourceware.org/bugzilla/show_bug.cgi?id=19341)
// So we forcibly set HOME so as to avoid call to os/user/Current()
func ensureHomeIfIAmStatic() error {
	// Note: dockerversion.IAmStatic and homedir.GetStatic() is only available for linux.
	if dockerversion.IAmStatic == "true" && os.Getenv("HOME") == "" {
		home, err := homedir.GetStatic()
		if err != nil {
			return err
		}
		logrus.Warnf("docker manifest requires HOME to be set for static client binary. Forcibly setting HOME to %s.", home)
		os.Setenv("HOME", home)
	}
	return nil
}
