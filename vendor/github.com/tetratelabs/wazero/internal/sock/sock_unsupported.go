//go:build plan9 || js || tinygo

package sock

// plan9/js doesn't declare these constants
const (
	SHUT_RD = 1 << iota
	SHUT_WR
	SHUT_RDWR = SHUT_RD | SHUT_WR
)
