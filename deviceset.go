package docker

type DeviceSet interface {
	AddDevice(hash, baseHash string) error
	SetInitialized(hash string) error
	DeactivateDevice(hash string) error
	RemoveDevice(hash string) error
	MountDevice(hash, path string) error
	UnmountDevice(hash, path string) error
	HasDevice(hash string) bool
	HasInitializedDevice(hash string) bool
	HasActivatedDevice(hash string) bool
	Shutdown() error
}

type DeviceSetWrapper struct {
	wrapped DeviceSet
	prefix  string
}

func (wrapper *DeviceSetWrapper) wrap(hash string) string {
	if hash != "" {
		hash = wrapper.prefix + "-" + hash
	}
	return hash
}

func (wrapper *DeviceSetWrapper) AddDevice(hash, baseHash string) error {
	return wrapper.wrapped.AddDevice(wrapper.wrap(hash), wrapper.wrap(baseHash))
}

func (wrapper *DeviceSetWrapper) SetInitialized(hash string) error {
	return wrapper.wrapped.SetInitialized(wrapper.wrap(hash))
}

func (wrapper *DeviceSetWrapper) DeactivateDevice(hash string) error {
	return wrapper.wrapped.DeactivateDevice(wrapper.wrap(hash))
}

func (wrapper *DeviceSetWrapper) Shutdown() error {
	return nil
}

func (wrapper *DeviceSetWrapper) RemoveDevice(hash string) error {
	return wrapper.wrapped.RemoveDevice(wrapper.wrap(hash))
}

func (wrapper *DeviceSetWrapper) MountDevice(hash, path string) error {
	return wrapper.wrapped.MountDevice(wrapper.wrap(hash), path)
}

func (wrapper *DeviceSetWrapper) UnmountDevice(hash, path string) error {
	return wrapper.wrapped.UnmountDevice(wrapper.wrap(hash), path)
}

func (wrapper *DeviceSetWrapper) HasDevice(hash string) bool {
	return wrapper.wrapped.HasDevice(wrapper.wrap(hash))
}

func (wrapper *DeviceSetWrapper) HasInitializedDevice(hash string) bool {
	return wrapper.wrapped.HasInitializedDevice(wrapper.wrap(hash))
}

func (wrapper *DeviceSetWrapper) HasActivatedDevice(hash string) bool {
	return wrapper.wrapped.HasActivatedDevice(wrapper.wrap(hash))
}

func NewDeviceSetWrapper(wrapped DeviceSet, prefix string) DeviceSet {
	wrapper := &DeviceSetWrapper{
		wrapped: wrapped,
		prefix:  prefix,
	}
	return wrapper
}
