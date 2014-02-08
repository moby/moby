// +build !openssl

package crypto

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
)

var (
	RandReader = rand.Reader
	RandRead   = rand.Read
	NewSHA1    = sha1.New
	NewSHA256  = sha256.New
)
