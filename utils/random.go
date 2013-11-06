package utils

import (
	"io"
	"crypto/rand"
	"encoding/hex"
)

func RandomString() string {
	id := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, id)
	if err != nil {
		panic(err) // This shouldn't happen
	}
	return hex.EncodeToString(id)
}
