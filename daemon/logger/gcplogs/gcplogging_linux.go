package gcplogs // import "github.com/moby/moby/daemon/logger/gcplogs"

import (
	"os"

	"github.com/moby/moby/dockerversion"
	"github.com/moby/moby/pkg/homedir"
	"github.com/sirupsen/logrus"
)

// ensureHomeIfIAmStatic ensure $HOME to be set if dockerversion.IAmStatic is "true".
// See issue #29344: gcplogs segfaults (static binary)
// If HOME is not set, logging.NewClient() will call os/user.Current() via oauth2/google.
// If compiling statically, make sure osusergo build tag is also used to prevent a segfault
// due to a glibc issue that won't be fixed in a short term
// (see golang/go#13470, https://sourceware.org/bugzilla/show_bug.cgi?id=19341).
// So we forcibly set HOME so as to avoid call to os/user/Current()
func ensureHomeIfIAmStatic() error {
	// Note: dockerversion.IAmStatic is only available for linux.
	// So we need to use them in this gcplogging_linux.go rather than in gcplogging.go
	if dockerversion.IAmStatic == "true" && os.Getenv("HOME") == "" {
		home := homedir.Get()
		logrus.Warnf("gcplogs requires HOME to be set for static daemon binary. Forcibly setting HOME to %s.", home)
		os.Setenv("HOME", home)
	}
	return nil
}
