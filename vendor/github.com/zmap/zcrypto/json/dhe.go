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
	"encoding/json"
	"math/big"
)

// DHParams can be used to store finite-field Diffie-Hellman parameters. At any
// point in time, it is unlikely that both OurPrivate and TheirPrivate will be
// non-nil.
type DHParams struct {
	Prime         *big.Int
	Generator     *big.Int
	ServerPublic  *big.Int
	ServerPrivate *big.Int
	ClientPublic  *big.Int
	ClientPrivate *big.Int
	SessionKey    *big.Int
}

type auxDHParams struct {
	Prime         *cryptoParameter `json:"prime"`
	Generator     *cryptoParameter `json:"generator"`
	ServerPublic  *cryptoParameter `json:"server_public,omitempty"`
	ServerPrivate *cryptoParameter `json:"server_private,omitempty"`
	ClientPublic  *cryptoParameter `json:"client_public,omitempty"`
	ClientPrivate *cryptoParameter `json:"client_private,omitempty"`
	SessionKey    *cryptoParameter `json:"session_key,omitempty"`
}

// MarshalJSON implements the json.Marshal interface
func (p *DHParams) MarshalJSON() ([]byte, error) {
	aux := auxDHParams{
		Prime:     &cryptoParameter{Int: p.Prime},
		Generator: &cryptoParameter{Int: p.Generator},
	}
	if p.ServerPublic != nil {
		aux.ServerPublic = &cryptoParameter{Int: p.ServerPublic}
	}
	if p.ServerPrivate != nil {
		aux.ServerPrivate = &cryptoParameter{Int: p.ServerPrivate}
	}
	if p.ClientPublic != nil {
		aux.ClientPublic = &cryptoParameter{Int: p.ClientPublic}
	}
	if p.ClientPrivate != nil {
		aux.ClientPrivate = &cryptoParameter{Int: p.ClientPrivate}
	}
	if p.SessionKey != nil {
		aux.SessionKey = &cryptoParameter{Int: p.SessionKey}
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implement the json.Unmarshaler interface
func (p *DHParams) UnmarshalJSON(b []byte) error {
	var aux auxDHParams
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if aux.Prime != nil {
		p.Prime = aux.Prime.Int
	}
	if aux.Generator != nil {
		p.Generator = aux.Generator.Int
	}
	if aux.ServerPublic != nil {
		p.ServerPublic = aux.ServerPublic.Int
	}
	if aux.ServerPrivate != nil {
		p.ServerPrivate = aux.ServerPrivate.Int
	}
	if aux.ClientPublic != nil {
		p.ClientPublic = aux.ClientPublic.Int
	}
	if aux.ClientPrivate != nil {
		p.ClientPrivate = aux.ClientPrivate.Int
	}
	if aux.SessionKey != nil {
		p.SessionKey = aux.SessionKey.Int
	}
	return nil
}

// CryptoParameter represents a big.Int used a parameter in some cryptography.
// It serializes to json as a tupe of a base64-encoded number and a length in
// bits.
type cryptoParameter struct {
	*big.Int
}

type auxCryptoParameter struct {
	Raw    []byte `json:"value"`
	Length int    `json:"length"`
}

// MarshalJSON implements the json.Marshaler interface
func (p *cryptoParameter) MarshalJSON() ([]byte, error) {
	var aux auxCryptoParameter
	if p.Int != nil {
		aux.Raw = p.Bytes()
		aux.Length = 8 * len(aux.Raw)
	}
	return json.Marshal(&aux)
}

// UnmarshalJSON implements the json.Unmarshal interface
func (p *cryptoParameter) UnmarshalJSON(b []byte) error {
	var aux auxCryptoParameter
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	p.Int = new(big.Int)
	p.SetBytes(aux.Raw)
	return nil
}
