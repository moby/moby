package timestamp

import (
	"crypto"
	"encoding/asn1"
)

// TODO(vanbroup): taken from "golang.org/x/crypto/ocsp"
// use directly from crypto/x509 when exported as suggested below.

var hashOIDs = map[crypto.Hash]asn1.ObjectIdentifier{
	crypto.SHA1:   asn1.ObjectIdentifier([]int{1, 3, 14, 3, 2, 26}),
	crypto.SHA256: asn1.ObjectIdentifier([]int{2, 16, 840, 1, 101, 3, 4, 2, 1}),
	crypto.SHA384: asn1.ObjectIdentifier([]int{2, 16, 840, 1, 101, 3, 4, 2, 2}),
	crypto.SHA512: asn1.ObjectIdentifier([]int{2, 16, 840, 1, 101, 3, 4, 2, 3}),
}

// TODO(rlb): This is not taken from crypto/x509, but it's of the same general form.
func getHashAlgorithmFromOID(target asn1.ObjectIdentifier) crypto.Hash {
	for hash, oid := range hashOIDs {
		if oid.Equal(target) {
			return hash
		}
	}
	return crypto.Hash(0)
}

func getOIDFromHashAlgorithm(target crypto.Hash) asn1.ObjectIdentifier {
	for hash, oid := range hashOIDs {
		if hash == target {
			return oid
		}
	}
	return nil
}

// TODO(vanbroup): taken from golang.org/x/crypto/x509
// asn1BitLength returns the bit-length of bitString by considering the
// most-significant bit in a byte to be the "first" bit. This convention
// matches ASN.1, but differs from almost everything else.
func asn1BitLength(bitString []byte) int {
	bitLen := len(bitString) * 8

	for i := range bitString {
		b := bitString[len(bitString)-i-1]

		for bit := uint(0); bit < 8; bit++ {
			if (b>>bit)&1 == 1 {
				return bitLen
			}
			bitLen--
		}
	}

	return 0
}
