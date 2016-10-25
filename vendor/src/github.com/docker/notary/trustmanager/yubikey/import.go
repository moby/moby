// +build pkcs11

package yubikey

import (
	"encoding/pem"
	"errors"
	"github.com/docker/notary"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/tuf/utils"
)

// YubiImport is a wrapper around the YubiStore that allows us to import private
// keys to the yubikey
type YubiImport struct {
	dest          *YubiStore
	passRetriever notary.PassRetriever
}

// NewImporter returns a wrapper for the YubiStore provided that enables importing
// keys via the simple Set(string, []byte) interface
func NewImporter(ys *YubiStore, ret notary.PassRetriever) *YubiImport {
	return &YubiImport{
		dest:          ys,
		passRetriever: ret,
	}
}

// Set determines if we are allowed to set the given key on the Yubikey and
// calls through to YubiStore.AddKey if it's valid
func (s *YubiImport) Set(name string, bytes []byte) error {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return errors.New("invalid PEM data, could not parse")
	}
	role, ok := block.Headers["role"]
	if !ok {
		return errors.New("no role found for key")
	}
	ki := trustmanager.KeyInfo{
		// GUN is ignored by YubiStore
		Role: role,
	}
	privKey, err := utils.ParsePEMPrivateKey(bytes, "")
	if err != nil {
		privKey, _, err = trustmanager.GetPasswdDecryptBytes(
			s.passRetriever,
			bytes,
			name,
			ki.Role,
		)
		if err != nil {
			return err
		}
	}
	return s.dest.AddKey(ki, privKey)
}
