package resolvconf

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashData returns the sha256 sum of data.
func hashData(data []byte) []byte {
	f := sha256.Sum256(data)
	out := make([]byte, 2*sha256.Size)
	hex.Encode(out, f[:])
	return append([]byte("sha256:"), out...)
}
