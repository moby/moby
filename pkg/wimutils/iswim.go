package wimutils

import (
	"bytes"
	"encoding/binary"
	"io"
)

var wimMagic = [8]byte{'M', 'S', 'W', 'I', 'M'}

// IsWIM looks at the first 12 bytes of a file to determine if the underlying stream
// is a WIM file.
func IsWIM(r io.ReaderAt) (bool, error) {
	var b [12]byte
	n, err := r.ReadAt(b[:], 0)
	if err != nil {
		return false, err
	}

	if n != len(b) || !bytes.Equal(b[:8], wimMagic[:]) {
		return false, nil
	}
	// This magic number could be confused with a tar whose first file is MSWIM.
	// Make sure there is non-zero data just after the NUL values, which would be prohibited
	// in the tar format.
	if binary.LittleEndian.Uint32(b[8:12]) == 0 {
		return false, nil
	}
	return true, nil
}
