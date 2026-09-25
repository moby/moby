package wasip1

const (
	ClockResGetName  = "clock_res_get"
	ClockTimeGetName = "clock_time_get"
)

// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-clockid-enumu32
const (
	// ClockIDRealtime is the name ID named "realtime" like sys.Walltime
	ClockIDRealtime = iota
	// ClockIDMonotonic is the name ID named "monotonic" like sys.Nanotime
	ClockIDMonotonic
	// Note: clockIDProcessCputime and clockIDThreadCputime were removed by
	// WASI maintainers: https://github.com/WebAssembly/wasi-libc/pull/294
)
