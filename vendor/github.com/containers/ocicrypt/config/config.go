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

package config

// EncryptConfig is the container image PGP encryption configuration holding
// the identifiers of those that will be able to decrypt the container and
// the PGP public keyring file data that contains their public keys.
type EncryptConfig struct {
	// map holding 'gpg-recipients', 'gpg-pubkeyringfile', 'pubkeys', 'x509s'
	Parameters map[string][][]byte

	DecryptConfig DecryptConfig
}

// DecryptConfig wraps the Parameters map that holds the decryption key
type DecryptConfig struct {
	// map holding 'privkeys', 'x509s', 'gpg-privatekeys'
	Parameters map[string][][]byte
}

// CryptoConfig is a common wrapper for EncryptConfig and DecrypConfig that can
// be passed through functions that share much code for encryption and decryption
type CryptoConfig struct {
	EncryptConfig *EncryptConfig
	DecryptConfig *DecryptConfig
}

// InitDecryption initialized a CryptoConfig object with parameters used for decryption
func InitDecryption(dcparameters map[string][][]byte) CryptoConfig {
	return CryptoConfig{
		DecryptConfig: &DecryptConfig{
			Parameters: dcparameters,
		},
	}
}

// InitEncryption initializes a CryptoConfig object with parameters used for encryption
// It also takes dcparameters that may be needed for decryption when adding a recipient
// to an already encrypted image
func InitEncryption(parameters, dcparameters map[string][][]byte) CryptoConfig {
	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters: parameters,
			DecryptConfig: DecryptConfig{
				Parameters: dcparameters,
			},
		},
	}
}

// CombineCryptoConfigs takes a CryptoConfig list and creates a single CryptoConfig
// containing the crypto configuration of all the key bundles
func CombineCryptoConfigs(ccs []CryptoConfig) CryptoConfig {
	ecparam := map[string][][]byte{}
	ecdcparam := map[string][][]byte{}
	dcparam := map[string][][]byte{}

	for _, cc := range ccs {
		if ec := cc.EncryptConfig; ec != nil {
			addToMap(ecparam, ec.Parameters)
			addToMap(ecdcparam, ec.DecryptConfig.Parameters)
		}

		if dc := cc.DecryptConfig; dc != nil {
			addToMap(dcparam, dc.Parameters)
		}
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters: ecparam,
			DecryptConfig: DecryptConfig{
				Parameters: ecdcparam,
			},
		},
		DecryptConfig: &DecryptConfig{
			Parameters: dcparam,
		},
	}

}

// AttachDecryptConfig adds DecryptConfig to the field of EncryptConfig so that
// the decryption parameters can be used to add recipients to an existing image
// if the user is able to decrypt it.
func (ec *EncryptConfig) AttachDecryptConfig(dc *DecryptConfig) {
	if dc != nil {
		addToMap(ec.DecryptConfig.Parameters, dc.Parameters)
	}
}

func addToMap(orig map[string][][]byte, add map[string][][]byte) {
	for k, v := range add {
		if ov, ok := orig[k]; ok {
			orig[k] = append(ov, v...)
		} else {
			orig[k] = v
		}
	}
}
