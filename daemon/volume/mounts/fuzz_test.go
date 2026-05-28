package mounts

import (
	"testing"
)

func FuzzParseLinux(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewLinuxParser()
		if p, ok := parser.(*linuxParser); ok {
			p.fi = mockFiProvider{}
		}
		_, _ = parser.ParseMountRaw(string(data), "local")
	})
}
