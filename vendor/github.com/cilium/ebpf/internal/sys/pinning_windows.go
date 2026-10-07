package sys

import (
	"errors"
	"runtime"

	"github.com/cilium/ebpf/internal/efw"
)

func Pin(currentPath, newPath string, fd *FD) error {
	defer runtime.KeepAlive(fd)

	if newPath == "" {
		return errors.New("given pinning path cannot be empty")
	}
	if currentPath == newPath {
		return nil
	}

	if currentPath == "" {
		return ObjPin(&ObjPinAttr{
			Pathname: NewStringPointer(newPath),
			BpfFd:    fd.Uint(),
		})
	}

	return ObjPin(&ObjPinAttr{
		Pathname: NewStringPointer(newPath),
		BpfFd:    fd.Uint(),
	})
}

func Unpin(pinnedPath string) error {
	if pinnedPath == "" {
		return nil
	}

	err := efw.EbpfObjectUnpin(pinnedPath)
	if err != nil && !errors.Is(err, efw.EBPF_KEY_NOT_FOUND) {
		return err
	}

	return nil
}
