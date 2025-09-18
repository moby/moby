package ipamapi

const (
	// Prefix constant marks the reserved label space for libnetwork
	Prefix = "com.docker.network"

	// AllocSerialPrefix constant marks the reserved label space for libnetwork ipam
	// allocation ordering.(serial/first available)
	AllocSerialPrefix = Prefix + ".ipam.serial"

	// SubnetSizeOption allows a user to specify a desired subnet size for a given
	// network from the default pool.
	SubnetSizeOption = Prefix + ".subnet_size"
)
