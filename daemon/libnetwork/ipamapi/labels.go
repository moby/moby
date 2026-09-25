package ipamapi

const (
	// Prefix constant marks the reserved label space for libnetwork
	Prefix = "com.docker.network"

	// AllocSerialPrefix constant marks the reserved label space for libnetwork ipam
	// allocation ordering.(serial/first available)
	AllocSerialPrefix = Prefix + ".ipam.serial"
)
