package networkdriver

import (
	"errors"
)

var (
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	ErrNetworkOverlaps                = errors.New("requested network overlaps with existing network")
)
