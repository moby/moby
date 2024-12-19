package chrootarchive

import (
	"fmt"
	"io"

	"github.com/docker/docker/pkg/archive"
	"golang.org/x/sys/unix"
)

func doUnpack(decompressedArchive io.Reader, relDest, root string, options *archive.TarOptions) error {
	done := make(chan error)
	err := goInChroot(root, func() { done <- archive.Unpack(decompressedArchive, relDest, options) })
	if err != nil {
		return err
	}
	return <-done
}

func doPack(relSrc, root string, options *archive.TarOptions) (io.ReadCloser, error) {
	tb, err := archive.NewTarballer(relSrc, options)
	if err != nil {
		return nil, fmt.Errorf("error processing tar file: %w", err)
	}
	err = goInChroot(root, tb.Do)
	if err != nil {
		return nil, fmt.Errorf("could not chroot: %w", err)
	}
	return tb.Reader(), nil
}

func doUnpackLayer(root string, layer io.Reader, options *archive.TarOptions) (int64, error) {
	type result struct {
		layerSize int64
		err       error
	}
	done := make(chan result)

	err := goInChroot(root, func() {
		// We need to be able to set any perms
		_ = unix.Umask(0)

		size, err := archive.UnpackLayer("/", layer, options)
		done <- result{layerSize: size, err: err}
	})
	if err != nil {
		return 0, err
	}

	res := <-done

	return res.layerSize, res.err
}
