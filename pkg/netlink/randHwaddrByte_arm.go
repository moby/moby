// +build linux,arm

package netlink

import "math/rand"

func randHwaddrByte() uint8 {
    // gcc:arm char is a unsigned integer, limit 255
    return uint8(rand.Intn(255))
}
