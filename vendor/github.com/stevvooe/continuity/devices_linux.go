package continuity

// from /usr/include/linux/kdev_t.h

func getmajor(dev uint64) uint64 {
	return dev >> 8
}

func getminor(dev uint64) uint64 {
	return dev & 0xff
}

func makedev(major int, minor int) int {
	return ((major << 8) | minor)
}
