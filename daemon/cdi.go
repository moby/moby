package daemon

import (
	"fmt"

	"github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/docker/docker/errdefs"
	"github.com/hashicorp/go-multierror"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type cdiHandler struct {
	registry *cdi.Cache
}

// RegisterCDIDriver registers the CDI device driver.
// The driver injects CDI devices into an incoming OCI spec and is called for DeviceRequests associated with CDI devices.
func RegisterCDIDriver(opts ...cdi.Option) {
	cache, err := cdi.NewCache(opts...)
	if err != nil {
		logrus.WithError(err).Error("CDI registry initialization failed")
		// We create a spec updater that always returns an error.
		// This error will be returned only when a CDI device is requested.
		// This ensures that daemon startup is not blocked by a CDI registry initialization failure.
		errorOnUpdateSpec := func(s *specs.Spec, dev *deviceInstance) error {
			return fmt.Errorf("CDI device injection failed due to registry initialization failure: %w", err)
		}
		driver := &deviceDriver{
			updateSpec: errorOnUpdateSpec,
		}
		registerDeviceDriver("cdi", driver)
		return
	}

	// We construct a spec updates that injects CDI devices into the OCI spec using the initialized registry.
	c := &cdiHandler{
		registry: cache,
	}

	driver := &deviceDriver{
		updateSpec: c.injectCDIDevices,
	}

	registerDeviceDriver("cdi", driver)
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
			logrus.WithError(rerrs).Warning("Refreshing the CDI registry generated errors")
		}

		return fmt.Errorf("CDI device injection failed: %w", err)
	}

	return nil
}

// getErrors returns a single error representation of errors that may have occurred while refreshing the CDI registry.
func (c *cdiHandler) getErrors() error {
	errors := c.registry.GetErrors()

	var err *multierror.Error
	for _, errs := range errors {
		err = multierror.Append(err, errs...)
	}
	return err.ErrorOrNil()
}
