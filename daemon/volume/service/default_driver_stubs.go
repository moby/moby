//go:build !linux && !windows

package service

import (
	"github.com/docker/docker/daemon/volume/drivers"
	"github.com/docker/docker/pkg/idtools"
)

func setupDefaultDriver(_ *drivers.Store, _ string, _ idtools.Identity) error { return nil }
