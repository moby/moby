package utils

import (
	"encoding/hex"
	"hash"
	"io"
)

type CheckSum struct {
	io.Reader
	Hash hash.Hash
}

func (cs *CheckSum) Read(buf []byte) (int, error) {
	n, err := cs.Reader.Read(buf)
	if err == nil {
		cs.Hash.Write(buf[:n])
	}
	return n, err
}

func (cs *CheckSum) Sum() string {
	return hex.EncodeToString(cs.Hash.Sum(nil))
}
