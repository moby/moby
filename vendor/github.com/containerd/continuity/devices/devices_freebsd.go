package devices

// from /usr/include/sys/types.h

func getmajor(dev uint32) uint64 {
	return (uint64(dev) >> 24) & 0xff
}

func getminor(dev uint32) uint64 {
	return uint64(dev) & 0xffffff
}

func makedev(major int, minor int) int {
	return ((major << 24) | minor)
}
