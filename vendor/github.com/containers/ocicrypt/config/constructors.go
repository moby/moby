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

import (
	"github.com/containers/ocicrypt/crypto/pkcs11"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// EncryptWithJwe returns a CryptoConfig to encrypt with jwe public keys
func EncryptWithJwe(pubKeys [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{}
	ep := map[string][][]byte{
		"pubkeys": pubKeys,
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// EncryptWithPkcs7 returns a CryptoConfig to encrypt with pkcs7 x509 certs
func EncryptWithPkcs7(x509s [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{}

	ep := map[string][][]byte{
		"x509s": x509s,
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// EncryptWithGpg returns a CryptoConfig to encrypt with configured gpg parameters
func EncryptWithGpg(gpgRecipients [][]byte, gpgPubRingFile []byte) (CryptoConfig, error) {
	dc := DecryptConfig{}
	ep := map[string][][]byte{
		"gpg-recipients":     gpgRecipients,
		"gpg-pubkeyringfile": {gpgPubRingFile},
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// EncryptWithPkcs11 returns a CryptoConfig to encrypt with configured pkcs11 parameters
func EncryptWithPkcs11(pkcs11Config *pkcs11.Pkcs11Config, pkcs11Pubkeys, pkcs11Yamls [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{}
	ep := map[string][][]byte{}

	if len(pkcs11Yamls) > 0 {
		if pkcs11Config == nil {
			return CryptoConfig{}, errors.New("pkcs11Config must not be nil")
		}
		p11confYaml, err := yaml.Marshal(pkcs11Config)
		if err != nil {
			return CryptoConfig{}, errors.Wrapf(err, "Could not marshal Pkcs11Config to Yaml")
		}

		dc = DecryptConfig{
			Parameters: map[string][][]byte{
				"pkcs11-config": {p11confYaml},
			},
		}
		ep["pkcs11-yamls"] = pkcs11Yamls
	}
	if len(pkcs11Pubkeys) > 0 {
		ep["pkcs11-pubkeys"] = pkcs11Pubkeys
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// EncryptWithKeyProvider returns a CryptoConfig to encrypt with configured keyprovider parameters
func EncryptWithKeyProvider(keyProviders [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{}
	ep := make(map[string][][]byte)
	for _, keyProvider := range keyProviders {
		keyProvidersStr := string(keyProvider)
		idx := strings.Index(keyProvidersStr, ":")
		if idx > 0 {
			ep[keyProvidersStr[:idx]] = append(ep[keyProvidersStr[:idx]], []byte(keyProvidersStr[idx+1:]))
		} else {
			ep[keyProvidersStr] = append(ep[keyProvidersStr], []byte("Enabled"))
		}
	}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// DecryptWithKeyProvider returns a CryptoConfig to decrypt with configured keyprovider parameters
func DecryptWithKeyProvider(keyProviders [][]byte) (CryptoConfig, error) {
	dp := make(map[string][][]byte)
	ep := map[string][][]byte{}
	for _, keyProvider := range keyProviders {
		keyProvidersStr := string(keyProvider)
		idx := strings.Index(keyProvidersStr, ":")
		if idx > 0 {
			dp[keyProvidersStr[:idx]] = append(dp[keyProvidersStr[:idx]], []byte(keyProvidersStr[idx+1:]))
		} else {
			dp[keyProvidersStr] = append(dp[keyProvidersStr], []byte("Enabled"))
		}
	}
	dc := DecryptConfig{
		Parameters: dp,
	}
	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// DecryptWithPrivKeys returns a CryptoConfig to decrypt with configured private keys
func DecryptWithPrivKeys(privKeys [][]byte, privKeysPasswords [][]byte) (CryptoConfig, error) {
	if len(privKeys) != len(privKeysPasswords) {
		return CryptoConfig{}, errors.New("Length of privKeys should match length of privKeysPasswords")
	}

	dc := DecryptConfig{
		Parameters: map[string][][]byte{
			"privkeys":           privKeys,
			"privkeys-passwords": privKeysPasswords,
		},
	}

	ep := map[string][][]byte{}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// DecryptWithX509s returns a CryptoConfig to decrypt with configured x509 certs
func DecryptWithX509s(x509s [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{
		Parameters: map[string][][]byte{
			"x509s": x509s,
		},
	}

	ep := map[string][][]byte{}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// DecryptWithGpgPrivKeys returns a CryptoConfig to decrypt with configured gpg private keys
func DecryptWithGpgPrivKeys(gpgPrivKeys, gpgPrivKeysPwds [][]byte) (CryptoConfig, error) {
	dc := DecryptConfig{
		Parameters: map[string][][]byte{
			"gpg-privatekeys":           gpgPrivKeys,
			"gpg-privatekeys-passwords": gpgPrivKeysPwds,
		},
	}

	ep := map[string][][]byte{}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}

// DecryptWithPkcs11Yaml returns a CryptoConfig to decrypt with pkcs11 YAML formatted key files
func DecryptWithPkcs11Yaml(pkcs11Config *pkcs11.Pkcs11Config, pkcs11Yamls [][]byte) (CryptoConfig, error) {
	p11confYaml, err := yaml.Marshal(pkcs11Config)
	if err != nil {
		return CryptoConfig{}, errors.Wrapf(err, "Could not marshal Pkcs11Config to Yaml")
	}

	dc := DecryptConfig{
		Parameters: map[string][][]byte{
			"pkcs11-yamls":  pkcs11Yamls,
			"pkcs11-config": {p11confYaml},
		},
	}

	ep := map[string][][]byte{}

	return CryptoConfig{
		EncryptConfig: &EncryptConfig{
			Parameters:    ep,
			DecryptConfig: dc,
		},
		DecryptConfig: &dc,
	}, nil
}
