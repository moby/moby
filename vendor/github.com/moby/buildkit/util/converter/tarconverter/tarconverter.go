package tarconverter

import (
	"archive/tar"
	"io"
)

type HeaderConverter func(*tar.Header)

// NewReader returns a reader that applies headerConverter.
// Forked from https://github.com/moby/moby/blob/v24.0.6/pkg/archive/copy.go#L308-L373 .
func NewReader(srcContent io.Reader, headerConverter HeaderConverter) io.ReadCloser {
	rebased, w := io.Pipe()

	go func() {
		srcTar := tar.NewReader(srcContent)
		rebasedTar := tar.NewWriter(w)

		for {
			hdr, err := srcTar.Next()
			if err == io.EOF {
				// Signals end of archive.
				rebasedTar.Close()
				w.Close()
				return
			}
			if err != nil {
				w.CloseWithError(err)
				return
			}
			if headerConverter != nil {
				headerConverter(hdr)
			}
			if err = rebasedTar.WriteHeader(hdr); err != nil {
				w.CloseWithError(err)
				return
			}

			// Ignoring GoSec G110. See https://github.com/moby/moby/blob/v24.0.6/pkg/archive/copy.go#L355-L363
			//nolint:gosec // G110: Potential DoS vulnerability via decompression bomb (gosec)
			if _, err = io.Copy(rebasedTar, srcTar); err != nil {
				w.CloseWithError(err)
				return
			}
		}
	}()

	return rebased
}
