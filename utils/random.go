package utils

import (
	"encoding/hex"
	"github.com/dotcloud/docker/pkg/crypto"
	"io"
)

func RandomString() string {
	id := make([]byte, 32)
	_, err := io.ReadFull(crypto.RandReader, id)
	if err != nil {
		panic(err) // This shouldn't happen
	}
	return hex.EncodeToString(id)
}
