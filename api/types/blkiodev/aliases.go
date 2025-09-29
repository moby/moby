package blkiodev

import "github.com/moby/moby/api/types/blkiodev"

// WeightDevice is a structure that holds device:weight pair
type WeightDevice = blkiodev.WeightDevice

// ThrottleDevice is a structure that holds device:rate_per_second pair
type ThrottleDevice = blkiodev.ThrottleDevice
