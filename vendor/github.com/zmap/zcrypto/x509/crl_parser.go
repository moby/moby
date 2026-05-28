package x509

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/zmap/zcrypto/cryptobyte"
	cryptobyte_asn1 "github.com/zmap/zcrypto/cryptobyte/asn1"
	"github.com/zmap/zcrypto/encoding/asn1"
	"github.com/zmap/zcrypto/x509/pkix"
)

const x509v2Version = 1

// RevokedCertificate represents an entry in the revokedCertificates sequence of
// a CRL.
// STARTBLOCK: This type does not exist in upstream.
type RevokedCertificate struct {
	// Raw contains the raw bytes of the revokedCertificates entry. It is set when
	// parsing a CRL; it is ignored when generating a CRL.
	Raw []byte

	// SerialNumber represents the serial number of a revoked certificate. It is
	// both used when creating a CRL and populated when parsing a CRL. It MUST NOT
	// be nil.
	SerialNumber *big.Int
	// RevocationTime represents the time at which the certificate was revoked. It
	// is both used when creating a CRL and populated when parsing a CRL. It MUST
	// NOT be nil.
	RevocationTime time.Time
	// ReasonCode represents the reason for revocation, using the integer enum
	// values specified in RFC 5280 Section 5.3.1. When creating a CRL, a value of
	// nil or zero will result in the reasonCode extension being omitted. When
	// parsing a CRL, a value of nil represents a no reasonCode extension, while a
	// value of 0 represents a reasonCode extension containing enum value 0 (this
	// SHOULD NOT happen, but can and does).
	ReasonCode *int

	// Extensions contains raw X.509 extensions. When creating a CRL, the
	// Extensions field is ignored, see ExtraExtensions.
	Extensions []pkix.Extension
	// ExtraExtensions contains any additional extensions to add directly to the
	// revokedCertificate entry. It is up to the caller to ensure that this field
	// does not contain any extensions which duplicate extensions created by this
	// package (currently, the reasonCode extension). The ExtraExtensions field is
	// not populated when parsing a CRL, see Extensions.
	ExtraExtensions []pkix.Extension
}

// ENDBLOCK

// ParseRevocationList parses a X509 v2 Certificate Revocation List from the given
// ASN.1 DER data.
func ParseRevocationList(der []byte) (*RevocationList, error) {
	rl := &RevocationList{}

	input := cryptobyte.String(der)
	// we read the SEQUENCE including length and tag bytes so that
	// we can populate RevocationList.Raw, before unwrapping the
	// SEQUENCE so it can be operated on
	if !input.ReadASN1Element(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed crl")
	}
	rl.Raw = input
	if !input.ReadASN1(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed crl")
	}

	var tbs cryptobyte.String
	// do the same trick again as above to extract the raw
	// bytes for Certificate.RawTBSCertificate
	if !input.ReadASN1Element(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs crl")
	}
	rl.RawTBSRevocationList = tbs
	if !tbs.ReadASN1(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs crl")
	}

	var version int
	if !tbs.PeekASN1Tag(cryptobyte_asn1.INTEGER) {
		return nil, errors.New("x509: unsupported crl version")
	}
	if !tbs.ReadASN1Integer(&version) {
		return nil, errors.New("x509: malformed crl")
	}
	if version != x509v2Version {
		return nil, fmt.Errorf("x509: unsupported crl version: %d", version)
	}

	var sigAISeq cryptobyte.String
	if !tbs.ReadASN1(&sigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed signature algorithm identifier")
	}
	// Before parsing the inner algorithm identifier, extract
	// the outer algorithm identifier and make sure that they
	// match.
	var outerSigAISeq cryptobyte.String
	if !input.ReadASN1(&outerSigAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed algorithm identifier")
	}
	if !bytes.Equal(outerSigAISeq, sigAISeq) {
		return nil, errors.New("x509: inner and outer signature algorithm identifiers don't match")
	}
	sigAI, err := parseAI(sigAISeq)
	if err != nil {
		return nil, err
	}
	rl.SignatureAlgorithm = getSignatureAlgorithmFromAI(sigAI)

	var signature asn1.BitString
	if !input.ReadASN1BitString(&signature) {
		return nil, errors.New("x509: malformed signature")
	}
	rl.Signature = signature.RightAlign()

	var issuerSeq cryptobyte.String
	if !tbs.ReadASN1Element(&issuerSeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed issuer")
	}
	rl.RawIssuer = issuerSeq
	issuerRDNs, err := parseName(issuerSeq)
	if err != nil {
		return nil, err
	}
	rl.Issuer.FillFromRDNSequence(issuerRDNs)

	rl.ThisUpdate, err = parseTime(&tbs)
	if err != nil {
		return nil, err
	}
	if tbs.PeekASN1Tag(cryptobyte_asn1.GeneralizedTime) || tbs.PeekASN1Tag(cryptobyte_asn1.UTCTime) {
		rl.NextUpdate, err = parseTime(&tbs)
		if err != nil {
			return nil, err
		}
	}

	if tbs.PeekASN1Tag(cryptobyte_asn1.SEQUENCE) {
		// NOTE: The block does not exist in upstream.
		rcs := make([]RevokedCertificate, 0)
		// ENDBLOCK
		var revokedSeq cryptobyte.String
		if !tbs.ReadASN1(&revokedSeq, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("x509: malformed crl")
		}
		for !revokedSeq.Empty() {
			var certSeq cryptobyte.String
			// NOTE: The block is different from upstream. Upstream: ReadASN1
			if !revokedSeq.ReadASN1Element(&certSeq, cryptobyte_asn1.SEQUENCE) {
				// ENDBLOCK
				return nil, errors.New("x509: malformed crl")
			}
			rc := RevokedCertificate{Raw: certSeq}
			if !certSeq.ReadASN1(&certSeq, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed crl")
			}
			rc.SerialNumber = new(big.Int)
			if !certSeq.ReadASN1Integer(rc.SerialNumber) {
				return nil, errors.New("x509: malformed serial number")
			}
			rc.RevocationTime, err = parseTime(&certSeq)
			if err != nil {
				return nil, err
			}
			var extensions cryptobyte.String
			var present bool
			if !certSeq.ReadOptionalASN1(&extensions, &present, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed extensions")
			}
			if present {
				for !extensions.Empty() {
					var extension cryptobyte.String
					if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
						return nil, errors.New("x509: malformed extension")
					}
					ext, err := parseExtension(extension)
					if err != nil {
						return nil, err
					}
					// STARTBLOCK: This block does not exist in upstream.
					if ext.Id.Equal(oidExtensionReasonCode) {
						val := cryptobyte.String(ext.Value)
						rc.ReasonCode = new(int)
						if !val.ReadASN1Enum(rc.ReasonCode) {
							return nil, fmt.Errorf("x509: malformed reasonCode extension")
						}
					}
					// ENDBLOCK
					rc.Extensions = append(rc.Extensions, ext)
				}
			}
			// STARTBLOCK: The block does not exist in upstream.
			rcs = append(rcs, rc)
			// ENDBLOCK
		}
		rl.RevokedCertificates = rcs
	}

	var extensions cryptobyte.String
	var present bool
	if !tbs.ReadOptionalASN1(&extensions, &present, cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
		return nil, errors.New("x509: malformed extensions")
	}
	if present {
		if !extensions.ReadASN1(&extensions, cryptobyte_asn1.SEQUENCE) {
			return nil, errors.New("x509: malformed extensions")
		}
		for !extensions.Empty() {
			var extension cryptobyte.String
			if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: malformed extension")
			}
			ext, err := parseExtension(extension)
			if err != nil {
				return nil, err
			}
			if ext.Id.Equal(oidExtensionAuthorityKeyId) {
				rl.AuthorityKeyId = ext.Value
			} else if ext.Id.Equal(oidExtensionCRLNumber) {
				value := cryptobyte.String(ext.Value)
				rl.Number = new(big.Int)
				if !value.ReadASN1Integer(rl.Number) {
					return nil, errors.New("x509: malformed crl number")
				}
			}
			rl.Extensions = append(rl.Extensions, ext)
		}
	}

	return rl, nil
}

// isPrintable reports whether the given b is in the ASN.1 PrintableString set.
// This is a simplified version of encoding/asn1.isPrintable.
func isPrintable(b byte) bool {
	return 'a' <= b && b <= 'z' ||
		'A' <= b && b <= 'Z' ||
		'0' <= b && b <= '9' ||
		'\'' <= b && b <= ')' ||
		'+' <= b && b <= '/' ||
		b == ' ' ||
		b == ':' ||
		b == '=' ||
		b == '?' ||
		// This is technically not allowed in a PrintableString.
		// However, x509 certificates with wildcard strings don't
		// always use the correct string type so we permit it.
		b == '*' ||
		// This is not technically allowed either. However, not
		// only is it relatively common, but there are also a
		// handful of CA certificates that contain it. At least
		// one of which will not expire until 2027.
		b == '&'
}

// parseASN1String parses the ASN.1 string types T61String, PrintableString,
// UTF8String, BMPString, IA5String, and NumericString. This is mostly copied
// from the respective encoding/asn1.parse... methods, rather than just
// increasing the API surface of that package.
func parseASN1String(tag cryptobyte_asn1.Tag, value []byte) (string, error) {
	switch tag {
	case cryptobyte_asn1.T61String:
		return string(value), nil
	case cryptobyte_asn1.PrintableString:
		for _, b := range value {
			if !isPrintable(b) {
				return "", errors.New("invalid PrintableString")
			}
		}
		return string(value), nil
	case cryptobyte_asn1.UTF8String:
		if !utf8.Valid(value) {
			return "", errors.New("invalid UTF-8 string")
		}
		return string(value), nil
	case cryptobyte_asn1.Tag(asn1.TagBMPString):
		if len(value)%2 != 0 {
			return "", errors.New("invalid BMPString")
		}

		// Strip terminator if present.
		if l := len(value); l >= 2 && value[l-1] == 0 && value[l-2] == 0 {
			value = value[:l-2]
		}

		s := make([]uint16, 0, len(value)/2)
		for len(value) > 0 {
			s = append(s, uint16(value[0])<<8+uint16(value[1]))
			value = value[2:]
		}

		return string(utf16.Decode(s)), nil
	case cryptobyte_asn1.IA5String:
		s := string(value)
		if isIA5String(s) != nil {
			return "", errors.New("invalid IA5String")
		}
		return s, nil
	case cryptobyte_asn1.Tag(asn1.TagNumericString):
		for _, b := range value {
			if !('0' <= b && b <= '9' || b == ' ') {
				return "", errors.New("invalid NumericString")
			}
		}
		return string(value), nil
	}
	return "", fmt.Errorf("unsupported string type: %v", tag)
}

// parseName parses a DER encoded Name as defined in RFC 5280. We may
// want to export this function in the future for use in crypto/tls.
func parseName(raw cryptobyte.String) (*pkix.RDNSequence, error) {
	if !raw.ReadASN1(&raw, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: invalid RDNSequence")
	}

	var rdnSeq pkix.RDNSequence
	for !raw.Empty() {
		var rdnSet pkix.RelativeDistinguishedNameSET
		var set cryptobyte.String
		if !raw.ReadASN1(&set, cryptobyte_asn1.SET) {
			return nil, errors.New("x509: invalid RDNSequence")
		}
		for !set.Empty() {
			var atav cryptobyte.String
			if !set.ReadASN1(&atav, cryptobyte_asn1.SEQUENCE) {
				return nil, errors.New("x509: invalid RDNSequence: invalid attribute")
			}
			var attr pkix.AttributeTypeAndValue
			if !atav.ReadASN1ObjectIdentifier(&attr.Type) {
				return nil, errors.New("x509: invalid RDNSequence: invalid attribute type")
			}
			var rawValue cryptobyte.String
			var valueTag cryptobyte_asn1.Tag
			if !atav.ReadAnyASN1(&rawValue, &valueTag) {
				return nil, errors.New("x509: invalid RDNSequence: invalid attribute value")
			}
			var err error
			attr.Value, err = parseASN1String(valueTag, rawValue)
			if err != nil {
				return nil, fmt.Errorf("x509: invalid RDNSequence: invalid attribute value: %s", err)
			}
			rdnSet = append(rdnSet, attr)
		}

		rdnSeq = append(rdnSeq, rdnSet)
	}

	return &rdnSeq, nil
}

func parseAI(der cryptobyte.String) (pkix.AlgorithmIdentifier, error) {
	ai := pkix.AlgorithmIdentifier{}
	if !der.ReadASN1ObjectIdentifier(&ai.Algorithm) {
		return ai, errors.New("x509: malformed OID")
	}
	if der.Empty() {
		return ai, nil
	}
	var params cryptobyte.String
	var tag cryptobyte_asn1.Tag
	if !der.ReadAnyASN1Element(&params, &tag) {
		return ai, errors.New("x509: malformed parameters")
	}
	ai.Parameters.Tag = int(tag)
	ai.Parameters.FullBytes = params
	return ai, nil
}

func parseTime(der *cryptobyte.String) (time.Time, error) {
	var t time.Time
	switch {
	case der.PeekASN1Tag(cryptobyte_asn1.UTCTime):
		if !der.ReadASN1UTCTime(&t) {
			return t, errors.New("x509: malformed UTCTime")
		}
	case der.PeekASN1Tag(cryptobyte_asn1.GeneralizedTime):
		if !der.ReadASN1GeneralizedTime(&t) {
			return t, errors.New("x509: malformed GeneralizedTime")
		}
	default:
		return t, errors.New("x509: unsupported time format")
	}
	return t, nil
}

func parseExtension(der cryptobyte.String) (pkix.Extension, error) {
	var ext pkix.Extension
	if !der.ReadASN1ObjectIdentifier(&ext.Id) {
		return ext, errors.New("x509: malformed extension OID field")
	}
	if der.PeekASN1Tag(cryptobyte_asn1.BOOLEAN) {
		if !der.ReadASN1Boolean(&ext.Critical) {
			return ext, errors.New("x509: malformed extension critical field")
		}
	}
	var val cryptobyte.String
	if !der.ReadASN1(&val, cryptobyte_asn1.OCTET_STRING) {
		return ext, errors.New("x509: malformed extension value field")
	}
	ext.Value = val
	return ext, nil
}
