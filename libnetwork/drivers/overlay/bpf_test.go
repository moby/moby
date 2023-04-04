package overlay

import (
	"testing"
)

func FuzzVNIMatchBPFDoesNotPanic(f *testing.F) {
	for _, seed := range []uint32{0, 1, 42, 0xfffffe, 0xffffff, 0xfffffffe, 0xffffffff} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, vni uint32) {
		_ = vniMatchBPF(vni)
	})
}
