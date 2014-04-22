// +build linux,amd64

package netlink

import "math/rand"

func randHwaddrByte() int8 {
	// gcc:amd64 char is a signed integer, limit 127
	return int8(rand.Intn(128))
}
