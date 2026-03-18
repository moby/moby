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

// getDriverOverride determines the storage driver name based on environment variables,
// configuration, and platform-specific logic.
// On Windows we don't support the environment variable, or a user supplied graphdriver,
// but it is allowed when using snapshotters.
// Unix platforms however run a single graphdriver for all containers, and it can
// be set through an environment variable, a daemon start parameter, or chosen through
// initialization of the layerstore through driver priority order for example.
func getDriverOverride(ctx context.Context, cfgGraphDriver string, imgStoreChoice imageStoreChoice) string {
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

type determineImageStoreChoiceOptions struct {
	hasPriorDriver          func(root string) bool
	isRegisteredGraphdriver func(driverName string) bool
	runtimeOS               string
}

func determineImageStoreChoice(cfgStore *config.Config, opts determineImageStoreChoiceOptions) (imageStoreChoice, error) {
	if opts.hasPriorDriver == nil {
		opts.hasPriorDriver = graphdriver.HasPriorDriver
	}
	if opts.isRegisteredGraphdriver == nil {
		opts.isRegisteredGraphdriver = graphdriver.IsRegistered
	}
	if opts.runtimeOS == "" {
		opts.runtimeOS = runtime.GOOS
	}

	out := imageStoreChoiceContainerd
	if opts.runtimeOS == "windows" {
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

	if out == imageStoreChoiceContainerd {
		if opts.hasPriorDriver(cfgStore.Root) {
			return imageStoreChoiceGraphdriverPrior, nil
		}
	}

	if driverName != "" {
		if !out.IsExplicit() && opts.isRegisteredGraphdriver(driverName) {
			return imageStoreChoiceGraphdriverExplicit, nil
		}
		if out.IsGraphDriver() {
			if opts.isRegisteredGraphdriver(driverName) {
				return imageStoreChoiceGraphdriverExplicit, nil
			} else if out.IsExplicit() {
				return imageStoreChoiceGraphdriverExplicit, fmt.Errorf("graphdriver is explicitly enabled but %q is not registered, %v %v", driverName, cfgStore.Features, os.Getenv("TEST_INTEGRATION_USE_GRAPHDRIVER"))
			}
		}

		// Assume snapshotter is chosen
		return imageStoreChoiceContainerdExplicit, nil
	}

	return out, nil
}
