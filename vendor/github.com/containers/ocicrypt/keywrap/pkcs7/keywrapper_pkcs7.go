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

package pkcs7

import (
	"crypto"
	"crypto/x509"

	"github.com/containers/ocicrypt/config"
	"github.com/containers/ocicrypt/keywrap"
	"github.com/containers/ocicrypt/utils"
	"github.com/pkg/errors"
	"go.mozilla.org/pkcs7"
)

type pkcs7KeyWrapper struct {
}

// NewKeyWrapper returns a new key wrapping interface using jwe
func NewKeyWrapper() keywrap.KeyWrapper {
	return &pkcs7KeyWrapper{}
}

func (kw *pkcs7KeyWrapper) GetAnnotationID() string {
	return "org.opencontainers.image.enc.keys.pkcs7"
}

// WrapKeys wraps the session key for recpients and encrypts the optsData, which
// describe the symmetric key used for encrypting the layer
func (kw *pkcs7KeyWrapper) WrapKeys(ec *config.EncryptConfig, optsData []byte) ([]byte, error) {
	x509Certs, err := collectX509s(ec.Parameters["x509s"])
	if err != nil {
		return nil, err
	}
	// no recipients is not an error...
	if len(x509Certs) == 0 {
		return nil, nil
	}

	pkcs7.ContentEncryptionAlgorithm = pkcs7.EncryptionAlgorithmAES128GCM
	return pkcs7.Encrypt(optsData, x509Certs)
}

func collectX509s(x509s [][]byte) ([]*x509.Certificate, error) {
	if len(x509s) == 0 {
		return nil, nil
	}
	var x509Certs []*x509.Certificate
	for _, x509 := range x509s {
		x509Cert, err := utils.ParseCertificate(x509, "PKCS7")
		if err != nil {
			return nil, err
		}
		x509Certs = append(x509Certs, x509Cert)
	}
	return x509Certs, nil
}

func (kw *pkcs7KeyWrapper) NoPossibleKeys(dcparameters map[string][][]byte) bool {
	return len(kw.GetPrivateKeys(dcparameters)) == 0
}

func (kw *pkcs7KeyWrapper) GetPrivateKeys(dcparameters map[string][][]byte) [][]byte {
	return dcparameters["privkeys"]
}

func (kw *pkcs7KeyWrapper) getPrivateKeysPasswords(dcparameters map[string][][]byte) [][]byte {
	return dcparameters["privkeys-passwords"]
}

// UnwrapKey unwraps the symmetric key with which the layer is encrypted
// This symmetric key is encrypted in the PKCS7 payload.
func (kw *pkcs7KeyWrapper) UnwrapKey(dc *config.DecryptConfig, pkcs7Packet []byte) ([]byte, error) {
	privKeys := kw.GetPrivateKeys(dc.Parameters)
	if len(privKeys) == 0 {
		return nil, errors.New("no private keys found for PKCS7 decryption")
	}
	privKeysPasswords := kw.getPrivateKeysPasswords(dc.Parameters)
	if len(privKeysPasswords) != len(privKeys) {
		return nil, errors.New("private key password array length must be same as that of private keys")
	}

	x509Certs, err := collectX509s(dc.Parameters["x509s"])
	if err != nil {
		return nil, err
	}
	if len(x509Certs) == 0 {
		return nil, errors.New("no x509 certificates found needed for PKCS7 decryption")
	}

	p7, err := pkcs7.Parse(pkcs7Packet)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse PKCS7 packet")
	}

	for idx, privKey := range privKeys {
		key, err := utils.ParsePrivateKey(privKey, privKeysPasswords[idx], "PKCS7")
		if err != nil {
			return nil, err
		}
		for _, x509Cert := range x509Certs {
			optsData, err := p7.Decrypt(x509Cert, crypto.PrivateKey(key))
			if err != nil {
				continue
			}
			return optsData, nil
		}
	}
	return nil, errors.New("PKCS7: No suitable private key found for decryption")
}

// GetKeyIdsFromWrappedKeys converts the base64 encoded Packet to uint64 keyIds;
// We cannot do this with pkcs7
func (kw *pkcs7KeyWrapper) GetKeyIdsFromPacket(b64pkcs7Packets string) ([]uint64, error) {
	return nil, nil
}

// GetRecipients converts the wrappedKeys to an array of recipients
// We cannot do this with pkcs7
func (kw *pkcs7KeyWrapper) GetRecipients(b64pkcs7Packets string) ([]string, error) {
	return []string{"[pkcs7]"}, nil
}
