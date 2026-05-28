package sshsig

import (
	"encoding/pem"
	"errors"
	"fmt"
)

// PEMType is the PEM type of an armored SSH signature.
const PEMType = "SSH SIGNATURE"

// Armor returns a PEM-encoded Signature. It does not perform any validation.
func Armor(s *Signature) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  PEMType,
		Bytes: s.Marshal(),
	})
}

// Unarmor decodes a PEM-encoded signature into a Signature. It returns an
// error if the PEM block is invalid or the signature parsing fails.
func Unarmor(b []byte) (*Signature, error) {
	p, _ := pem.Decode(b)
	if p == nil {
		return nil, errors.New("invalid PEM block")
	}
	if p.Type != PEMType {
		return nil, fmt.Errorf("invalid PEM type %q: expected %q", p.Type, PEMType)
	}
	return ParseSignature(p.Bytes)
}
