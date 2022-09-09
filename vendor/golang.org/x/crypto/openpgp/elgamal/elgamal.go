// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package elgamal implements ElGamal encryption, suitable for OpenPGP,
// as specified in "A Public-Key Cryptosystem and a Signature Scheme Based on
// Discrete Logarithms," IEEE Transactions on Information Theory, v. IT-31,
// n. 4, 1985, pp. 469-472.
//
// This form of ElGamal embeds PKCS#1 v1.5 padding, which may make it
// unsuitable for other protocols. RSA should be used in preference in any
// case.
//
// Deprecated: this package was only provided to support ElGamal encryption in
// OpenPGP. The golang.org/x/crypto/openpgp package is now deprecated (see
// https://golang.org/issue/44226), and ElGamal in the OpenPGP ecosystem has
// compatibility and security issues (see https://eprint.iacr.org/2021/923).
// Moreover, this package doesn't protect against side-channel attacks.
package elgamal // import "golang.org/x/crypto/openpgp/elgamal"

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"io"
	"math/big"
)

// PublicKey represents an ElGamal public key.
type PublicKey struct {
	G, P, Y *big.Int
}

// PrivateKey represents an ElGamal private key.
type PrivateKey struct {
	PublicKey
	X *big.Int
}

// Encrypt encrypts the given message to the given public key. The result is a
// pair of integers. Errors can result from reading random, or because msg is
// too large to be encrypted to the public key.
func Encrypt(random io.Reader, pub *PublicKey, msg []byte) (c1, c2 *big.Int, err error) {
	pLen := (pub.P.BitLen() + 7) / 8
	if len(msg) > pLen-11 {
		err = errors.New("elgamal: message too long")
		return
	}

	// EM = 0x02 || PS || 0x00 || M
	em := make([]byte, pLen-1)
	em[0] = 2
	ps, mm := em[1:len(em)-len(msg)-1], em[len(em)-len(msg):]
	err = nonZeroRandomBytes(ps, random)
	if err != nil {
		return
	}
	em[len(em)-len(msg)-1] = 0
	copy(mm, msg)

	m := new(big.Int).SetBytes(em)

	k, err := rand.Int(random, pub.P)
	if err != nil {
		return
	}

	c1 = new(big.Int).Exp(pub.G, k, pub.P)
	s := new(big.Int).Exp(pub.Y, k, pub.P)
	c2 = s.Mul(s, m)
	c2.Mod(c2, pub.P)

	return
}

// Decrypt takes two integers, resulting from an ElGamal encryption, and
// returns the plaintext of the message. An error can result only if the
// ciphertext is invalid. Users should keep in mind that this is a padding
// oracle and thus, if exposed to an adaptive chosen ciphertext attack, can
// be used to break the cryptosystem.  See ``Chosen Ciphertext Attacks
// Against Protocols Based on the RSA Encryption Standard PKCS #1'', Daniel
// Bleichenbacher, Advances in Cryptology (Crypto '98),
func Decrypt(priv *PrivateKey, c1, c2 *big.Int) (msg []byte, err error) {
	s := new(big.Int).Exp(c1, priv.X, priv.P)
	if s.ModInverse(s, priv.P) == nil {
		return nil, errors.New("elgamal: invalid private key")
	}
	s.Mul(s, c2)
	s.Mod(s, priv.P)
	em := s.Bytes()

	firstByteIsTwo := subtle.ConstantTimeByteEq(em[0], 2)

	// The remainder of the plaintext must be a string of non-zero random
	// octets, followed by a 0, followed by the message.
	//   lookingForIndex: 1 iff we are still looking for the zero.
	//   index: the offset of the first zero byte.
	var lookingForIndex, index int
	lookingForIndex = 1

	for i := 1; i < len(em); i++ {
		equals0 := subtle.ConstantTimeByteEq(em[i], 0)
		index = subtle.ConstantTimeSelect(lookingForIndex&equals0, i, index)
		lookingForIndex = subtle.ConstantTimeSelect(equals0, 0, lookingForIndex)
	}

	if firstByteIsTwo != 1 || lookingForIndex != 0 || index < 9 {
		return nil, errors.New("elgamal: decryption error")
	}
	return em[index+1:], nil
}

// nonZeroRandomBytes fills the given slice with non-zero random octets.
func nonZeroRandomBytes(s []byte, rand io.Reader) (err error) {
	_, err = io.ReadFull(rand, s)
	if err != nil {
		return
	}

	for i := 0; i < len(s); i++ {
		for s[i] == 0 {
			_, err = io.ReadFull(rand, s[i:i+1])
			if err != nil {
				return
			}
		}
	}

	return
}
