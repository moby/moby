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

package blockcipher

import (
	"io"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// LayerCipherType is the ciphertype as specified in the layer metadata
type LayerCipherType string

// TODO: Should be obtained from OCI spec once included
const (
	AES256CTR LayerCipherType = "AES_256_CTR_HMAC_SHA256"
)

// PrivateLayerBlockCipherOptions includes the information required to encrypt/decrypt
// an image which are sensitive and should not be in plaintext
type PrivateLayerBlockCipherOptions struct {
	// SymmetricKey represents the symmetric key used for encryption/decryption
	// This field should be populated by Encrypt/Decrypt calls
	SymmetricKey []byte `json:"symkey"`

	// Digest is the digest of the original data for verification.
	// This is NOT populated by Encrypt/Decrypt calls
	Digest digest.Digest `json:"digest"`

	// CipherOptions contains the cipher metadata used for encryption/decryption
	// This field should be populated by Encrypt/Decrypt calls
	CipherOptions map[string][]byte `json:"cipheroptions"`
}

// PublicLayerBlockCipherOptions includes the information required to encrypt/decrypt
// an image which are public and can be deduplicated in plaintext across multiple
// recipients
type PublicLayerBlockCipherOptions struct {
	// CipherType denotes the cipher type according to the list of OCI suppported
	// cipher types.
	CipherType LayerCipherType `json:"cipher"`

	// Hmac contains the hmac string to help verify encryption
	Hmac []byte `json:"hmac"`

	// CipherOptions contains the cipher metadata used for encryption/decryption
	// This field should be populated by Encrypt/Decrypt calls
	CipherOptions map[string][]byte `json:"cipheroptions"`
}

// LayerBlockCipherOptions contains the public and private LayerBlockCipherOptions
// required to encrypt/decrypt an image
type LayerBlockCipherOptions struct {
	Public  PublicLayerBlockCipherOptions
	Private PrivateLayerBlockCipherOptions
}

// LayerBlockCipher returns a provider for encrypt/decrypt functionality
// for handling the layer data for a specific algorithm
type LayerBlockCipher interface {
	// GenerateKey creates a symmetric key
	GenerateKey() ([]byte, error)
	// Encrypt takes in layer data and returns the ciphertext and relevant LayerBlockCipherOptions
	Encrypt(layerDataReader io.Reader, opt LayerBlockCipherOptions) (io.Reader, Finalizer, error)
	// Decrypt takes in layer ciphertext data and returns the plaintext and relevant LayerBlockCipherOptions
	Decrypt(layerDataReader io.Reader, opt LayerBlockCipherOptions) (io.Reader, LayerBlockCipherOptions, error)
}

// LayerBlockCipherHandler is the handler for encrypt/decrypt for layers
type LayerBlockCipherHandler struct {
	cipherMap map[LayerCipherType]LayerBlockCipher
}

// Finalizer is called after data blobs are written, and returns the LayerBlockCipherOptions for the encrypted blob
type Finalizer func() (LayerBlockCipherOptions, error)

// GetOpt returns the value of the cipher option and if the option exists
func (lbco LayerBlockCipherOptions) GetOpt(key string) (value []byte, ok bool) {
	if v, ok := lbco.Public.CipherOptions[key]; ok {
		return v, ok
	} else if v, ok := lbco.Private.CipherOptions[key]; ok {
		return v, ok
	} else {
		return nil, false
	}
}

func wrapFinalizerWithType(fin Finalizer, typ LayerCipherType) Finalizer {
	return func() (LayerBlockCipherOptions, error) {
		lbco, err := fin()
		if err != nil {
			return LayerBlockCipherOptions{}, err
		}
		lbco.Public.CipherType = typ
		return lbco, err
	}
}

// Encrypt is the handler for the layer decryption routine
func (h *LayerBlockCipherHandler) Encrypt(plainDataReader io.Reader, typ LayerCipherType) (io.Reader, Finalizer, error) {
	if c, ok := h.cipherMap[typ]; ok {
		sk, err := c.GenerateKey()
		if err != nil {
			return nil, nil, err
		}
		opt := LayerBlockCipherOptions{
			Private: PrivateLayerBlockCipherOptions{
				SymmetricKey: sk,
			},
		}
		encDataReader, fin, err := c.Encrypt(plainDataReader, opt)
		if err == nil {
			fin = wrapFinalizerWithType(fin, typ)
		}
		return encDataReader, fin, err
	}
	return nil, nil, errors.Errorf("unsupported cipher type: %s", typ)
}

// Decrypt is the handler for the layer decryption routine
func (h *LayerBlockCipherHandler) Decrypt(encDataReader io.Reader, opt LayerBlockCipherOptions) (io.Reader, LayerBlockCipherOptions, error) {
	typ := opt.Public.CipherType
	if typ == "" {
		return nil, LayerBlockCipherOptions{}, errors.New("no cipher type provided")
	}
	if c, ok := h.cipherMap[LayerCipherType(typ)]; ok {
		return c.Decrypt(encDataReader, opt)
	}
	return nil, LayerBlockCipherOptions{}, errors.Errorf("unsupported cipher type: %s", typ)
}

// NewLayerBlockCipherHandler returns a new default handler
func NewLayerBlockCipherHandler() (*LayerBlockCipherHandler, error) {
	h := LayerBlockCipherHandler{
		cipherMap: map[LayerCipherType]LayerBlockCipher{},
	}

	var err error
	h.cipherMap[AES256CTR], err = NewAESCTRLayerBlockCipher(256)
	if err != nil {
		return nil, errors.Wrap(err, "unable to set up Cipher AES-256-CTR")
	}

	return &h, nil
}
