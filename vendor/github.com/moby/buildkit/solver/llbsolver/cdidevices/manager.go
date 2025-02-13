package cdidevices

import (
	"context"
	"strings"

	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/locker"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"tags.cncf.io/container-device-interface/pkg/cdi"
	"tags.cncf.io/container-device-interface/pkg/parser"
)

const deviceAnnotationClass = "org.mobyproject.buildkit.device.class"

var installers = map[string]Setup{}

type Setup interface {
	Validate() error
	Run(ctx context.Context) error
}

// Register registers new setup for a device.
func Register(name string, setup Setup) {
	installers[name] = setup
}

type Device struct {
	Name        string
	AutoAllow   bool
	OnDemand    bool
	Annotations map[string]string
}

type Manager struct {
	cache  *cdi.Cache
	locker *locker.Locker
}

func NewManager(cache *cdi.Cache) *Manager {
	return &Manager{
		cache:  cache,
		locker: locker.New(),
	}
}

func (m *Manager) ListDevices() []Device {
	devs := m.cache.ListDevices()
	out := make([]Device, 0, len(devs))
	kinds := make(map[string]struct{})
	for _, dev := range devs {
		kind, _, _ := strings.Cut(dev, "=")
		dd := m.cache.GetDevice(dev)
		out = append(out, Device{
			Name:        dev,
			AutoAllow:   true, // TODO
			Annotations: deviceAnnotations(dd),
		})
		kinds[kind] = struct{}{}
	}

	for k, setup := range installers {
		if _, ok := kinds[k]; ok {
			continue
		}
		if err := setup.Validate(); err != nil {
			continue
		}
		out = append(out, Device{
			Name:     k,
			OnDemand: true,
		})
	}

	return out
}

func (m *Manager) Refresh() error {
	return m.cache.Refresh()
}

func (m *Manager) InjectDevices(spec *specs.Spec, devs ...*pb.CDIDevice) error {
	pdevs, err := m.parseDevices(devs...)
	if err != nil {
		return err
	} else if len(pdevs) == 0 {
		return nil
	}
	bklog.G(context.TODO()).Debugf("Injecting devices %v", pdevs)
	_, err = m.cache.InjectDevices(spec, pdevs...)
	return err
}

func (m *Manager) parseDevices(devs ...*pb.CDIDevice) ([]string, error) {
	var out []string
	for _, dev := range devs {
		if dev == nil {
			continue
		}
		pdev, err := m.parseDevice(dev)
		if err != nil {
			return nil, err
		} else if len(pdev) == 0 {
			continue
		}
		out = append(out, pdev...)
	}
	return dedupSlice(out), nil
}

func (m *Manager) parseDevice(dev *pb.CDIDevice) ([]string, error) {
	var out []string

	kind, name, _ := strings.Cut(dev.Name, "=")

	// validate kind
	if vendor, class := parser.ParseQualifier(kind); vendor == "" {
		return nil, errors.Errorf("invalid device %q", dev.Name)
	} else if err := parser.ValidateVendorName(vendor); err != nil {
		return nil, errors.Wrapf(err, "invalid device %q", dev.Name)
	} else if err := parser.ValidateClassName(class); err != nil {
		return nil, errors.Wrapf(err, "invalid device %q", dev.Name)
	}

	switch name {
	case "":
		// first device of kind if no name is specified
		for _, d := range m.cache.ListDevices() {
			if strings.HasPrefix(d, kind+"=") {
				out = append(out, d)
				break
			}
		}
	case "*":
		// all devices of kind if the name is a wildcard
		for _, d := range m.cache.ListDevices() {
			if strings.HasPrefix(d, kind+"=") {
				out = append(out, d)
			}
		}
	default:
		// the specified device
		for _, d := range m.cache.ListDevices() {
			if d == dev.Name {
				out = append(out, d)
				break
			}
		}
		if len(out) == 0 {
			// check class annotation if name unknown
			for _, d := range m.cache.ListDevices() {
				if !strings.HasPrefix(d, kind+"=") {
					continue
				}
				if a := deviceAnnotations(m.cache.GetDevice(d)); a != nil {
					if class, ok := a[deviceAnnotationClass]; ok && class == name {
						out = append(out, d)
					}
				}
			}
		}
	}

	if len(out) == 0 {
		if !dev.Optional {
			return nil, errors.Errorf("required device %q is not registered", dev.Name)
		}
		bklog.G(context.TODO()).Warnf("Optional device %q is not registered", dev.Name)
	}
	return out, nil
}

func (m *Manager) hasDevice(name string) bool {
	for _, d := range m.cache.ListDevices() {
		kind, _, _ := strings.Cut(d, "=")
		if kind == name {
			return true
		}
	}
	return false
}

func (m *Manager) OnDemandInstaller(name string) (func(context.Context) error, bool) {
	name, _, _ = strings.Cut(name, "=")

	installer, ok := installers[name]
	if !ok {
		return nil, false
	}

	if m.hasDevice(name) {
		return nil, false
	}

	return func(ctx context.Context) error {
		m.locker.Lock(name)
		defer m.locker.Unlock(name)

		if m.hasDevice(name) {
			return nil
		}

		if err := installer.Validate(); err != nil {
			return errors.Wrapf(err, "failed to find preconditions for %s device", name)
		}

		if err := installer.Run(ctx); err != nil {
			return errors.Wrapf(err, "failed to create %s device", name)
		}

		if err := m.cache.Refresh(); err != nil {
			return errors.Wrapf(err, "failed to refresh CDI cache")
		}

		return nil
	}, true
}

func deviceAnnotations(dev *cdi.Device) map[string]string {
	if dev == nil {
		return nil
	}
	out := make(map[string]string)
	// spec annotations
	for k, v := range dev.GetSpec().Annotations {
		out[k] = v
	}
	// device annotations
	for k, v := range dev.Device.Annotations {
		out[k] = v
	}
	return out
}

func dedupSlice(s []string) []string {
	if len(s) == 0 {
		return s
	}
	var res []string
	seen := make(map[string]struct{})
	for _, val := range s {
		if _, ok := seen[val]; !ok {
			res = append(res, val)
			seen[val] = struct{}{}
		}
	}
	return res
}
