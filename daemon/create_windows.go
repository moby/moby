package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Microsoft/hcsshim/osversion"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/image"
	volumemounts "github.com/docker/docker/volume/mounts"
	volumeopts "github.com/docker/docker/volume/service/opts"
	"github.com/sirupsen/logrus"
)

// createContainerOSSpecificSettings performs host-OS specific container create functionality
func (daemon *Daemon) createContainerOSSpecificSettings(container *container.Container, config *containertypes.Config, hostConfig *containertypes.HostConfig) error {
	if containertypes.Isolation.IsDefault(hostConfig.Isolation) {
		// Make sure the host config has the default daemon isolation if not specified by caller.
		hostConfig.Isolation = daemon.defaultIsolation
	}
	parser := volumemounts.NewParser()
	for spec := range config.Volumes {

		mp, err := parser.ParseMountRaw(spec, hostConfig.VolumeDriver)
		if err != nil {
			return fmt.Errorf("Unrecognised volume spec: %v", err)
		}

		// Skip volumes for which we already have something mounted on that
		// destination because of a --volume-from.
		if container.IsDestinationMounted(mp.Destination) {
			continue
		}

		volumeDriver := hostConfig.VolumeDriver

		// Create the volume in the volume driver. If it doesn't exist,
		// a new one will be created.
		v, err := daemon.volumes.Create(context.TODO(), "", volumeDriver, volumeopts.WithCreateReference(container.ID))
		if err != nil {
			return err
		}

		// FIXME Windows: This code block is present in the Linux version and
		// allows the contents to be copied to the container FS prior to it
		// being started. However, the function utilizes the FollowSymLinkInScope
		// path which does not cope with Windows volume-style file paths. There
		// is a separate effort to resolve this (@swernli), so this processing
		// is deferred for now. A case where this would be useful is when
		// a dockerfile includes a VOLUME statement, but something is created
		// in that directory during the dockerfile processing. What this means
		// on Windows for TP5 is that in that scenario, the contents will not
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
		//		if err := container.CopyImagePathContent(v, mp.Destination); err != nil {
		//			return err
		//		}
		//	}

		// Add it to container.MountPoints
		container.AddMountPointWithVolume(mp.Destination, &volumeWrapper{v: v, s: daemon.volumes}, mp.RW)
	}
	return nil
}

// checkImageIsolationCompatiblity checks whether the provided image is compatible
// with the host config's isolation settings.
func (daemon *Daemon) checkImageIsolationCompatiblity(hostConfig *containertypes.HostConfig, img *image.Image) error {
	hostOSV := osversion.Get()
	isHyperV := hostConfig.Isolation.IsHyperV()
	// fallback on default daemon isolation if none was explicitly provided:
	if hostConfig.Isolation.IsDefault() {
		isHyperV = daemon.defaultIsolation.IsHyperV()
	}
	return checkImageCompatibilityForHostIsolation(hostOSV, img.OSVersion, isHyperV)
}

// checkImageCompatibilityForHostIsolation checks whether the provided `imageOSVersion`
// should be able to run on the provided host OS version given Hyper-V isolation.
// Its contained logic can be distilled into:
// - if imageOS == hostOS => can run under any isolation mode
// - if imageOS < hostOS => must be run under Hyper-V
// - if imageOS > hostOS => can be run under Hyper-V only on an RS5 (1809+) host
// Please see the below:
// https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility?tabs=windows-server-2016%2Cwindows-10#windows-server-host-os-compatibility
func checkImageCompatibilityForHostIsolation(hostOSV osversion.OSVersion, imageOSVersion string, isHyperV bool) error {
	var err error
	var imageOSBuildU64 uint64
	splitImageOSVersion := strings.Split(imageOSVersion, ".") // eg 10.0.16299.nnnn
	if len(splitImageOSVersion) >= 3 {
		if imageOSBuildU64, err = strconv.ParseUint(splitImageOSVersion[2], 10, 16); err == nil {
			logrus.Debugf("parsed Windows build number %d from image OS version %s", imageOSBuildU64, imageOSVersion)
		} else {
			return fmt.Errorf("failed to ParseUint() Windows image build %q from image version %q: %s", splitImageOSVersion[2], imageOSVersion, err)
		}
	} else {
		return fmt.Errorf("failed to split and parse Windows image version %q (was expecting format like '10.0.16299.nnnn')", imageOSVersion)
	}
	truncatedImageOSVersion := fmt.Sprintf("%s.%s.%s", splitImageOSVersion[0], splitImageOSVersion[1], splitImageOSVersion[2])

	imageOSBuild := uint16(imageOSBuildU64)
	if imageOSBuild == hostOSV.Build {
		// same image version should run on identically-versioned host regardless of isolation:
		logrus.Debugf("image version %s is trivially compatible with host version %s", truncatedImageOSVersion, hostOSV.ToString())
		return nil
	} else if imageOSBuild < hostOSV.Build {
		// images older than the host must be run under Hyper-V:
		if isHyperV {
			logrus.Debugf("older image version %s is compatible with host version %s under Hyper-V", truncatedImageOSVersion, hostOSV.ToString())
			return nil
		} else {
			return fmt.Errorf("an older Windows version %s-based image can only be run on a %s host using Hyper-V isolation", truncatedImageOSVersion, hostOSV.ToString())
		}
	} else {
		// images newer than the host can only run if the host is RS5 and is using Hyper-V:
		if hostOSV.Build < osversion.RS5 {
			return fmt.Errorf("a Windows version %s-based image is incompatible with a %s host", truncatedImageOSVersion, hostOSV.ToString())
		} else {
			if isHyperV {
				logrus.Debugf("newer Windows image version %s is compatible with host version %s under Hyper-V", truncatedImageOSVersion, hostOSV.ToString())
				return nil
			} else {
				return fmt.Errorf("a newer Windows version %s-based image can only be run on a %s host using Hyper-V isolation", truncatedImageOSVersion, hostOSV.ToString())
			}
		}
	}
}
