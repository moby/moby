package resolvconf

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// hashData returns the sha256 sum of src.
func hashData(src io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
