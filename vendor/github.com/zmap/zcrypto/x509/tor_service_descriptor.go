package x509

import (
	"encoding/asn1"
	"github.com/zmap/zcrypto/x509/pkix"
)

var (
	// oidBRTorServiceDescriptor is the assigned OID for the CAB Forum Tor Service
	// Descriptor Hash extension (see EV Guidelines Appendix F)
	oidBRTorServiceDescriptor = asn1.ObjectIdentifier{2, 23, 140, 1, 31}
)

// TorServiceDescriptorHash is a structure corrsponding to the
// TorServiceDescriptorHash SEQUENCE described in Appendix F ("Issuance of
// Certificates for .onion Domain Names").
//
// Each TorServiceDescriptorHash holds an onion URI (a utf8 string with the
// .onion address that was validated), a hash algorithm name (computed based on
// the pkix.AlgorithmIdentifier in the TorServiceDescriptorHash), the hash bytes
// (computed over the DER encoding of the ASN.1 SubjectPublicKey of the .onion
// service), and the number of bits in the hash bytes.
type TorServiceDescriptorHash struct {
	Onion         string                   `json:"onion"`
	Algorithm     pkix.AlgorithmIdentifier `json:"-"`
	AlgorithmName string                   `json:"algorithm_name"`
	Hash          CertificateFingerprint   `json:"hash"`
	HashBits      int                      `json:"hash_bits"`
}

// parseTorServiceDescriptorSyntax parses the given pkix.Extension (assumed to
// have OID == oidBRTorServiceDescriptor) and returns a slice of parsed
// TorServiceDescriptorHash objects, or an error. An error will be returned if
// there are any structural errors related to the ASN.1 content (wrong tags,
// trailing data, missing fields, etc).
func parseTorServiceDescriptorSyntax(ext pkix.Extension) ([]*TorServiceDescriptorHash, error) {
	// TorServiceDescriptorSyntax ::=
	//    SEQUENCE ( 1..MAX ) of TorServiceDescriptorHash
	var seq asn1.RawValue
	rest, err := asn1.Unmarshal(ext.Value, &seq)
	if err != nil {
		return nil, asn1.SyntaxError{
			Msg: "unable to unmarshal outer TorServiceDescriptor SEQUENCE",
		}
	}
	if len(rest) != 0 {
		return nil, asn1.SyntaxError{
			Msg: "trailing data after outer TorServiceDescriptor SEQUENCE",
		}
	}
	if seq.Tag != asn1.TagSequence || seq.Class != asn1.ClassUniversal || !seq.IsCompound {
		return nil, asn1.SyntaxError{
			Msg: "invalid outer TorServiceDescriptor SEQUENCE",
		}
	}

	var descriptors []*TorServiceDescriptorHash
	rest = seq.Bytes
	for len(rest) > 0 {
		var descriptor *TorServiceDescriptorHash
		descriptor, rest, err = parseTorServiceDescriptorHash(rest)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

// parseTorServiceDescriptorHash unmarshals a SEQUENCE from the provided data
// and parses a TorServiceDescriptorHash using the data contained in the
// sequence. The TorServiceDescriptorHash object and the remaining data are
// returned if no error occurs.
func parseTorServiceDescriptorHash(data []byte) (*TorServiceDescriptorHash, []byte, error) {
	// TorServiceDescriptorHash:: = SEQUENCE {
	//   onionURI UTF8String
	//   algorithm AlgorithmIdentifier
	//   subjectPublicKeyHash BIT STRING
	// }
	var outerSeq asn1.RawValue
	var err error
	data, err = asn1.Unmarshal(data, &outerSeq)
	if err != nil {
		return nil, data, asn1.SyntaxError{
			Msg: "error unmarshaling TorServiceDescriptorHash SEQUENCE",
		}
	}
	if outerSeq.Tag != asn1.TagSequence ||
		outerSeq.Class != asn1.ClassUniversal ||
		!outerSeq.IsCompound {
		return nil, data, asn1.SyntaxError{
			Msg: "TorServiceDescriptorHash missing compound SEQUENCE tag",
		}
	}
	fieldData := outerSeq.Bytes

	// Unmarshal and verify the structure of the onionURI UTF8String field.
	var rawOnionURI asn1.RawValue
	fieldData, err = asn1.Unmarshal(fieldData, &rawOnionURI)
	if err != nil {
		return nil, data, asn1.SyntaxError{
			Msg: "error unmarshaling TorServiceDescriptorHash onionURI",
		}
	}
	if rawOnionURI.Tag != asn1.TagUTF8String ||
		rawOnionURI.Class != asn1.ClassUniversal ||
		rawOnionURI.IsCompound {
		return nil, data, asn1.SyntaxError{
			Msg: "TorServiceDescriptorHash missing non-compound UTF8String tag",
		}
	}

	// Unmarshal and verify the structure of the algorithm UTF8String field.
	var algorithm pkix.AlgorithmIdentifier
	fieldData, err = asn1.Unmarshal(fieldData, &algorithm)
	if err != nil {
		return nil, nil, asn1.SyntaxError{
			Msg: "error unmarshaling TorServiceDescriptorHash algorithm",
		}
	}

	var algorithmName string
	if algorithm.Algorithm.Equal(oidSHA256) {
		algorithmName = "SHA256"
	} else if algorithm.Algorithm.Equal(oidSHA384) {
		algorithmName = "SHA384"
	} else if algorithm.Algorithm.Equal(oidSHA512) {
		algorithmName = "SHA512"
	} else {
		algorithmName = "Unknown"
	}

	// Unmarshal and verify the structure of the Subject Public Key Hash BitString
	// field.
	var spkh asn1.BitString
	fieldData, err = asn1.Unmarshal(fieldData, &spkh)
	if err != nil {
		return nil, data, asn1.SyntaxError{
			Msg: "error unmarshaling TorServiceDescriptorHash Hash",
		}
	}

	// There should be no trailing data after the TorServiceDescriptorHash
	// SEQUENCE.
	if len(fieldData) > 0 {
		return nil, data, asn1.SyntaxError{
			Msg: "trailing data after TorServiceDescriptorHash",
		}
	}

	return &TorServiceDescriptorHash{
		Onion:         string(rawOnionURI.Bytes),
		Algorithm:     algorithm,
		AlgorithmName: algorithmName,
		HashBits:      spkh.BitLength,
		Hash:          CertificateFingerprint(spkh.Bytes),
	}, data, nil
}
