/*
 * ZGrab Copyright 2015 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

package json

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/big"
)

// RSAPublicKey provides JSON methods for the standard rsa.PublicKey.
type RSAPublicKey struct {
	*rsa.PublicKey
}

type auxRSAPublicKey struct {
	Exponent int    `json:"exponent"`
	Modulus  []byte `json:"modulus"`
	Length   int    `json:"length"`
}

// RSAClientParams are the TLS key exchange parameters for RSA keys.
type RSAClientParams struct {
	Length       uint16 `json:"length,omitempty"`
	EncryptedPMS []byte `json:"encrypted_pre_master_secret,omitempty"`
}

// MarshalJSON implements the json.Marshal interface
func (rp *RSAPublicKey) MarshalJSON() ([]byte, error) {
	var aux auxRSAPublicKey
	if rp.PublicKey != nil {
		aux.Exponent = rp.E
		aux.Modulus = rp.N.Bytes()
		aux.Length = len(aux.Modulus) * 8
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshal interface
func (rp *RSAPublicKey) UnmarshalJSON(b []byte) error {
	var aux auxRSAPublicKey
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if rp.PublicKey == nil {
		rp.PublicKey = new(rsa.PublicKey)
	}
	rp.E = aux.Exponent
	rp.N = big.NewInt(0).SetBytes(aux.Modulus)
	if len(aux.Modulus)*8 != aux.Length {
		return fmt.Errorf("mismatched length (got %d, field specified %d)", len(aux.Modulus), aux.Length)
	}
	return nil
}
