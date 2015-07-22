package sysinfo

// New returns an empty SysInfo for freebsd for now.
func New(quiet bool) *SysInfo {
	sysInfo := &SysInfo{}
	return sysInfo
}
