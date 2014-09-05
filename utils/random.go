package utils

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

func RandomString() string {
	id := make([]byte, 32)

	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		panic(err) // This shouldn't happen
	}
	return hex.EncodeToString(id)
}
