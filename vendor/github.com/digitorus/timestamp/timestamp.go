// Package timestamp implements the Time-Stamp Protocol (TSP) as specified in
// RFC3161 (Internet X.509 Public Key Infrastructure Time-Stamp Protocol (TSP)).
package timestamp

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/digitorus/pkcs7"
)

// FailureInfo contains the failure details of an Time-Stamp request. See
// https://tools.ietf.org/html/rfc3161#section-2.4.2
type FailureInfo int

const (
	// UnknownFailureInfo mean that no known failure info was provided
	UnknownFailureInfo FailureInfo = -1
	// BadAlgorithm defines an unrecognized or unsupported Algorithm Identifier
	BadAlgorithm FailureInfo = 0
	// BadRequest indicates that the transaction not permitted or supported
	BadRequest FailureInfo = 2
	// BadDataFormat means tha data submitted has the wrong format
	BadDataFormat FailureInfo = 5
	// TimeNotAvailable indicates that TSA's time source is not available
	TimeNotAvailable FailureInfo = 14
	// UnacceptedPolicy indicates that the requested TSA policy is not supported
	// by the TSA
	UnacceptedPolicy FailureInfo = 15
	// UnacceptedExtension indicates that the requested extension is not supported
	// by the TSA
	UnacceptedExtension FailureInfo = 16
	// AddInfoNotAvailable means that the information requested could not be
	// understood or is not available
	AddInfoNotAvailable FailureInfo = 17
	// SystemFailure indicates that the request cannot be handled due to system
	// failure
	SystemFailure FailureInfo = 25
)

func (f FailureInfo) String() string {
	switch f {
	case BadAlgorithm:
		return "unrecognized or unsupported Algorithm Identifier"
	case BadRequest:
		return "transaction not permitted or supported"
	case BadDataFormat:
		return "the data submitted has the wrong format"
	case TimeNotAvailable:
		return "the TSA's time source is not available"
	case UnacceptedPolicy:
		return "the requested TSA policy is not supported by the TSA"
	case UnacceptedExtension:
		return "the requested extension is not supported by the TSA"
	case AddInfoNotAvailable:
		return "the additional information requested could not be understood or is not available"
	case SystemFailure:
		return "the request cannot be handled due to system failure"
	default:
		return "unknown failure"
	}
}

// Status contains the status of an Time-Stamp request. See
// https://tools.ietf.org/html/rfc3161#section-2.4.2
type Status int

const (
	// Granted PKIStatus contains the value zero a TimeStampToken, as requested,
	// is present.
	Granted Status = 0
	// GrantedWithMods PKIStatus contains the value one a TimeStampToken, with
	// modifications, is present.
	GrantedWithMods Status = 1
	// Rejection PKIStatus
	Rejection Status = 2
	// Waiting PKIStatus
	Waiting Status = 3
	// RevocationWarning PKIStatus
	RevocationWarning Status = 4
	// RevocationNotification PKIStatus
	RevocationNotification Status = 5
)

func (s Status) String() string {
	switch s {
	case Granted:
		return "the request is granted"
	case GrantedWithMods:
		return "the request is granted with modifications"
	case Rejection:
		return "the request is rejected"
	case Waiting:
		return "the request is waiting"
	case RevocationWarning:
		return "revocation is imminent"
	case RevocationNotification:
		return "revocation has occurred"
	default:
		return "unknown status: " + strconv.Itoa(int(s))
	}
}

// ParseError results from an invalid Time-Stamp request or response.
type ParseError string

func (p ParseError) Error() string {
	return string(p)
}

// Request represents an Time-Stamp request. See
// https://tools.ietf.org/html/rfc3161#section-2.4.1
type Request struct {
	HashAlgorithm crypto.Hash
	HashedMessage []byte

	// Certificates indicates if the TSA needs to return the signing certificate
	// and optionally any other certificates of the chain as part of the response.
	Certificates bool

	// The TSAPolicyOID field, if provided, indicates the TSA policy under
	// which the TimeStampToken SHOULD be provided
	TSAPolicyOID asn1.ObjectIdentifier

	// The nonce, if provided, allows the client to verify the timeliness of
	// the response.
	Nonce *big.Int

	// Extensions contains raw X.509 extensions from the Extensions field of the
	// Time-Stamp request. When parsing requests, this can be used to extract
	// non-critical extensions that are not parsed by this package. When
	// marshaling OCSP requests, the Extensions field is ignored, see
	// ExtraExtensions.
	Extensions []pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any marshaled
	// OCSP response (in the singleExtensions field). Values override any
	// extensions that would otherwise be produced based on the other fields. The
	// ExtraExtensions field is not populated when parsing Time-Stamp requests,
	// see Extensions.
	ExtraExtensions []pkix.Extension
}

// ParseRequest parses an timestamp request in DER form.
func ParseRequest(bytes []byte) (*Request, error) {
	var err error
	var rest []byte
	var req request

	if rest, err = asn1.Unmarshal(bytes, &req); err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, ParseError("trailing data in Time-Stamp request")
	}

	if len(req.MessageImprint.HashedMessage) == 0 {
		return nil, ParseError("Time-Stamp request contains no hashed message")
	}

	hashFunc := getHashAlgorithmFromOID(req.MessageImprint.HashAlgorithm.Algorithm)
	if hashFunc == crypto.Hash(0) {
		return nil, ParseError("Time-Stamp request uses unknown hash function")
	}

	return &Request{
		HashAlgorithm: hashFunc,
		HashedMessage: req.MessageImprint.HashedMessage,
		Certificates:  req.CertReq,
		Nonce:         req.Nonce,
		TSAPolicyOID:  req.ReqPolicy,
		Extensions:    req.Extensions,
	}, nil
}

// Marshal marshals the Time-Stamp request to ASN.1 DER encoded form.
func (req *Request) Marshal() ([]byte, error) {
	request := request{
		Version: 1,
		MessageImprint: messageImprint{
			HashAlgorithm: pkix.AlgorithmIdentifier{
				Algorithm: getOIDFromHashAlgorithm(req.HashAlgorithm),
				Parameters: asn1.RawValue{
					Tag: 5, /* ASN.1 NULL */
				},
			},
			HashedMessage: req.HashedMessage,
		},
		CertReq:    req.Certificates,
		Extensions: req.ExtraExtensions,
	}

	if req.TSAPolicyOID != nil {
		request.ReqPolicy = req.TSAPolicyOID
	}
	if req.Nonce != nil {
		request.Nonce = req.Nonce
	}
	reqBytes, err := asn1.Marshal(request)
	if err != nil {
		return nil, err
	}
	return reqBytes, nil
}

// Timestamp represents an Time-Stamp. See:
// https://tools.ietf.org/html/rfc3161#section-2.4.1
type Timestamp struct {
	// Timestamp token part of raw ASN.1 DER content.
	RawToken []byte

	HashAlgorithm crypto.Hash
	HashedMessage []byte

	Time         time.Time
	Accuracy     time.Duration
	SerialNumber *big.Int
	Policy       asn1.ObjectIdentifier
	Ordering     bool
	Nonce        *big.Int
	Qualified    bool

	Certificates []*x509.Certificate

	// If set to true, includes TSA certificate in timestamp response
	AddTSACertificate bool

	// Extensions contains raw X.509 extensions from the Extensions field of the
	// Time-Stamp. When parsing time-stamps, this can be used to extract
	// non-critical extensions that are not parsed by this package. When
	// marshaling time-stamps, the Extensions field is ignored, see
	// ExtraExtensions.
	Extensions []pkix.Extension

	// ExtraExtensions contains extensions to be copied, raw, into any marshaled
	// Time-Stamp response. Values override any extensions that would otherwise
	// be produced based on the other fields. The ExtraExtensions field is not
	// populated when parsing Time-Stamp responses, see Extensions.
	ExtraExtensions []pkix.Extension
}

// ParseResponse parses an Time-Stamp response in DER form containing a
// TimeStampToken.
//
// Invalid signatures or parse failures will result in a ParseError. Error
// responses will result in a ResponseError.
func ParseResponse(bytes []byte) (*Timestamp, error) {
	var err error
	var rest []byte
	var resp response

	if rest, err = asn1.Unmarshal(bytes, &resp); err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, ParseError("trailing data in Time-Stamp response")
	}

	if resp.Status.Status > 0 {
		var fis string
		fi := resp.Status.FailureInfo()
		if fi != UnknownFailureInfo {
			fis = fi.String()
		}
		return nil, fmt.Errorf("%s: %s (%v)",
			resp.Status.Status.String(),
			strings.Join(resp.Status.StatusString, ","),
			fis)
	}

	if len(resp.TimeStampToken.Bytes) == 0 {
		return nil, ParseError("no pkcs7 data in Time-Stamp response")
	}

	return Parse(resp.TimeStampToken.FullBytes)
}

// Parse parses an Time-Stamp in DER form. If the time-stamp contains a
// certificate then the signature over the response is checked.
//
// Invalid signatures or parse failures will result in a ParseError. Error
// responses will result in a ResponseError.
func Parse(bytes []byte) (*Timestamp, error) {
	var addTSACertificate bool
	p7, err := pkcs7.Parse(bytes)
	if err != nil {
		return nil, err
	}

	if len(p7.Certificates) > 0 {
		if err = p7.Verify(); err != nil {
			return nil, err
		}
		addTSACertificate = true
	} else {
		addTSACertificate = false
	}

	var inf tstInfo
	if _, err = asn1.Unmarshal(p7.Content, &inf); err != nil {
		return nil, err
	}

	if len(inf.MessageImprint.HashedMessage) == 0 {
		return nil, ParseError("Time-Stamp response contains no hashed message")
	}

	ret := &Timestamp{
		RawToken:      bytes,
		HashedMessage: inf.MessageImprint.HashedMessage,
		Time:          inf.Time,
		Accuracy: time.Duration((time.Second * time.Duration(inf.Accuracy.Seconds)) +
			(time.Millisecond * time.Duration(inf.Accuracy.Milliseconds)) +
			(time.Microsecond * time.Duration(inf.Accuracy.Microseconds))),
		SerialNumber:      inf.SerialNumber,
		Policy:            inf.Policy,
		Ordering:          inf.Ordering,
		Nonce:             inf.Nonce,
		Certificates:      p7.Certificates,
		AddTSACertificate: addTSACertificate,
		Extensions:        inf.Extensions,
	}

	ret.HashAlgorithm = getHashAlgorithmFromOID(inf.MessageImprint.HashAlgorithm.Algorithm)
	if ret.HashAlgorithm == crypto.Hash(0) {
		return nil, ParseError("Time-Stamp response uses unknown hash function")
	}

	if oidInExtensions(asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}, inf.Extensions) {
		ret.Qualified = true
	}
	return ret, nil
}

// RequestOptions contains options for constructing timestamp requests.
type RequestOptions struct {
	// Hash contains the hash function that should be used when
	// constructing the timestamp request. If zero, SHA-256 will be used.
	Hash crypto.Hash

	// Certificates sets Request.Certificates
	Certificates bool

	// The TSAPolicyOID field, if provided, indicates the TSA policy under
	// which the TimeStampToken SHOULD be provided
	TSAPolicyOID asn1.ObjectIdentifier

	// The nonce, if provided, allows the client to verify the timeliness of
	// the response.
	Nonce *big.Int
}

func (opts *RequestOptions) hash() crypto.Hash {
	if opts == nil || opts.Hash == 0 {
		return crypto.SHA256
	}
	return opts.Hash
}

// CreateRequest returns a DER-encoded, timestamp request for the status of cert. If
// opts is nil then sensible defaults are used.
func CreateRequest(r io.Reader, opts *RequestOptions) ([]byte, error) {
	hashFunc := opts.hash()

	if !hashFunc.Available() {
		return nil, x509.ErrUnsupportedAlgorithm
	}
	h := opts.hash().New()

	b := make([]byte, h.Size())
	for {
		n, err := r.Read(b)
		if err == io.EOF {
			break
		}

		_, err = h.Write(b[:n])
		if err != nil {
			return nil, fmt.Errorf("failed to create hash")
		}
	}

	req := &Request{
		HashAlgorithm: opts.hash(),
		HashedMessage: h.Sum(nil),
	}
	if opts != nil {
		req.Certificates = opts.Certificates
	}
	if opts != nil && opts.TSAPolicyOID != nil {
		req.TSAPolicyOID = opts.TSAPolicyOID
	}
	if opts != nil && opts.Nonce != nil {
		req.Nonce = opts.Nonce
	}
	return req.Marshal()
}

// CreateResponseWithOpts returns a DER-encoded timestamp response with the specified contents.
// The fields in the response are populated as follows:
//
// The responder cert is used to populate the responder's name field, and the
// certificate itself is provided alongside the timestamp response signature.
func (t *Timestamp) CreateResponseWithOpts(signingCert *x509.Certificate, priv crypto.Signer, opts crypto.SignerOpts) ([]byte, error) {
	messageImprint := getMessageImprint(t.HashAlgorithm, t.HashedMessage)

	tsaSerialNumber, err := generateTSASerialNumber()
	if err != nil {
		return nil, err
	}
	tstInfo, err := t.populateTSTInfo(messageImprint, t.Policy, tsaSerialNumber, signingCert)
	if err != nil {
		return nil, err
	}
	signature, err := t.generateSignedData(tstInfo, priv, signingCert, opts)
	if err != nil {
		return nil, err
	}
	timestampRes := response{
		Status: pkiStatusInfo{
			Status: Granted,
		},
		TimeStampToken: asn1.RawValue{FullBytes: signature},
	}
	tspResponseBytes, err := asn1.Marshal(timestampRes)
	if err != nil {
		return nil, err
	}
	return tspResponseBytes, nil
}

// CreateResponse returns a DER-encoded timestamp response with the specified contents.
// The fields in the response are populated as follows:
//
// The responder cert is used to populate the responder's name field, and the
// certificate itself is provided alongside the timestamp response signature.
//
// This function is equivalent to CreateResponseWithOpts, using a SHA256 hash.
//
// Deprecated: Use CreateResponseWithOpts instead.
func (t *Timestamp) CreateResponse(signingCert *x509.Certificate, priv crypto.Signer) ([]byte, error) {
	return t.CreateResponseWithOpts(signingCert, priv, crypto.SHA256)
}

// CreateErrorResponse is used to create response other than granted and granted with mod status
func CreateErrorResponse(pkiStatus Status, pkiFailureInfo FailureInfo) ([]byte, error) {
	var bs asn1.BitString
	setFlag(&bs, int(pkiFailureInfo))

	timestampRes := response{
		Status: pkiStatusInfo{
			Status:   pkiStatus,
			FailInfo: bs,
		},
	}
	tspResponseBytes, err := asn1.Marshal(timestampRes)
	if err != nil {
		return nil, err
	}
	return tspResponseBytes, nil
}

func setFlag(bs *asn1.BitString, i int) {
	for l := len(bs.Bytes); l < 4; l++ {
		(*bs).Bytes = append((*bs).Bytes, byte(0))
		(*bs).BitLength = len((*bs).Bytes) * 8
	}
	b := i / 8
	p := uint(7 - (i - 8*b))
	(*bs).Bytes[b] = (*bs).Bytes[b] | (1 << p)
	bs.BitLength = asn1BitLength(bs.Bytes)
	bs.Bytes = bs.Bytes[0 : (bs.BitLength/8)+1]
}

func getMessageImprint(hashAlgorithm crypto.Hash, hashedMessage []byte) messageImprint {
	messageImprint := messageImprint{
		HashAlgorithm: pkix.AlgorithmIdentifier{
			Algorithm:  getOIDFromHashAlgorithm(hashAlgorithm),
			Parameters: asn1.NullRawValue,
		},
		HashedMessage: hashedMessage,
	}
	return messageImprint
}

func generateTSASerialNumber() (*big.Int, error) {
	randomBytes := make([]byte, 20)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}
	serialNumber := big.NewInt(0)
	serialNumber = serialNumber.SetBytes(randomBytes)
	return serialNumber, nil
}

func (t *Timestamp) populateTSTInfo(messageImprint messageImprint, policyOID asn1.ObjectIdentifier, tsaSerialNumber *big.Int, tsaCert *x509.Certificate) ([]byte, error) {
	dirGeneralName, err := asn1.Marshal(asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: tsaCert.RawSubject})
	if err != nil {
		return nil, err
	}
	tstInfo := tstInfo{
		Version:        1,
		Policy:         policyOID,
		MessageImprint: messageImprint,
		SerialNumber:   tsaSerialNumber,
		Time:           t.Time,
		TSA:            asn1.RawValue{Tag: 0, Class: 2, IsCompound: true, Bytes: dirGeneralName},
		Ordering:       t.Ordering,
	}
	if t.Nonce != nil {
		tstInfo.Nonce = t.Nonce
	}
	if t.Accuracy != 0 {
		if t.Accuracy < time.Microsecond {
			// Round up to 1 microsecond if accuracy is lower than 1 microsecond but greater than 0 nanosecond
			tstInfo.Accuracy.Microseconds = 1
		} else {
			seconds := t.Accuracy.Truncate(time.Second)
			tstInfo.Accuracy.Seconds = int64(seconds.Seconds())
			ms := (t.Accuracy - seconds).Truncate(time.Millisecond)
			if ms != 0 {
				tstInfo.Accuracy.Milliseconds = int64(ms.Milliseconds())
			}
			microSeconds := (t.Accuracy - seconds - ms).Truncate(time.Microsecond)
			if microSeconds != 0 {
				tstInfo.Accuracy.Microseconds = int64(microSeconds.Microseconds())
			}
		}
	}
	if len(t.ExtraExtensions) != 0 {
		tstInfo.Extensions = t.ExtraExtensions
	}
	if t.Qualified && !oidInExtensions(asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}, t.ExtraExtensions) {
		qcStatements := []qcStatement{{
			StatementID: asn1.ObjectIdentifier{0, 4, 0, 19422, 1, 1},
		}}
		asn1QcStats, err := asn1.Marshal(qcStatements)
		if err != nil {
			return nil, err
		}
		tstInfo.Extensions = append(tstInfo.Extensions, pkix.Extension{
			Id:       []int{1, 3, 6, 1, 5, 5, 7, 1, 3},
			Value:    asn1QcStats,
			Critical: false,
		})
	}
	tstInfoBytes, err := asn1.Marshal(tstInfo)
	if err != nil {
		return nil, err
	}
	return tstInfoBytes, nil
}

func (t *Timestamp) populateSigningCertificateV2Ext(certificate *x509.Certificate) ([]byte, error) {
	if !t.HashAlgorithm.Available() {
		return nil, x509.ErrUnsupportedAlgorithm
	}
	if t.HashAlgorithm.HashFunc() == crypto.SHA1 {
		return nil, fmt.Errorf("for SHA1 use ESSCertID instead of ESSCertIDv2")
	}

	h := t.HashAlgorithm.HashFunc().New()
	_, err := h.Write(certificate.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash")
	}

	var hashAlg pkix.AlgorithmIdentifier

	// HashAlgorithm defaults to SHA256
	if t.HashAlgorithm.HashFunc() != crypto.SHA256 {
		hashAlg = pkix.AlgorithmIdentifier{
			Algorithm:  hashOIDs[t.HashAlgorithm.HashFunc()],
			Parameters: asn1.NullRawValue,
		}
	}

	signingCertificateV2 := signingCertificateV2{
		Certs: []essCertIDv2{{
			HashAlgorithm: hashAlg,
			CertHash:      h.Sum(nil),
			IssuerSerial: issuerAndSerial{
				IssuerName: generalNames{
					Name: asn1.RawValue{Tag: 4, Class: 2, IsCompound: true, Bytes: certificate.RawIssuer},
				},
				SerialNumber: certificate.SerialNumber,
			},
		}},
	}
	signingCertV2Bytes, err := asn1.Marshal(signingCertificateV2)
	if err != nil {
		return nil, err
	}
	return signingCertV2Bytes, nil
}

// digestAlgorithmToOID converts the hash func to the corresponding OID.
// This should have parity with [pkcs7.getHashForOID].
func digestAlgorithmToOID(hash crypto.Hash) (asn1.ObjectIdentifier, error) {
	switch hash {
	case crypto.SHA1:
		return pkcs7.OIDDigestAlgorithmSHA1, nil
	case crypto.SHA256:
		return pkcs7.OIDDigestAlgorithmSHA256, nil
	case crypto.SHA384:
		return pkcs7.OIDDigestAlgorithmSHA384, nil
	case crypto.SHA512:
		return pkcs7.OIDDigestAlgorithmSHA512, nil
	}
	return nil, pkcs7.ErrUnsupportedAlgorithm
}

func (t *Timestamp) generateSignedData(tstInfo []byte, signer crypto.Signer, certificate *x509.Certificate, opts crypto.SignerOpts) ([]byte, error) {
	signedData, err := pkcs7.NewSignedData(tstInfo)
	if err != nil {
		return nil, err
	}

	alg, err := digestAlgorithmToOID(opts.HashFunc())
	if err != nil {
		return nil, err
	}
	signedData.SetDigestAlgorithm(alg)
	signedData.SetContentType(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4})
	signedData.GetSignedData().Version = 3

	signingCertV2Bytes, err := t.populateSigningCertificateV2Ext(certificate)
	if err != nil {
		return nil, err
	}

	signerInfoConfig := pkcs7.SignerInfoConfig{
		ExtraSignedAttributes: []pkcs7.Attribute{
			{
				Type:  asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47},
				Value: asn1.RawValue{FullBytes: signingCertV2Bytes},
			},
		},
	}
	if !t.AddTSACertificate {
		signerInfoConfig.SkipCertificates = true
	}

	if len(t.Certificates) > 0 {
		err = signedData.AddSignerChain(certificate, signer, t.Certificates, signerInfoConfig)
	} else {
		err = signedData.AddSigner(certificate, signer, signerInfoConfig)
	}
	if err != nil {
		return nil, err
	}

	signature, err := signedData.Finish()
	if err != nil {
		return nil, err
	}
	return signature, nil
}

// copied from crypto/x509 package
// oidNotInExtensions reports whether an extension with the given oid exists in
// extensions.
func oidInExtensions(oid asn1.ObjectIdentifier, extensions []pkix.Extension) bool {
	for _, e := range extensions {
		if e.Id.Equal(oid) {
			return true
		}
	}
	return false
}
