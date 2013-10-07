package docker

type DeviceSet interface {
	AddDevice(hash, baseHash string) error
	SetInitialized(hash string) error
	DeactivateDevice(hash string) error
	RemoveDevice(hash string) error
	MountDevice(hash, path string) error
	UnmountDevice(hash, path string, deactivate bool) error
	HasDevice(hash string) bool
	HasInitializedDevice(hash string) bool
	HasActivatedDevice(hash string) bool
	Shutdown() error
}
