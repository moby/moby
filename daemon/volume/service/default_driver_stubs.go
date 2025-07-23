//go:build !linux && !windows

package service

import (
	"github.com/docker/docker/daemon/internal/idtools"
	"github.com/docker/docker/daemon/volume/drivers"
)

func setupDefaultDriver(_ *drivers.Store, _ string, _ idtools.Identity) error { return nil }
