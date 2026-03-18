package ct

// This file contains selectively chosen snippets of
// github.com/google/certificate-transparency-go@ 5cfe585726ad9d990d4db524d6ce2567b13e2f80
//
// These snippets only perform deserialization for SCTs and are recreated here to prevent pulling in the whole of the ct
// which contains yet another version of x509,asn1 and tls

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Variable size structure prefix-header byte lengths
const (
	CertificateLengthBytes      = 3
	PreCertificateLengthBytes   = 3
	ExtensionsLengthBytes       = 2
	CertificateChainLengthBytes = 3
	SignatureLengthBytes        = 2
)

func writeUint(w io.Writer, value uint64, numBytes int) error {
	buf := make([]uint8, numBytes)
	for i := 0; i < numBytes; i++ {
		buf[numBytes-i-1] = uint8(value & 0xff)
		value >>= 8
	}
	if value != 0 {
		return errors.New("numBytes was insufficiently large to represent value")
	}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}

func writeVarBytes(w io.Writer, value []byte, numLenBytes int) error {
	if err := writeUint(w, uint64(len(value)), numLenBytes); err != nil {
		return err
	}
	if _, err := w.Write(value); err != nil {
		return err
	}
	return nil
}

func readUint(r io.Reader, numBytes int) (uint64, error) {
	var l uint64
	for i := 0; i < numBytes; i++ {
		l <<= 8
		var t uint8
		if err := binary.Read(r, binary.BigEndian, &t); err != nil {
			return 0, err
		}
		l |= uint64(t)
	}
	return l, nil
}

// Reads a variable length array of bytes from |r|. |numLenBytes| specifies the
// number of (BigEndian) prefix-bytes which contain the length of the actual
// array data bytes that follow.
// Allocates an array to hold the contents and returns a slice view into it if
// the read was successful, or an error otherwise.
func readVarBytes(r io.Reader, numLenBytes int) ([]byte, error) {
	switch {
	case numLenBytes > 8:
		return nil, fmt.Errorf("numLenBytes too large (%d)", numLenBytes)
	case numLenBytes == 0:
		return nil, errors.New("numLenBytes should be > 0")
	}
	l, err := readUint(r, numLenBytes)
	if err != nil {
		return nil, err
	}
	data := make([]byte, l)
	if n, err := io.ReadFull(r, data); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("short read: expected %d but got %d", l, n)
		}
		return nil, err
	}
	return data, nil
}

// UnmarshalDigitallySigned reconstructs a DigitallySigned structure from a Reader
func UnmarshalDigitallySigned(r io.Reader) (*DigitallySigned, error) {
	var h byte
	if err := binary.Read(r, binary.BigEndian, &h); err != nil {
		return nil, fmt.Errorf("failed to read HashAlgorithm: %v", err)
	}

	var s byte
	if err := binary.Read(r, binary.BigEndian, &s); err != nil {
		return nil, fmt.Errorf("failed to read SignatureAlgorithm: %v", err)
	}

	sig, err := readVarBytes(r, SignatureLengthBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read Signature bytes: %v", err)
	}

	return &DigitallySigned{
		HashAlgorithm:      HashAlgorithm(h),
		SignatureAlgorithm: SignatureAlgorithm(s),
		Signature:          sig,
	}, nil
}

func marshalDigitallySignedHere(ds DigitallySigned, here []byte) ([]byte, error) {
	sigLen := len(ds.Signature)
	dsOutLen := 2 + SignatureLengthBytes + sigLen
	if here == nil {
		here = make([]byte, dsOutLen)
	}
	if len(here) < dsOutLen {
		return nil, ErrNotEnoughBuffer
	}
	here = here[0:dsOutLen]

	here[0] = byte(ds.HashAlgorithm)
	here[1] = byte(ds.SignatureAlgorithm)
	binary.BigEndian.PutUint16(here[2:4], uint16(sigLen))
	copy(here[4:], ds.Signature)

	return here, nil
}

// MarshalDigitallySigned marshalls a DigitallySigned structure into a byte array
func MarshalDigitallySigned(ds DigitallySigned) ([]byte, error) {
	return marshalDigitallySignedHere(ds, nil)
}

func deserializeSCTV1(r io.Reader, sct *SignedCertificateTimestamp) error {
	if err := binary.Read(r, binary.BigEndian, &sct.LogID); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &sct.Timestamp); err != nil {
		return err
	}
	ext, err := readVarBytes(r, ExtensionsLengthBytes)
	if err != nil {
		return err
	}
	sct.Extensions = ext
	ds, err := UnmarshalDigitallySigned(r)
	if err != nil {
		return err
	}
	sct.Signature = *ds
	return nil
}

func DeserializeSCT(r io.Reader) (*SignedCertificateTimestamp, error) {
	var sct SignedCertificateTimestamp
	if err := binary.Read(r, binary.BigEndian, &sct.SCTVersion); err != nil {
		return nil, err
	}
	switch sct.SCTVersion {
	case V1:
		return &sct, deserializeSCTV1(r, &sct)
	default:
		return nil, fmt.Errorf("unknown SCT version %d", sct.SCTVersion)
	}
}
