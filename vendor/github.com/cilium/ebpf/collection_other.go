//go:build !windows

package ebpf

import "github.com/cilium/ebpf/internal"

func loadCollectionFromNativeImage(_ string) (*Collection, error) {
	return nil, internal.ErrNotSupportedOnOS
}
