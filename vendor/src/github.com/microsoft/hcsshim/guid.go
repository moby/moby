package hcsshim

import (
	"crypto/sha1"
	"fmt"
)

type GUID [16]byte

func NewGUID(source string) *GUID {
	h := sha1.Sum([]byte(source))
	var g GUID
	copy(g[0:], h[0:16])
	return &g
}

func (g *GUID) ToString() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", g[0:4], g[4:6], g[6:8], g[8:10], g[10:])
}
