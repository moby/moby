/*
   Copyright The ocicrypt Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package pkcs11

import (
	"github.com/containers/ocicrypt/config"
	"github.com/containers/ocicrypt/crypto/pkcs11"
	"github.com/containers/ocicrypt/keywrap"
	"github.com/containers/ocicrypt/utils"

	"github.com/pkg/errors"
)

type pkcs11KeyWrapper struct {
}

func (kw *pkcs11KeyWrapper) GetAnnotationID() string {
	return "org.opencontainers.image.enc.keys.pkcs11"
}

// NewKeyWrapper returns a new key wrapping interface using pkcs11
func NewKeyWrapper() keywrap.KeyWrapper {
	return &pkcs11KeyWrapper{}
}

// WrapKeys wraps the session key for recpients and encrypts the optsData, which
// describe the symmetric key used for encrypting the layer
func (kw *pkcs11KeyWrapper) WrapKeys(ec *config.EncryptConfig, optsData []byte) ([]byte, error) {
	pkcs11Recipients, err := addPubKeys(&ec.DecryptConfig, append(ec.Parameters["pkcs11-pubkeys"], ec.Parameters["pkcs11-yamls"]...))
	if err != nil {
		return nil, err
	}
	// no recipients is not an error...
	if len(pkcs11Recipients) == 0 {
		return nil, nil
	}

	jsonString, err := pkcs11.EncryptMultiple(pkcs11Recipients, optsData)
	if err != nil {
		return nil, errors.Wrapf(err, "PKCS11 EncryptMulitple failed")
	}
	return jsonString, nil
}

func (kw *pkcs11KeyWrapper) UnwrapKey(dc *config.DecryptConfig, jsonString []byte) ([]byte, error) {
	var pkcs11PrivKeys []*pkcs11.Pkcs11KeyFileObject

	privKeys := kw.GetPrivateKeys(dc.Parameters)
	if len(privKeys) == 0 {
		return nil, errors.New("No private keys found for PKCS11 decryption")
	}

	p11conf, err := p11confFromParameters(dc.Parameters)
	if err != nil {
		return nil, err
	}

	for _, privKey := range privKeys {
		key, err := utils.ParsePrivateKey(privKey, nil, "PKCS11")
		if err != nil {
			return nil, err
		}
		switch pkcs11PrivKey := key.(type) {
		case *pkcs11.Pkcs11KeyFileObject:
			if p11conf != nil {
				pkcs11PrivKey.Uri.SetModuleDirectories(p11conf.ModuleDirectories)
				pkcs11PrivKey.Uri.SetAllowedModulePaths(p11conf.AllowedModulePaths)
			}
			pkcs11PrivKeys = append(pkcs11PrivKeys, pkcs11PrivKey)
		default:
			continue
		}
	}

	plaintext, err := pkcs11.Decrypt(pkcs11PrivKeys, jsonString)
	if err == nil {
		return plaintext, nil
	}

	return nil, errors.Wrapf(err, "PKCS11: No suitable private key found for decryption")
}

func (kw *pkcs11KeyWrapper) NoPossibleKeys(dcparameters map[string][][]byte) bool {
	return len(kw.GetPrivateKeys(dcparameters)) == 0
}

func (kw *pkcs11KeyWrapper) GetPrivateKeys(dcparameters map[string][][]byte) [][]byte {
	return dcparameters["pkcs11-yamls"]
}

func (kw *pkcs11KeyWrapper) GetKeyIdsFromPacket(_ string) ([]uint64, error) {
	return nil, nil
}

func (kw *pkcs11KeyWrapper) GetRecipients(_ string) ([]string, error) {
	return []string{"[pkcs11]"}, nil
}

func addPubKeys(dc *config.DecryptConfig, pubKeys [][]byte) ([]interface{}, error) {
	var pkcs11Keys []interface{}

	if len(pubKeys) == 0 {
		return pkcs11Keys, nil
	}

	p11conf, err := p11confFromParameters(dc.Parameters)
	if err != nil {
		return nil, err
	}

	for _, pubKey := range pubKeys {
		key, err := utils.ParsePublicKey(pubKey, "PKCS11")
		if err != nil {
			return nil, err
		}
		switch pkcs11PubKey := key.(type) {
		case *pkcs11.Pkcs11KeyFileObject:
			if p11conf != nil {
				pkcs11PubKey.Uri.SetModuleDirectories(p11conf.ModuleDirectories)
				pkcs11PubKey.Uri.SetAllowedModulePaths(p11conf.AllowedModulePaths)
			}
		}
		pkcs11Keys = append(pkcs11Keys, key)
	}
	return pkcs11Keys, nil
}

func p11confFromParameters(dcparameters map[string][][]byte) (*pkcs11.Pkcs11Config, error){
	if _, ok := dcparameters["pkcs11-config"]; ok {
		return pkcs11.ParsePkcs11ConfigFile(dcparameters["pkcs11-config"][0])
	}
	return nil, nil
}
