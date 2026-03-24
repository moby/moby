//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
	"golang.org/x/mod/sumdb/note"
)

type SignedNote struct {
	// Textual representation of a note to sign.
	Note string
	// Signatures are one or more signature lines covering the payload
	Signatures []note.Signature
}

// Sign adds a signature to a SignedCheckpoint object
// The signature is added to the signature array as well as being directly returned to the caller
func (s *SignedNote) Sign(identity string, signer signature.Signer, opts signature.SignOption) (*note.Signature, error) {
	sig, err := signer.SignMessage(bytes.NewReader([]byte(s.Note)), opts)
	if err != nil {
		return nil, fmt.Errorf("signing note: %w", err)
	}

	pk, err := signer.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("retrieving public key: %w", err)
	}
	pkHash, err := getPublicKeyHash(pk)
	if err != nil {
		return nil, err
	}

	signature := note.Signature{
		Name:   identity,
		Hash:   pkHash,
		Base64: base64.StdEncoding.EncodeToString(sig),
	}

	s.Signatures = append(s.Signatures, signature)
	return &signature, nil
}

// Verify checks that one of the signatures can be successfully verified using
// the supplied public key
func (s SignedNote) Verify(verifier signature.Verifier) bool {
	if len(s.Signatures) == 0 {
		return false
	}

	msg := []byte(s.Note)
	digest := sha256.Sum256(msg)

	pk, err := verifier.PublicKey()
	if err != nil {
		return false
	}
	verifierPkHash, err := getPublicKeyHash(pk)
	if err != nil {
		return false
	}

	for _, s := range s.Signatures {
		sigBytes, err := base64.StdEncoding.DecodeString(s.Base64)
		if err != nil {
			return false
		}

		if s.Hash != verifierPkHash {
			return false
		}

		opts := []signature.VerifyOption{}
		switch pk.(type) {
		case *rsa.PublicKey, *ecdsa.PublicKey:
			opts = append(opts, options.WithDigest(digest[:]))
		case ed25519.PublicKey:
			break
		default:
			return false
		}
		if err := verifier.VerifySignature(bytes.NewReader(sigBytes), bytes.NewReader(msg), opts...); err != nil {
			return false
		}
	}
	return true
}

// MarshalText returns the common format representation of this SignedNote.
func (s SignedNote) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// String returns the String representation of the SignedNote
func (s SignedNote) String() string {
	var b strings.Builder
	b.WriteString(s.Note)
	b.WriteRune('\n')
	for _, sig := range s.Signatures {
		var hbuf [4]byte
		binary.BigEndian.PutUint32(hbuf[:], sig.Hash)
		sigBytes, _ := base64.StdEncoding.DecodeString(sig.Base64)
		b64 := base64.StdEncoding.EncodeToString(append(hbuf[:], sigBytes...))
		fmt.Fprintf(&b, "%c %s %s\n", '\u2014', sig.Name, b64)
	}

	return b.String()
}

// UnmarshalText parses the common formatted signed note data and stores the result
// in the SignedNote. THIS DOES NOT VERIFY SIGNATURES INSIDE THE CONTENT!
//
// The supplied data is expected to contain a single Note, followed by a single
// line with no comment, followed by one or more lines with the following format:
//
// \u2014 name signature
//
//   - name is the string associated with the signer
//   - signature is a base64 encoded string; the first 4 bytes of the decoded value is a
//     hint to the public key; it is a big-endian encoded uint32 representing the first
//     4 bytes of the SHA256 hash of the public key
func (s *SignedNote) UnmarshalText(data []byte) error {
	sigSplit := []byte("\n\n")
	// Must end with signature block preceded by blank line.
	split := bytes.LastIndex(data, sigSplit)
	if split < 0 {
		return errors.New("malformed note")
	}
	text, data := data[:split+1], data[split+2:]
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return errors.New("malformed note")
	}

	sn := SignedNote{
		Note: string(text),
	}

	b := bufio.NewScanner(bytes.NewReader(data))
	for b.Scan() {
		var name, signature string
		if _, err := fmt.Fscanf(strings.NewReader(b.Text()), "\u2014 %s %s\n", &name, &signature); err != nil {
			return fmt.Errorf("parsing signature: %w", err)
		}

		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			return fmt.Errorf("decoding signature: %w", err)
		}
		if len(sigBytes) < 5 {
			return errors.New("signature is too small")
		}

		sig := note.Signature{
			Name:   name,
			Hash:   binary.BigEndian.Uint32(sigBytes[0:4]),
			Base64: base64.StdEncoding.EncodeToString(sigBytes[4:]),
		}
		sn.Signatures = append(sn.Signatures, sig)

	}
	if len(sn.Signatures) == 0 {
		return errors.New("no signatures found in input")
	}

	// copy sc to s
	*s = sn
	return nil
}

func SignedNoteValidator(strToValidate string) bool {
	s := SignedNote{}
	return s.UnmarshalText([]byte(strToValidate)) == nil
}

func getPublicKeyHash(publicKey crypto.PublicKey) (uint32, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return 0, fmt.Errorf("marshalling public key: %w", err)
	}
	pkSha := sha256.Sum256(pubKeyBytes)
	hash := binary.BigEndian.Uint32(pkSha[:])
	return hash, nil
}
