package sysx

import (
	"errors"
)

var unsupported = errors.New("extended attributes unsupported on OpenBSD")
