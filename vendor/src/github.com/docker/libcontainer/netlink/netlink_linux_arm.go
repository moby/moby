package netlink

import (
	"math/rand"
)

func randIfrDataByte() uint8 {
	return uint8(rand.Intn(255))
}
