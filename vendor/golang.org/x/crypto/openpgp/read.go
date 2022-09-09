// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package openpgp implements high level operations on OpenPGP messages.
//
// Deprecated: this package is unmaintained except for security fixes. New
// applications should consider a more focused, modern alternative to OpenPGP
// for their specific task. If you are required to interoperate with OpenPGP
// systems and need a maintained package, consider a community fork.
// See https://golang.org/issue/44226.
package openpgp // import "golang.org/x/crypto/openpgp"

import (
	"crypto"
	_ "crypto/sha256"
	"hash"
	"io"
	"strconv"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/errors"
	"golang.org/x/crypto/openpgp/packet"
)

// SignatureType is the armor type for a PGP signature.
var SignatureType = "PGP SIGNATURE"

// readArmored reads an armored block with the given type.
func readArmored(r io.Reader, expectedType string) (body io.Reader, err error) {
	block, err := armor.Decode(r)
	if err != nil {
		return
	}

	if block.Type != expectedType {
		return nil, errors.InvalidArgumentError("expected '" + expectedType + "', got: " + block.Type)
	}

	return block.Body, nil
}

// MessageDetails contains the result of parsing an OpenPGP encrypted and/or
// signed message.
type MessageDetails struct {
	IsEncrypted              bool                // true if the message was encrypted.
	EncryptedToKeyIds        []uint64            // the list of recipient key ids.
	IsSymmetricallyEncrypted bool                // true if a passphrase could have decrypted the message.
	DecryptedWith            Key                 // the private key used to decrypt the message, if any.
	IsSigned                 bool                // true if the message is signed.
	SignedByKeyId            uint64              // the key id of the signer, if any.
	SignedBy                 *Key                // the key of the signer, if available.
	LiteralData              *packet.LiteralData // the metadata of the contents
	UnverifiedBody           io.Reader           // the contents of the message.

	// If IsSigned is true and SignedBy is non-zero then the signature will
	// be verified as UnverifiedBody is read. The signature cannot be
	// checked until the whole of UnverifiedBody is read so UnverifiedBody
	// must be consumed until EOF before the data can be trusted. Even if a
	// message isn't signed (or the signer is unknown) the data may contain
	// an authentication code that is only checked once UnverifiedBody has
	// been consumed. Once EOF has been seen, the following fields are
	// valid. (An authentication code failure is reported as a
	// SignatureError error when reading from UnverifiedBody.)
	SignatureError error               // nil if the signature is good.
	Signature      *packet.Signature   // the signature packet itself, if v4 (default)
	SignatureV3    *packet.SignatureV3 // the signature packet if it is a v2 or v3 signature

	decrypted io.ReadCloser
}

// A PromptFunction is used as a callback by functions that may need to decrypt
// a private key, or prompt for a passphrase. It is called with a list of
// acceptable, encrypted private keys and a boolean that indicates whether a
// passphrase is usable. It should either decrypt a private key or return a
// passphrase to try. If the decrypted private key or given passphrase isn't
// correct, the function will be called again, forever. Any error returned will
// be passed up.
type PromptFunction func(keys []Key, symmetric bool) ([]byte, error)

// A keyEnvelopePair is used to store a private key with the envelope that
// contains a symmetric key, encrypted with that key.
type keyEnvelopePair struct {
	key          Key
	encryptedKey *packet.EncryptedKey
}

// ReadMessage parses an OpenPGP message that may be signed and/or encrypted.
// The given KeyRing should contain both public keys (for signature
// verification) and, possibly encrypted, private keys for decrypting.
// If config is nil, sensible defaults will be used.
func ReadMessage(r io.Reader, keyring KeyRing, prompt PromptFunction, config *packet.Config) (md *MessageDetails, err error) {
	var p packet.Packet

	var symKeys []*packet.SymmetricKeyEncrypted
	var pubKeys []keyEnvelopePair
	var se *packet.SymmetricallyEncrypted

	packets := packet.NewReader(r)
	md = new(MessageDetails)
	md.IsEncrypted = true

	// The message, if encrypted, starts with a number of packets
	// containing an encrypted decryption key. The decryption key is either
	// encrypted to a public key, or with a passphrase. This loop
	// collects these packets.
ParsePackets:
	for {
		p, err = packets.Next()
		if err != nil {
			return nil, err
		}
		switch p := p.(type) {
		case *packet.SymmetricKeyEncrypted:
			// This packet contains the decryption key encrypted with a passphrase.
			md.IsSymmetricallyEncrypted = true
			symKeys = append(symKeys, p)
		case *packet.EncryptedKey:
			// This packet contains the decryption key encrypted to a public key.
			md.EncryptedToKeyIds = append(md.EncryptedToKeyIds, p.KeyId)
			switch p.Algo {
			case packet.PubKeyAlgoRSA, packet.PubKeyAlgoRSAEncryptOnly, packet.PubKeyAlgoElGamal:
				break
			default:
				continue
			}
			var keys []Key
			if p.KeyId == 0 {
				keys = keyring.DecryptionKeys()
			} else {
				keys = keyring.KeysById(p.KeyId)
			}
			for _, k := range keys {
				pubKeys = append(pubKeys, keyEnvelopePair{k, p})
			}
		case *packet.SymmetricallyEncrypted:
			se = p
			break ParsePackets
		case *packet.Compressed, *packet.LiteralData, *packet.OnePassSignature:
			// This message isn't encrypted.
			if len(symKeys) != 0 || len(pubKeys) != 0 {
				return nil, errors.StructuralError("key material not followed by encrypted message")
			}
			packets.Unread(p)
			return readSignedMessage(packets, nil, keyring)
		}
	}

	var candidates []Key
	var decrypted io.ReadCloser

	// Now that we have the list of encrypted keys we need to decrypt at
	// least one of them or, if we cannot, we need to call the prompt
	// function so that it can decrypt a key or give us a passphrase.
FindKey:
	for {
		// See if any of the keys already have a private key available
		candidates = candidates[:0]
		candidateFingerprints := make(map[string]bool)

		for _, pk := range pubKeys {
			if pk.key.PrivateKey == nil {
				continue
			}
			if !pk.key.PrivateKey.Encrypted {
				if len(pk.encryptedKey.Key) == 0 {
					pk.encryptedKey.Decrypt(pk.key.PrivateKey, config)
				}
				if len(pk.encryptedKey.Key) == 0 {
					continue
				}
				decrypted, err = se.Decrypt(pk.encryptedKey.CipherFunc, pk.encryptedKey.Key)
				if err != nil && err != errors.ErrKeyIncorrect {
					return nil, err
				}
				if decrypted != nil {
					md.DecryptedWith = pk.key
					break FindKey
				}
			} else {
				fpr := string(pk.key.PublicKey.Fingerprint[:])
				if v := candidateFingerprints[fpr]; v {
					continue
				}
				candidates = append(candidates, pk.key)
				candidateFingerprints[fpr] = true
			}
		}

		if len(candidates) == 0 && len(symKeys) == 0 {
			return nil, errors.ErrKeyIncorrect
		}

		if prompt == nil {
			return nil, errors.ErrKeyIncorrect
		}

		passphrase, err := prompt(candidates, len(symKeys) != 0)
		if err != nil {
			return nil, err
		}

		// Try the symmetric passphrase first
		if len(symKeys) != 0 && passphrase != nil {
			for _, s := range symKeys {
				key, cipherFunc, err := s.Decrypt(passphrase)
				if err == nil {
					decrypted, err = se.Decrypt(cipherFunc, key)
					if err != nil && err != errors.ErrKeyIncorrect {
						return nil, err
					}
					if decrypted != nil {
						break FindKey
					}
				}

			}
		}
	}

	md.decrypted = decrypted
	if err := packets.Push(decrypted); err != nil {
		return nil, err
	}
	return readSignedMessage(packets, md, keyring)
}

// readSignedMessage reads a possibly signed message if mdin is non-zero then
// that structure is updated and returned. Otherwise a fresh MessageDetails is
// used.
func readSignedMessage(packets *packet.Reader, mdin *MessageDetails, keyring KeyRing) (md *MessageDetails, err error) {
	if mdin == nil {
		mdin = new(MessageDetails)
	}
	md = mdin

	var p packet.Packet
	var h hash.Hash
	var wrappedHash hash.Hash
FindLiteralData:
	for {
		p, err = packets.Next()
		if err != nil {
			return nil, err
		}
		switch p := p.(type) {
		case *packet.Compressed:
			if err := packets.Push(p.Body); err != nil {
				return nil, err
			}
		case *packet.OnePassSignature:
			if !p.IsLast {
				return nil, errors.UnsupportedError("nested signatures")
			}

			h, wrappedHash, err = hashForSignature(p.Hash, p.SigType)
			if err != nil {
				md = nil
				return
			}

			md.IsSigned = true
			md.SignedByKeyId = p.KeyId
			keys := keyring.KeysByIdUsage(p.KeyId, packet.KeyFlagSign)
			if len(keys) > 0 {
				md.SignedBy = &keys[0]
			}
		case *packet.LiteralData:
			md.LiteralData = p
			break FindLiteralData
		}
	}

	if md.SignedBy != nil {
		md.UnverifiedBody = &signatureCheckReader{packets, h, wrappedHash, md}
	} else if md.decrypted != nil {
		md.UnverifiedBody = checkReader{md}
	} else {
		md.UnverifiedBody = md.LiteralData.Body
	}

	return md, nil
}

// hashForSignature returns a pair of hashes that can be used to verify a
// signature. The signature may specify that the contents of the signed message
// should be preprocessed (i.e. to normalize line endings). Thus this function
// returns two hashes. The second should be used to hash the message itself and
// performs any needed preprocessing.
func hashForSignature(hashId crypto.Hash, sigType packet.SignatureType) (hash.Hash, hash.Hash, error) {
	if !hashId.Available() {
		return nil, nil, errors.UnsupportedError("hash not available: " + strconv.Itoa(int(hashId)))
	}
	h := hashId.New()

	switch sigType {
	case packet.SigTypeBinary:
		return h, h, nil
	case packet.SigTypeText:
		return h, NewCanonicalTextHash(h), nil
	}

	return nil, nil, errors.UnsupportedError("unsupported signature type: " + strconv.Itoa(int(sigType)))
}

// checkReader wraps an io.Reader from a LiteralData packet. When it sees EOF
// it closes the ReadCloser from any SymmetricallyEncrypted packet to trigger
// MDC checks.
type checkReader struct {
	md *MessageDetails
}

func (cr checkReader) Read(buf []byte) (n int, err error) {
	n, err = cr.md.LiteralData.Body.Read(buf)
	if err == io.EOF {
		mdcErr := cr.md.decrypted.Close()
		if mdcErr != nil {
			err = mdcErr
		}
	}
	return
}

// signatureCheckReader wraps an io.Reader from a LiteralData packet and hashes
// the data as it is read. When it sees an EOF from the underlying io.Reader
// it parses and checks a trailing Signature packet and triggers any MDC checks.
type signatureCheckReader struct {
	packets        *packet.Reader
	h, wrappedHash hash.Hash
	md             *MessageDetails
}

func (scr *signatureCheckReader) Read(buf []byte) (n int, err error) {
	n, err = scr.md.LiteralData.Body.Read(buf)
	scr.wrappedHash.Write(buf[:n])
	if err == io.EOF {
		var p packet.Packet
		p, scr.md.SignatureError = scr.packets.Next()
		if scr.md.SignatureError != nil {
			return
		}

		var ok bool
		if scr.md.Signature, ok = p.(*packet.Signature); ok {
			scr.md.SignatureError = scr.md.SignedBy.PublicKey.VerifySignature(scr.h, scr.md.Signature)
		} else if scr.md.SignatureV3, ok = p.(*packet.SignatureV3); ok {
			scr.md.SignatureError = scr.md.SignedBy.PublicKey.VerifySignatureV3(scr.h, scr.md.SignatureV3)
		} else {
			scr.md.SignatureError = errors.StructuralError("LiteralData not followed by Signature")
			return
		}

		// The SymmetricallyEncrypted packet, if any, might have an
		// unsigned hash of its own. In order to check this we need to
		// close that Reader.
		if scr.md.decrypted != nil {
			mdcErr := scr.md.decrypted.Close()
			if mdcErr != nil {
				err = mdcErr
			}
		}
	}
	return
}

// CheckDetachedSignature takes a signed file and a detached signature and
// returns the signer if the signature is valid. If the signer isn't known,
// ErrUnknownIssuer is returned.
func CheckDetachedSignature(keyring KeyRing, signed, signature io.Reader) (signer *Entity, err error) {
	var issuerKeyId uint64
	var hashFunc crypto.Hash
	var sigType packet.SignatureType
	var keys []Key
	var p packet.Packet

	packets := packet.NewReader(signature)
	for {
		p, err = packets.Next()
		if err == io.EOF {
			return nil, errors.ErrUnknownIssuer
		}
		if err != nil {
			return nil, err
		}

		switch sig := p.(type) {
		case *packet.Signature:
			if sig.IssuerKeyId == nil {
				return nil, errors.StructuralError("signature doesn't have an issuer")
			}
			issuerKeyId = *sig.IssuerKeyId
			hashFunc = sig.Hash
			sigType = sig.SigType
		case *packet.SignatureV3:
			issuerKeyId = sig.IssuerKeyId
			hashFunc = sig.Hash
			sigType = sig.SigType
		default:
			return nil, errors.StructuralError("non signature packet found")
		}

		keys = keyring.KeysByIdUsage(issuerKeyId, packet.KeyFlagSign)
		if len(keys) > 0 {
			break
		}
	}

	if len(keys) == 0 {
		panic("unreachable")
	}

	h, wrappedHash, err := hashForSignature(hashFunc, sigType)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(wrappedHash, signed); err != nil && err != io.EOF {
		return nil, err
	}

	for _, key := range keys {
		switch sig := p.(type) {
		case *packet.Signature:
			err = key.PublicKey.VerifySignature(h, sig)
		case *packet.SignatureV3:
			err = key.PublicKey.VerifySignatureV3(h, sig)
		default:
			panic("unreachable")
		}

		if err == nil {
			return key.Entity, nil
		}
	}

	return nil, err
}

// CheckArmoredDetachedSignature performs the same actions as
// CheckDetachedSignature but expects the signature to be armored.
func CheckArmoredDetachedSignature(keyring KeyRing, signed, signature io.Reader) (signer *Entity, err error) {
	body, err := readArmored(signature, SignatureType)
	if err != nil {
		return
	}

	return CheckDetachedSignature(keyring, signed, body)
}
