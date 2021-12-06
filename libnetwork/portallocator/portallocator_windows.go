package portallocator

const (
	// DefaultPortRangeStart indicates the first port in port range
	DefaultPortRangeStart = 60000
	// DefaultPortRangeEnd indicates the last port in port range
	DefaultPortRangeEnd = 65000
)

func getDynamicPortRange() (start int, end int, err error) {
	return DefaultPortRangeStart, DefaultPortRangeEnd, nil
}
