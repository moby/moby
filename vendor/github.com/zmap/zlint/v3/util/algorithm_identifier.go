package util

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/zmap/zcrypto/x509"
	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

// additional OIDs not provided by the x509 package.
var (
	// 1.2.840.10045.4.3.1 is SHA224withECDSA
	OidSignatureSHA224withECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 1}
)

// RSAAlgorithmIDToDER contains DER representations of pkix.AlgorithmIdentifier for different RSA OIDs with Parameters as asn1.NULL.
var RSAAlgorithmIDToDER = map[string][]byte{
	// rsaEncryption
	"1.2.840.113549.1.1.1": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0x1, 0x5, 0x0},
	// md2WithRSAEncryption
	"1.2.840.113549.1.1.2": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0x2, 0x5, 0x0},
	// md5WithRSAEncryption
	"1.2.840.113549.1.1.4": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0x4, 0x5, 0x0},
	// sha-1WithRSAEncryption
	"1.2.840.113549.1.1.5": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0x5, 0x5, 0x0},
	// sha224WithRSAEncryption
	"1.2.840.113549.1.1.14": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0xe, 0x5, 0x0},
	// sha256WithRSAEncryption
	"1.2.840.113549.1.1.11": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0xb, 0x5, 0x0},
	// sha384WithRSAEncryption
	"1.2.840.113549.1.1.12": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0xc, 0x5, 0x0},
	// sha512WithRSAEncryption
	"1.2.840.113549.1.1.13": {0x30, 0x0d, 0x6, 0x9, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0xd, 0x1, 0x1, 0xd, 0x5, 0x0},
}

// CheckAlgorithmIDParamNotNULL parses an AlgorithmIdentifier with algorithm OID rsaEncryption to check the Param field is asn1.NULL
// Expects DER-encoded AlgorithmIdentifier including tag and length.
func CheckAlgorithmIDParamNotNULL(algorithmIdentifier []byte, requiredAlgoID asn1.ObjectIdentifier) error {
	expectedAlgoIDBytes, ok := RSAAlgorithmIDToDER[requiredAlgoID.String()]
	if !ok {
		return errors.New("error algorithmID to check is not RSA")
	}

	algorithmSequence := cryptobyte.String(algorithmIdentifier)

	// byte comparison of algorithm sequence and checking no trailing data is present
	var algorithmBytes []byte
	if algorithmSequence.ReadBytes(&algorithmBytes, len(expectedAlgoIDBytes)) {
		if bytes.Equal(algorithmBytes, expectedAlgoIDBytes) && algorithmSequence.Empty() {
			return nil
		}
	}

	// re-parse to get an error message detailing what did not match in the byte comparison
	algorithmSequence = cryptobyte.String(algorithmIdentifier)
	var algorithm cryptobyte.String
	if !algorithmSequence.ReadASN1(&algorithm, cryptobyte_asn1.SEQUENCE) {
		return errors.New("error reading algorithm")
	}

	encryptionOID := asn1.ObjectIdentifier{}
	if !algorithm.ReadASN1ObjectIdentifier(&encryptionOID) {
		return errors.New("error reading algorithm OID")
	}

	if !encryptionOID.Equal(requiredAlgoID) {
		return fmt.Errorf("algorithm OID is not equal to %s", requiredAlgoID.String())
	}

	if algorithm.Empty() {
		return errors.New("RSA algorithm identifier missing required NULL parameter")
	}

	var nullValue cryptobyte.String
	if !algorithm.ReadASN1(&nullValue, cryptobyte_asn1.NULL) {
		return errors.New("RSA algorithm identifier with non-NULL parameter")
	}

	if len(nullValue) != 0 {
		return errors.New("RSA algorithm identifier with NULL parameter containing data")
	}

	// ensure algorithm is empty and no trailing data is present
	if !algorithm.Empty() {
		return errors.New("RSA algorithm identifier with trailing data")
	}

	return errors.New("RSA algorithm appears correct, but didn't match byte-wise comparison")
}

// Returns the signature field of the tbsCertificate of this certificate in a DER encoded form or an error
// if the signature field could not be extracted. The encoded form contains the tag and the length.
//
//    TBSCertificate  ::=  SEQUENCE  {
//        version         [0]  EXPLICIT Version DEFAULT v1,
//        serialNumber         CertificateSerialNumber,
//        signature            AlgorithmIdentifier,
//        issuer               Name,
//        validity             Validity,
//        subject              Name,
//        subjectPublicKeyInfo SubjectPublicKeyInfo,
//        issuerUniqueID  [1]  IMPLICIT UniqueIdentifier OPTIONAL,
//                             -- If present, version MUST be v2 or v3
//        subjectUniqueID [2]  IMPLICIT UniqueIdentifier OPTIONAL,
//                             -- If present, version MUST be v2 or v3
//        extensions      [3]  EXPLICIT Extensions OPTIONAL
//                             -- If present, version MUST be v3
//        }
func GetSignatureAlgorithmInTBSEncoded(c *x509.Certificate) ([]byte, error) {
	input := cryptobyte.String(c.RawTBSCertificate)

	var tbsCert cryptobyte.String
	if !input.ReadASN1(&tbsCert, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("error reading tbsCertificate")
	}

	if !tbsCert.SkipOptionalASN1(cryptobyte_asn1.Tag(0).Constructed().ContextSpecific()) {
		return nil, errors.New("error reading tbsCertificate.version")
	}

	if !tbsCert.SkipASN1(cryptobyte_asn1.INTEGER) {
		return nil, errors.New("error reading tbsCertificate.serialNumber")
	}

	var signatureAlgoID cryptobyte.String
	var tag cryptobyte_asn1.Tag
	// use ReadAnyElement to preserve tag and length octets
	if !tbsCert.ReadAnyASN1Element(&signatureAlgoID, &tag) {
		return nil, errors.New("error reading tbsCertificate.signature")
	}

	return signatureAlgoID, nil
}

// Returns the algorithm field of the SubjectPublicKeyInfo of the certificate or an error
// if the algorithm field could not be extracted.
//
//    SubjectPublicKeyInfo  ::=  SEQUENCE  {
//        algorithm            AlgorithmIdentifier,
//        subjectPublicKey     BIT STRING  }
//
func GetPublicKeyOID(c *x509.Certificate) (asn1.ObjectIdentifier, error) {
	input := cryptobyte.String(c.RawSubjectPublicKeyInfo)

	var publicKeyInfo cryptobyte.String
	if !input.ReadASN1(&publicKeyInfo, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("error reading pkixPublicKey")
	}

	var algorithm cryptobyte.String
	if !publicKeyInfo.ReadASN1(&algorithm, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("error reading public key algorithm identifier")
	}

	publicKeyOID := asn1.ObjectIdentifier{}
	if !algorithm.ReadASN1ObjectIdentifier(&publicKeyOID) {
		return nil, errors.New("error reading public key OID")
	}

	return publicKeyOID, nil
}

// Returns the algorithm field of the SubjectPublicKeyInfo of the certificate in its encoded form (containing Tag
// and Length) or an error if the algorithm field could not be extracted.
//
//    SubjectPublicKeyInfo  ::=  SEQUENCE  {
//        algorithm            AlgorithmIdentifier,
//        subjectPublicKey     BIT STRING  }
//
func GetPublicKeyAidEncoded(c *x509.Certificate) ([]byte, error) {
	input := cryptobyte.String(c.RawSubjectPublicKeyInfo)
	var spkiContent cryptobyte.String

	if !input.ReadASN1(&spkiContent, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("error reading pkixPublicKey")
	}

	var algorithm cryptobyte.String
	var tag cryptobyte_asn1.Tag

	if !spkiContent.ReadAnyASN1Element(&algorithm, &tag) {
		return nil, errors.New("error reading public key algorithm identifier")
	}

	return algorithm, nil
}
