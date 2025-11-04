package daemon

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/graphdriver"
)

type imageStoreChoice string

const (
	// containerd by default
	imageStoreChoiceContainerd imageStoreChoice = "containerd-by-default"

	// containerd explicitly chosen by user
	imageStoreChoiceContainerdExplicit imageStoreChoice = "containerd-explicit"

	// graphdriver by default
	imageStoreChoiceGraphdriver imageStoreChoice = "graphdriver-by-default"

	// graphdrivers explicitly chosen by user
	imageStoreChoiceGraphdriverExplicit imageStoreChoice = "graphdriver-explicit"

	// would be containerd, but the system has already been running with a graphdriver
	imageStoreChoiceGraphdriverPrior imageStoreChoice = "graphdriver-prior"
)

func (c imageStoreChoice) IsGraphDriver() bool {
	switch c {
	case imageStoreChoiceContainerd, imageStoreChoiceContainerdExplicit:
		return false
	default:
		return true
	}
}

func (c imageStoreChoice) IsExplicit() bool {
	switch c {
	case imageStoreChoiceGraphdriverExplicit, imageStoreChoiceContainerdExplicit:
		return true
	default:
		return false
	}
}

// chooseDriver determines the storage driver name based on environment variables,
// configuration, and platform-specific logic.
// On Windows we don't support the environment variable, or a user supplied graphdriver,
// but it is allowed when using snapshotters.
// Unix platforms however run a single graphdriver for all containers, and it can
// be set through an environment variable, a daemon start parameter, or chosen through
// initialization of the layerstore through driver priority order for example.
func chooseDriver(ctx context.Context, cfgGraphDriver string, imgStoreChoice imageStoreChoice) string {
	driverName := os.Getenv("DOCKER_DRIVER")
	if driverName == "" {
		driverName = cfgGraphDriver
	} else {
		log.G(ctx).Infof("Setting the storage driver from the $DOCKER_DRIVER environment variable (%s)", driverName)
	}
	if runtime.GOOS == "windows" {
		if driverName == "" {
			driverName = cfgGraphDriver
		}
		switch driverName {
		case "windows":
			// Docker WCOW snapshotters
		case "windowsfilter":
			// Docker WCOW graphdriver
		case "":
			if imgStoreChoice.IsGraphDriver() {
				driverName = "windowsfilter"
			} else {
				driverName = "windows"
			}
		default:
			log.G(ctx).Infof("Using non-default snapshotter %s", driverName)

		}
	}
	return driverName
}

func determineImageStoreChoice(cfgStore *config.Config) (imageStoreChoice, error) {
	out := imageStoreChoiceContainerd
	if runtime.GOOS == "windows" {
		out = imageStoreChoiceGraphdriver
	}

	driverName := os.Getenv("DOCKER_DRIVER")
	if driverName == "" {
		driverName = cfgStore.GraphDriver
	}

	if enabled, ok := cfgStore.Features["containerd-snapshotter"]; ok {
		if enabled {
			if out == imageStoreChoiceContainerd {
				log.G(context.TODO()).Warn(`"containerd-snapshotter" is now the default and no longer needed to be set`)
			}
			out = imageStoreChoiceContainerdExplicit
		} else {
			out = imageStoreChoiceGraphdriverExplicit
		}
	}

	if os.Getenv("TEST_INTEGRATION_USE_GRAPHDRIVER") != "" {
		out = imageStoreChoiceGraphdriverExplicit
	}

	if driverName != "" {
		if !out.IsExplicit() {
			switch driverName {
			case "vfs", "overlay2":
				out = imageStoreChoiceGraphdriverExplicit
			case "btrfs":
				// The btrfs driver is not heavily used in containerd and has no
				// advantage over overlayfs anymore since overlay works fine.
				// If btrfs is explicitly chosen, the user most likely means graphdrivers.
				out = imageStoreChoiceGraphdriverExplicit
			}
		}
		if out.IsGraphDriver() {
			if graphdriver.IsRegistered(driverName) {
				return imageStoreChoiceGraphdriverExplicit, nil
			} else {
				return imageStoreChoiceGraphdriverExplicit, fmt.Errorf("graphdriver is explicitly enabled but %q is not registered, %v %v", driverName, cfgStore.Features, os.Getenv("TEST_INTEGRATION_USE_GRAPHDRIVER"))
			}
		}

		if runtime.GOOS == "windows" && !out.IsExplicit() {
			switch driverName {
			case "windows":
				return imageStoreChoiceContainerdExplicit, nil
			case "windowsfilter":
				return imageStoreChoiceGraphdriverExplicit, nil
			}
		}

		// Assume snapshotter is chosen
		return imageStoreChoiceContainerdExplicit, nil
	}

	if out == imageStoreChoiceContainerd {
		if graphdriver.HasPriorDriver(cfgStore.Root) {
			return imageStoreChoiceGraphdriverPrior, nil
		}
	}

	return out, nil
}
