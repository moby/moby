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
	"crypto/elliptic"
	"encoding/json"
	"math/big"
)

// TLSCurveID is the type of a TLS identifier for an elliptic curve. See
// http://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-8
type TLSCurveID uint16

// ECDHPrivateParams are the TLS key exchange parameters for ECDH keys.
type ECDHPrivateParams struct {
	Value  []byte `json:"value,omitempty"`
	Length int    `json:"length,omitempty"`
}

// ECDHParams stores elliptic-curve Diffie-Hellman paramters.At any point in
// time, it is unlikely that both ServerPrivate and ClientPrivate will be non-nil.
type ECDHParams struct {
	TLSCurveID    TLSCurveID         `json:"curve_id,omitempty"`
	Curve         elliptic.Curve     `json:"-"`
	ServerPublic  *ECPoint           `json:"server_public,omitempty"`
	ServerPrivate *ECDHPrivateParams `json:"server_private,omitempty"`
	ClientPublic  *ECPoint           `json:"client_public,omitempty"`
	ClientPrivate *ECDHPrivateParams `json:"client_private,omitempty"`
}

// ECPoint represents an elliptic curve point and serializes nicely to JSON
type ECPoint struct {
	X *big.Int
	Y *big.Int
}

// MarshalJSON implements the json.Marshler interface
func (p *ECPoint) MarshalJSON() ([]byte, error) {
	aux := struct {
		X *cryptoParameter `json:"x"`
		Y *cryptoParameter `json:"y"`
	}{
		X: &cryptoParameter{Int: p.X},
		Y: &cryptoParameter{Int: p.Y},
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshler interface
func (p *ECPoint) UnmarshalJSON(b []byte) error {
	aux := struct {
		X *cryptoParameter `json:"x"`
		Y *cryptoParameter `json:"y"`
	}{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	p.X = aux.X.Int
	p.Y = aux.Y.Int
	return nil
}

// Description returns the description field for the given ID. See
// http://www.iana.org/assignments/tls-parameters/tls-parameters.xml#tls-parameters-8
func (c *TLSCurveID) Description() string {
	if desc, ok := ecIDToName[*c]; ok {
		return desc
	}
	return "unknown"
}

// MarshalJSON implements the json.Marshaler interface
func (c *TLSCurveID) MarshalJSON() ([]byte, error) {
	aux := struct {
		Name string `json:"name"`
		ID   uint16 `json:"id"`
	}{
		Name: c.Description(),
		ID:   uint16(*c),
	}
	return json.Marshal(&aux)
}

//UnmarshalJSON implements the json.Unmarshaler interface
func (c *TLSCurveID) UnmarshalJSON(b []byte) error {
	aux := struct {
		ID uint16 `json:"id"`
	}{}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*c = TLSCurveID(aux.ID)
	return nil
}
