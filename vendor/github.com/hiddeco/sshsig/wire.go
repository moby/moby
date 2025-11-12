package sshsig

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/ssh"
)

var (
	// ErrUnsupportedSignatureVersion is returned when the signature version is
	// not supported.
	ErrUnsupportedSignatureVersion = errors.New("unsupported signature version")
	// ErrInvalidMagicPreamble is returned when the magic preamble is invalid.
	ErrInvalidMagicPreamble = errors.New("invalid magic preamble")
)

// sigVersion is the supported version of the SSH signature format.
// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L35
const sigVersion = 1

// magicPreamble is the six-byte sequence "SSHSIG". It is included to
// ensure that manual signatures can never be confused with any message
// signed during SSH user or host authentication.
// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L89-L91
var magicPreamble = [6]byte{'S', 'S', 'H', 'S', 'I', 'G'}

// signedData represents data that is signed.
// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L79
type signedData struct {
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Hash          string
}

// Marshal returns the signed data in SSH wire format.
func (s signedData) Marshal() []byte {
	return append(magicPreamble[:], ssh.Marshal(s)...)
}

// blob represents the SSH signature blob.
// xref: https://github.com/openssh/openssh-portable/blob/V_9_2_P1/PROTOCOL.sshsig#L32
type blob struct {
	// MagicPreamble is included in the struct to ensure we can unmarshal the
	// blob correctly.
	MagicPreamble [6]byte
	Version       uint32
	PublicKey     string
	Namespace     string
	Reserved      string
	HashAlgorithm string
	Signature     string
}

// Validate returns an error if the blob is invalid. This does not check the
// signature itself.
func (b blob) Validate() error {
	if b.Version != sigVersion {
		return fmt.Errorf("%w %d: expected %d", ErrUnsupportedSignatureVersion, b.Version, sigVersion)
	}
	if b.MagicPreamble != magicPreamble {
		return fmt.Errorf("%w %q: expected %q", ErrInvalidMagicPreamble, b.MagicPreamble, magicPreamble)
	}
	if err := HashAlgorithm(b.HashAlgorithm).Supported(); err != nil {
		return err
	}
	return nil
}

// Marshal returns the blob in SSH wire format.
func (b blob) Marshal() []byte {
	copy(b.MagicPreamble[:], magicPreamble[:])
	return ssh.Marshal(b)
}
