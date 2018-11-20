package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"runtime"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/stringid"
	volumeopts "github.com/docker/docker/volume/service/opts"
)

// createContainerOSSpecificSettings performs host-OS specific container create functionality
func (daemon *Daemon) createContainerOSSpecificSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if container.OS == runtime.GOOS {
		// Make sure the host config has the default daemon isolation if not specified by caller.
		if containertypes.Isolation.IsDefault(containertypes.Isolation(hostConfig.Isolation)) {
			hostConfig.Isolation = daemon.defaultIsolation
		}
	} else {
		// LCOW must be a Hyper-V container as you can't run a shared kernel when one
		// is a Windows kernel, the other is a Linux kernel.
		if containertypes.Isolation.IsProcess(containertypes.Isolation(hostConfig.Isolation)) {
			return fmt.Errorf("process isolation is invalid for Linux containers on Windows")
		}
		hostConfig.Isolation = "hyperv"
	}

	for spec := range config.Volumes {
		var destination string
		if container.OS == runtime.GOOS {
			// We do a filepath.FromSlash here to not break "legacy" Windows
			// dockerfiles use Unix-style paths such as "VOLUME c:/somevolume".
			// If we don't do this, HCS will balk.
			destination = filepath.Clean(filepath.FromSlash(spec))
		} else {
			destination = path.Clean(spec)
		}

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.IsDestinationMounted(destination) {
			continue
		}

		// Create the volume in the volume driver.
		v, err := daemon.volumes.Create(
			context.TODO(),
			stringid.GenerateNonCryptoID(),
			hostConfig.VolumeDriver,
			volumeopts.WithCreateReference(container.ID))
		if err != nil {
			return err
		}

		// Add it to container.MountPoints. Note that the last parameter is true - as in read-write
		container.AddMountPointWithVolume(destination, &volumeWrapper{v: v, s: daemon.volumes}, true)
	}

	// NOTE: On Linux, this function returns daemon.populateVolumes(container).
	// This allows the contents of a volume to be copied to the containers
	// filesystem prior to it be in started.
	//
	// There are many issues with solving this problem, and the reality is
	// that it will likely be one thing that can't be done on Windows.
	//
	// FollowSymLinkInScope doesn't cope with Windows volume-style file paths.
	// To avoid break outs we need to do scoped access. This is surmountable.
	//
	// Argons might be possible, but at this execution point on Windows,
	// the container filesystem isn't mounted as it's done much later than
	// on Linux.
	//
	// Xenons (both WCOW and LCOW) are difficult as we don't want to mount
	// the filesystem on the host, for both security and perf reasons.
	// Further, obviously the LCOW filesystem can't be mounted on the host
	// directly.
	//
	// What this means is that on Windows, the contents of a VOLUME created
	// in a Dockerfile with contents will NOT be copied.
	//
	// Pre ~RS3 had limitations in mapped directories which preclude actually
	// doing this in the platform anyway.
	//
	// Example for repro later:
	//   FROM windowsservercore
	//   RUN mkdir c:\myvol
	//   RUN copy c:\windows\system32\ntdll.dll c:\myvol
	//   VOLUME "c:\myvol"
	//
	// Then
	//   docker build -t vol .
	//   docker run -it --rm vol cmd
	//
	// Result
	//   RS1 to ~RS3 HCS would error out
	//   RS4+ Succeeds, container starts, but c:\source will be empty

	return nil
}
