/*
Copyright 2025 The Sigstore Authors.

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

// Heavily borrowed from https://gist.githubusercontent.com/AlCutter/c6c69076dc55652e2d278900ccc1a5e7/raw/aac2bafc17a8efa162bd99b4453070b724779307/ecdsa_note.go - thanks, Al

package note

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
	"golang.org/x/mod/sumdb/note"
)

const (
	algEd25519 = 1
	algUndef   = 255
	rsaID      = "PKIX-RSA-PKCS#1v1.5"
)

// noteSigner uses an arbitrary sigstore signer to implement golang.org/x/mod/sumdb/note.Signer,
// which is used in Tessera to sign checkpoints in the signed notes format
// (https://github.com/C2SP/C2SP/blob/main/signed-note.md).
type noteSigner struct {
	name string
	hash uint32
	sign func(msg []byte) ([]byte, error)
}

// Name returns the server name associated with the key.
func (n *noteSigner) Name() string {
	return n.name
}

// KeyHash returns the key hash.
func (n *noteSigner) KeyHash() uint32 {
	return n.hash
}

// Sign returns a signature for the given message.
func (n *noteSigner) Sign(msg []byte) ([]byte, error) {
	return n.sign(msg)
}

type noteVerifier struct {
	name   string
	hash   uint32
	verify func(msg, sig []byte) bool
}

// Name implements note.Verifier.
func (n *noteVerifier) Name() string {
	return n.name
}

// Keyhash implements note.Verifier.
func (n *noteVerifier) KeyHash() uint32 {
	return n.hash
}

// Verify implements note.Verifier.
func (n *noteVerifier) Verify(msg, sig []byte) bool {
	return n.verify(msg, sig)
}

// isValidName reports whether the name conforms to the spec for the origin string of the note text
// as defined in https://github.com/C2SP/C2SP/blob/main/tlog-checkpoint.md#note-text.
func isValidName(name string) bool {
	return name != "" && utf8.ValidString(name) && strings.IndexFunc(name, unicode.IsSpace) < 0 && !strings.Contains(name, "+")
}

// genConformantKeyHash generates a truncated (4-byte) and non-truncated
// identifier for typical (non-ECDSA) keys.
func genConformantKeyHash(name string, sigType, key []byte) (uint32, []byte) {
	hash := sha256.New()
	hash.Write([]byte(name))
	hash.Write([]byte("\n"))
	hash.Write(sigType)
	hash.Write(key)
	sum := hash.Sum(nil)
	return binary.BigEndian.Uint32(sum), sum
}

// ed25519KeyHash generates the 4-byte key ID for an Ed25519 public key.
// Ed25519 keys are the only key type compatible with witnessing.
func ed25519KeyHash(name string, key []byte) (uint32, []byte) {
	return genConformantKeyHash(name, []byte{algEd25519}, key)
}

// ecdsaKeyHash generates the 4-byte key ID for an ECDSA public key.
// ECDSA key IDs do not conform to the note standard for other key type
// (see https://github.com/C2SP/C2SP/blob/8991f70ddf8a11de3a68d5a081e7be27e59d87c8/signed-note.md#signature-types).
func ecdsaKeyHash(key *ecdsa.PublicKey) (uint32, []byte, error) {
	marshaled, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return 0, nil, fmt.Errorf("marshaling public key: %w", err)
	}
	hash := sha256.Sum256(marshaled)
	return binary.BigEndian.Uint32(hash[:]), hash[:], nil
}

// rsaKeyhash generates the 4-byte key ID for an RSA public key.
func rsaKeyHash(name string, key *rsa.PublicKey) (uint32, []byte, error) {
	marshaled, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return 0, nil, fmt.Errorf("marshaling public key: %w", err)
	}
	rsaAlg := append([]byte{algUndef}, []byte(rsaID)...)
	id, hash := genConformantKeyHash(name, rsaAlg, marshaled)
	return id, hash, nil
}

// KeyHash generates a truncated (4-byte) and non-truncated identifier for a
// public key/origin
func KeyHash(origin string, key crypto.PublicKey) (uint32, []byte, error) {
	var keyID uint32
	var logID []byte
	var err error

	switch pk := key.(type) {
	case *ecdsa.PublicKey:
		keyID, logID, err = ecdsaKeyHash(pk)
		if err != nil {
			return 0, nil, fmt.Errorf("getting ECDSA key hash: %w", err)
		}
	case ed25519.PublicKey:
		keyID, logID = ed25519KeyHash(origin, pk)
	case *rsa.PublicKey:
		keyID, logID, err = rsaKeyHash(origin, pk)
		if err != nil {
			return 0, nil, fmt.Errorf("getting RSA key hash: %w", err)
		}
	default:
		return 0, nil, fmt.Errorf("unsupported key type: %T", key)
	}

	return keyID, logID, nil
}

// NewNoteSigner converts a sigstore/sigstore/pkg/signature.Signer into a note.Signer.
func NewNoteSigner(ctx context.Context, origin string, signer signature.Signer) (note.Signer, error) {
	if !isValidName(origin) {
		return nil, fmt.Errorf("invalid name %s", origin)
	}

	pubKey, err := signer.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("getting public key: %w", err)
	}

	keyID, _, err := KeyHash(origin, pubKey)
	if err != nil {
		return nil, err
	}

	sign := func(msg []byte) ([]byte, error) {
		return signer.SignMessage(bytes.NewReader(msg), options.WithContext(ctx))
	}

	return &noteSigner{
		name: origin,
		hash: keyID,
		sign: sign,
	}, nil
}

// NewNoteVerifier converts a sigstore/sigstore/pkg/signature.Verifier into a note.Verifier.
func NewNoteVerifier(origin string, verifier signature.Verifier) (note.Verifier, error) {
	if !isValidName(origin) {
		return nil, fmt.Errorf("invalid name %s", origin)
	}

	pubKey, err := verifier.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("getting public key: %w", err)
	}

	keyID, _, err := KeyHash(origin, pubKey)
	if err != nil {
		return nil, err
	}

	return &noteVerifier{
		name: origin,
		hash: keyID,
		verify: func(msg, sig []byte) bool {
			if err := verifier.VerifySignature(bytes.NewReader(sig), bytes.NewReader(msg)); err != nil {
				return false
			}
			return true
		},
	}, nil
}
