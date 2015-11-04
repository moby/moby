package daemon

import (
	"fmt"

	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
)

// createContainerPlatformSpecificSettings performs platform specific container create functionality
func (daemon *Daemon) createContainerPlatformSpecificSettings(container *Container, config *runconfig.Config, hostConfig *runconfig.HostConfig, img *image.Image) error {
	for spec := range config.Volumes {

		mp, err := volume.ParseMountSpec(spec, hostConfig.VolumeDriver)
		if err != nil {
			return fmt.Errorf("Unrecognised volume spec: %v", err)
		}

		// If the mountpoint doesn't have a name, generate one.
		if len(mp.Name) == 0 {
			mp.Name = stringid.GenerateNonCryptoID()
		}

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.isDestinationMounted(mp.Destination) {
			continue
		}

		volumeDriver := hostConfig.VolumeDriver
		if mp.Destination != "" && img != nil {
			if _, ok := img.ContainerConfig.Volumes[mp.Destination]; ok {
				// check for whether bind is not specified and then set to local
				if _, ok := container.MountPoints[mp.Destination]; !ok {
					volumeDriver = volume.DefaultDriverName
				}
			}
		}

		// Create the volume in the volume driver. If it doesn't exist,
		// a new one will be created.
		v, err := daemon.createVolume(mp.Name, volumeDriver, nil)
		if err != nil {
			return err
		}

		// FIXME Windows: This code block is present in the Linux version and
		// allows the contents to be copied to the container FS prior to it
		// being started. However, the function utilises the FollowSymLinkInScope
		// path which does not cope with Windows volume-style file paths. There
		// is a seperate effort to resolve this (@swernli), so this processing
		// is deferred for now. A case where this would be useful is when
		// a dockerfile includes a VOLUME statement, but something is created
		// in that directory during the dockerfile processing. What this means
		// on Windows for TP4 is that in that scenario, the contents will not
		// copied, but that's (somewhat) OK as HCS will bomb out soon after
		// at it doesn't support mapped directories which have contents in the
		// destination path anyway.
		//
		// Example for repro later:
		//   FROM windowsservercore
		//   RUN mkdir c:\myvol
		//   RUN copy c:\windows\system32\ntdll.dll c:\myvol
		//   VOLUME "c:\myvol"
		//
		// Then
		//   docker build -t vol .
		//   docker run -it --rm vol cmd  <-- This is where HCS will error out.
		//
		//	// never attempt to copy existing content in a container FS to a shared volume
		//	if v.DriverName() == volume.DefaultDriverName {
		//		if err := container.copyImagePathContent(v, mp.Destination); err != nil {
		//			return err
		//		}
		//	}

		// Add it to container.MountPoints
		container.addMountPointWithVolume(mp.Destination, v, mp.RW)
	}
	return nil
}
