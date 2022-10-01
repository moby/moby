package resolvconf

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashData returns the sha256 sum of data.
func hashData(data []byte) string {
	f := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(f[:])
}
