package cgroups

func getClockTicks() uint64 {
	// The value comes from `C.sysconf(C._SC_CLK_TCK)`, and
	// on Linux it's a constant which is safe to be hard coded,
	// so we can avoid using cgo here.
	// See https://github.com/containerd/cgroups/pull/12 for
	// more details.
	return 100
}
