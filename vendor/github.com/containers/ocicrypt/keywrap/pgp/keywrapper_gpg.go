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

package pgp

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/mail"
	"strconv"
	"strings"

	"github.com/containers/ocicrypt/config"
	"github.com/containers/ocicrypt/keywrap"
	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

type gpgKeyWrapper struct {
}

// NewKeyWrapper returns a new key wrapping interface for pgp
func NewKeyWrapper() keywrap.KeyWrapper {
	return &gpgKeyWrapper{}
}

var (
	// GPGDefaultEncryptConfig is the default configuration for layer encryption/decryption
	GPGDefaultEncryptConfig = &packet.Config{
		Rand:              rand.Reader,
		DefaultHash:       crypto.SHA256,
		DefaultCipher:     packet.CipherAES256,
		CompressionConfig: &packet.CompressionConfig{Level: 0}, // No compression
		RSABits:           2048,
	}
)

func (kw *gpgKeyWrapper) GetAnnotationID() string {
	return "org.opencontainers.image.enc.keys.pgp"
}

// WrapKeys wraps the session key for recpients and encrypts the optsData, which
// describe the symmetric key used for encrypting the layer
func (kw *gpgKeyWrapper) WrapKeys(ec *config.EncryptConfig, optsData []byte) ([]byte, error) {
	ciphertext := new(bytes.Buffer)
	el, err := kw.createEntityList(ec)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create entity list")
	}
	if len(el) == 0 {
		// nothing to do -- not an error
		return nil, nil
	}

	plaintextWriter, err := openpgp.Encrypt(ciphertext,
		el,  /*EntityList*/
		nil, /* Sign*/
		nil, /* FileHint */
		GPGDefaultEncryptConfig)
	if err != nil {
		return nil, err
	}

	if _, err = plaintextWriter.Write(optsData); err != nil {
		return nil, err
	} else if err = plaintextWriter.Close(); err != nil {
		return nil, err
	}
	return ciphertext.Bytes(), err
}

// UnwrapKey unwraps the symmetric key with which the layer is encrypted
// This symmetric key is encrypted in the PGP payload.
func (kw *gpgKeyWrapper) UnwrapKey(dc *config.DecryptConfig, pgpPacket []byte) ([]byte, error) {
	pgpPrivateKeys, pgpPrivateKeysPwd, err := kw.getKeyParameters(dc.Parameters)
	if err != nil {
		return nil, err
	}

	for idx, pgpPrivateKey := range pgpPrivateKeys {
		r := bytes.NewBuffer(pgpPrivateKey)
		entityList, err := openpgp.ReadKeyRing(r)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse private keys")
		}

		var prompt openpgp.PromptFunction
		if len(pgpPrivateKeysPwd) > idx {
			responded := false
			prompt = func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
				if responded {
					return nil, fmt.Errorf("don't seem to have the right password")
				}
				responded = true
				for _, key := range keys {
					if key.PrivateKey != nil {
						_ = key.PrivateKey.Decrypt(pgpPrivateKeysPwd[idx])
					}
				}
				return pgpPrivateKeysPwd[idx], nil
			}
		}

		r = bytes.NewBuffer(pgpPacket)
		md, err := openpgp.ReadMessage(r, entityList, prompt, GPGDefaultEncryptConfig)
		if err != nil {
			continue
		}
		// we get the plain key options back
		optsData, err := ioutil.ReadAll(md.UnverifiedBody)
		if err != nil {
			continue
		}
		return optsData, nil
	}
	return nil, errors.New("PGP: No suitable key found to unwrap key")
}

// GetKeyIdsFromWrappedKeys converts the base64 encoded PGPPacket to uint64 keyIds
func (kw *gpgKeyWrapper) GetKeyIdsFromPacket(b64pgpPackets string) ([]uint64, error) {

	var keyids []uint64
	for _, b64pgpPacket := range strings.Split(b64pgpPackets, ",") {
		pgpPacket, err := base64.StdEncoding.DecodeString(b64pgpPacket)
		if err != nil {
			return nil, errors.Wrapf(err, "could not decode base64 encoded PGP packet")
		}
		newids, err := kw.getKeyIDs(pgpPacket)
		if err != nil {
			return nil, err
		}
		keyids = append(keyids, newids...)
	}
	return keyids, nil
}

// getKeyIDs parses a PGPPacket and gets the list of recipients' key IDs
func (kw *gpgKeyWrapper) getKeyIDs(pgpPacket []byte) ([]uint64, error) {
	var keyids []uint64

	kbuf := bytes.NewBuffer(pgpPacket)
	packets := packet.NewReader(kbuf)
ParsePackets:
	for {
		p, err := packets.Next()
		if err == io.EOF {
			break ParsePackets
		}
		if err != nil {
			return []uint64{}, errors.Wrapf(err, "packets.Next() failed")
		}
		switch p := p.(type) {
		case *packet.EncryptedKey:
			keyids = append(keyids, p.KeyId)
		case *packet.SymmetricallyEncrypted:
			break ParsePackets
		}
	}
	return keyids, nil
}

// GetRecipients converts the wrappedKeys to an array of recipients
func (kw *gpgKeyWrapper) GetRecipients(b64pgpPackets string) ([]string, error) {
	keyIds, err := kw.GetKeyIdsFromPacket(b64pgpPackets)
	if err != nil {
		return nil, err
	}
	var array []string
	for _, keyid := range keyIds {
		array = append(array, "0x"+strconv.FormatUint(keyid, 16))
	}
	return array, nil
}

func (kw *gpgKeyWrapper) NoPossibleKeys(dcparameters map[string][][]byte) bool {
	return len(kw.GetPrivateKeys(dcparameters)) == 0
}

func (kw *gpgKeyWrapper) GetPrivateKeys(dcparameters map[string][][]byte) [][]byte {
	return dcparameters["gpg-privatekeys"]
}

func (kw *gpgKeyWrapper) getKeyParameters(dcparameters map[string][][]byte) ([][]byte, [][]byte, error) {

	privKeys := kw.GetPrivateKeys(dcparameters)
	if len(privKeys) == 0 {
		return nil, nil, errors.New("GPG: Missing private key parameter")
	}

	return privKeys, dcparameters["gpg-privatekeys-passwords"], nil
}

// createEntityList creates the opengpg EntityList by reading the KeyRing
// first and then filtering out recipients' keys
func (kw *gpgKeyWrapper) createEntityList(ec *config.EncryptConfig) (openpgp.EntityList, error) {
	pgpPubringFile := ec.Parameters["gpg-pubkeyringfile"]
	if len(pgpPubringFile) == 0 {
		return nil, nil
	}
	r := bytes.NewReader(pgpPubringFile[0])

	entityList, err := openpgp.ReadKeyRing(r)
	if err != nil {
		return nil, err
	}

	gpgRecipients := ec.Parameters["gpg-recipients"]
	if len(gpgRecipients) == 0 {
		return nil, nil
	}

	rSet := make(map[string]int)
	for _, r := range gpgRecipients {
		rSet[string(r)] = 0
	}

	var filteredList openpgp.EntityList
	for _, entity := range entityList {
		for k := range entity.Identities {
			addr, err := mail.ParseAddress(k)
			if err != nil {
				return nil, err
			}
			for _, r := range gpgRecipients {
				recp := string(r)
				if strings.Compare(addr.Name, recp) == 0 || strings.Compare(addr.Address, recp) == 0 {
					filteredList = append(filteredList, entity)
					rSet[recp] = rSet[recp] + 1
				}
			}
		}
	}

	// make sure we found keys for all the Recipients...
	var buffer bytes.Buffer
	notFound := false
	buffer.WriteString("PGP: No key found for the following recipients: ")

	for k, v := range rSet {
		if v == 0 {
			if notFound {
				buffer.WriteString(", ")
			}
			buffer.WriteString(k)
			notFound = true
		}
	}

	if notFound {
		return nil, errors.New(buffer.String())
	}

	return filteredList, nil
}
