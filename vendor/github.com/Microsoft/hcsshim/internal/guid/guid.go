package guid

import (
	"crypto/rand"
	"fmt"
	"io"
)

type GUID [16]byte

func New() GUID {
	g := GUID{}
	_, err := io.ReadFull(rand.Reader, g[:])
	if err != nil {
		panic(err)
	}
	return g
}

func (g GUID) String() string {
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x-%02x", g[3], g[2], g[1], g[0], g[5], g[4], g[7], g[6], g[8:10], g[10:])
}
