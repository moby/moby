package tarheader

import (
	"archive/tar"
	"os"
)

// sysStat populates hdr from system-dependent fields of fi without performing
// any OS lookups. It is a no-op on Windows.
func sysStat(os.FileInfo, *tar.Header) error {
	return nil
}
