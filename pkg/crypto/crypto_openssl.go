// +build openssl

package crypto

import (
	"github.com/vbatts/gossl/rand"
	"github.com/vbatts/gossl/sha1"
	"github.com/vbatts/gossl/sha256"
)

var (
	RandReader = rand.Reader
	RandRead   = rand.Read
	NewSHA1    = sha1.New
	NewSHA256  = sha256.New
)
