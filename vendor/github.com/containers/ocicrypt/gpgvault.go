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

package ocicrypt

import (
	"bytes"
	"io/ioutil"

	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
)

// GPGVault defines an interface for wrapping multiple secret key rings
type GPGVault interface {
	// AddSecretKeyRingData adds a secret keyring via its raw byte array
	AddSecretKeyRingData(gpgSecretKeyRingData []byte) error
	// AddSecretKeyRingDataArray adds secret keyring via its raw byte arrays
	AddSecretKeyRingDataArray(gpgSecretKeyRingDataArray [][]byte) error
	// AddSecretKeyRingFiles adds secret keyrings given their filenames
	AddSecretKeyRingFiles(filenames []string) error
	// GetGPGPrivateKey gets the private key bytes of a keyid given a passphrase
	GetGPGPrivateKey(keyid uint64) ([]openpgp.Key, []byte)
}

// gpgVault wraps an array of gpgSecretKeyRing
type gpgVault struct {
	entityLists []openpgp.EntityList
	keyDataList [][]byte // the raw data original passed in
}

// NewGPGVault creates an empty GPGVault
func NewGPGVault() GPGVault {
	return &gpgVault{}
}

// AddSecretKeyRingData adds a secret keyring's to the gpgVault; the raw byte
// array read from the file must be passed and will be parsed by this function
func (g *gpgVault) AddSecretKeyRingData(gpgSecretKeyRingData []byte) error {
	// read the private keys
	r := bytes.NewReader(gpgSecretKeyRingData)
	entityList, err := openpgp.ReadKeyRing(r)
	if err != nil {
		return errors.Wrapf(err, "could not read keyring")
	}
	g.entityLists = append(g.entityLists, entityList)
	g.keyDataList = append(g.keyDataList, gpgSecretKeyRingData)
	return nil
}

// AddSecretKeyRingDataArray adds secret keyrings to the gpgVault; the raw byte
// arrays read from files must be passed
func (g *gpgVault) AddSecretKeyRingDataArray(gpgSecretKeyRingDataArray [][]byte) error {
	for _, gpgSecretKeyRingData := range gpgSecretKeyRingDataArray {
		if err := g.AddSecretKeyRingData(gpgSecretKeyRingData); err != nil {
			return err
		}
	}
	return nil
}

// AddSecretKeyRingFiles adds the secret key rings given their filenames
func (g *gpgVault) AddSecretKeyRingFiles(filenames []string) error {
	for _, filename := range filenames {
		gpgSecretKeyRingData, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		err = g.AddSecretKeyRingData(gpgSecretKeyRingData)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetGPGPrivateKey gets the bytes of a specified keyid, supplying a passphrase
func (g *gpgVault) GetGPGPrivateKey(keyid uint64) ([]openpgp.Key, []byte) {
	for i, el := range g.entityLists {
		decKeys := el.KeysByIdUsage(keyid, packet.KeyFlagEncryptCommunications)
		if len(decKeys) > 0 {
			return decKeys, g.keyDataList[i]
		}
	}
	return nil, nil
}
