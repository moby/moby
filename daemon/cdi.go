package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/log"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"tags.cncf.io/container-device-interface/pkg/cdi"
)

type cdiHandler struct {
	registry *cdi.Cache
}

// RegisterCDIDriver registers the CDI device driver.
// The driver injects CDI devices into an incoming OCI spec and is called for DeviceRequests associated with CDI devices.
// If the list of CDI spec directories is empty, the driver is not registered.
func RegisterCDIDriver(cdiSpecDirs ...string) *cdi.Cache {
	for i, dir := range cdiSpecDirs {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			cdiSpecDirs[i], err = filepath.EvalSymlinks(dir)
			if err != nil {
				log.L.WithField("dir", dir).WithError(err).Warn("Failed to evaluate symlinks for CDI spec directory")
			}
		}
	}
	driver, cache := newCDIDeviceDriver(cdiSpecDirs...)
	registerDeviceDriver("cdi", driver)
	return cache
}

// newCDIDeviceDriver creates a new CDI device driver.
// If the creation of the CDI cache fails, a driver is returned that will return an error on an injection request.
func newCDIDeviceDriver(cdiSpecDirs ...string) (*deviceDriver, *cdi.Cache) {
	cache, err := createCDICache(cdiSpecDirs...)
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("Failed to create CDI cache")
		// We create a spec updater that always returns an error.
		// This error will be returned only when a CDI device is requested.
		// This ensures that daemon startup is not blocked by a CDI registry initialization failure or being disabled
		// by configuration.
		errorOnUpdateSpec := func(s *specs.Spec, dev *deviceInstance) error {
			return fmt.Errorf("CDI device injection failed: %w", err)
		}
		return &deviceDriver{
			updateSpec: errorOnUpdateSpec,
			ListDevices: func(ctx context.Context, cfg *config.Config) (deviceListing, error) {
				return deviceListing{
					Warnings: []string{fmt.Sprintf("CDI cache initialization failed: %v", err)},
				}, nil
			},
		}, nil
	}

	// We construct a spec updates that injects CDI devices into the OCI spec using the initialized registry.
	c := &cdiHandler{
		registry: cache,
	}

	return &deviceDriver{
		updateSpec:  c.injectCDIDevices,
		ListDevices: c.listDevices,
	}, cache
}

// createCDICache creates a CDI cache for the specified CDI specification directories.
// If the list of CDI specification directories is empty or the creation of the CDI cache fails, an error is returned.
func createCDICache(cdiSpecDirs ...string) (*cdi.Cache, error) {
	if len(cdiSpecDirs) == 0 {
		return nil, errors.New("no CDI specification directories specified")
	}

	cache, err := cdi.NewCache(cdi.WithSpecDirs(cdiSpecDirs...))
	if err != nil {
		return nil, fmt.Errorf("CDI registry initialization failure: %w", err)
	}

	for dir, errs := range cache.GetErrors() {
		for _, err := range errs {
			if errors.Is(err, os.ErrNotExist) {
				log.L.WithField("dir", dir).Infof("CDI directory does not exist, skipping: %v", err)
				continue
			}
			log.L.WithField("dir", dir).Warnf("CDI setup error: %+v", err)
		}
	}

	return cache, nil
}

// injectCDIDevices injects a set of CDI devices into the specified OCI specification.
func (c *cdiHandler) injectCDIDevices(s *specs.Spec, dev *deviceInstance) error {
	if dev.req.Count != 0 {
		return errdefs.InvalidParameter(errors.New("unexpected count in CDI device request"))
	}
	if len(dev.req.Options) > 0 {
		return errdefs.InvalidParameter(errors.New("unexpected options in CDI device request"))
	}

	cdiDeviceNames := dev.req.DeviceIDs
	if len(cdiDeviceNames) == 0 {
		return nil
	}

	_, err := c.registry.InjectDevices(s, cdiDeviceNames...)
	if err != nil {
		if rerrs := c.getErrors(); rerrs != nil {
			// We log the errors that may have been generated while refreshing the CDI registry.
			// These may be due to malformed specifications or device name conflicts that could be
			// the cause of an injection failure.
			log.G(context.TODO()).WithError(rerrs).Warning("Refreshing the CDI registry generated errors")
		}

		return fmt.Errorf("CDI device injection failed: %w", err)
	}

	return nil
}

// getErrors returns a single error representation of errors that may have occurred while refreshing the CDI registry.
func (c *cdiHandler) getErrors() error {
	var errs []error
	for _, es := range c.registry.GetErrors() {
		errs = append(errs, es...)
	}
	return errors.Join(errs...)
}

// listDevices uses the CDI cache to list all discovered CDI devices.
// It conforms to the deviceDriver.ListDevices function signature.
func (c *cdiHandler) listDevices(ctx context.Context, cfg *config.Config) (deviceListing, error) {
	var out deviceListing

	// Collect global errors from the CDI cache (e.g., issues with spec files themselves).
	for specPath, specErrs := range c.registry.GetErrors() {
		for _, err := range specErrs {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			out.Warnings = append(out.Warnings, fmt.Sprintf("CDI: Error associated with spec file %s: %v", specPath, err))
		}
	}

	qualifiedDeviceNames := c.registry.ListDevices()
	if len(qualifiedDeviceNames) == 0 {
		return out, nil
	}

	for _, qdn := range qualifiedDeviceNames {
		device := c.registry.GetDevice(qdn)
		if device == nil {
			log.G(ctx).WithField("device", qdn).Warn("CDI: Cache.GetDevice() returned nil for a listed device, skipping.")
			out.Warnings = append(out.Warnings, fmt.Sprintf("CDI: Device %s listed but not found by GetDevice()", qdn))
			continue
		}

		out.Devices = append(out.Devices, system.DeviceInfo{
			ID: qdn,
		})
	}

	return out, nil
}
