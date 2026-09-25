package timestamp

import (
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"
)

// http://www.ietf.org/rfc/rfc3161.txt
// 2.4.1. Request Format
type request struct {
	Version        int
	MessageImprint messageImprint
	ReqPolicy      asn1.ObjectIdentifier `asn1:"optional"`
	Nonce          *big.Int              `asn1:"optional"`
	CertReq        bool                  `asn1:"optional,default:false"`
	Extensions     []pkix.Extension      `asn1:"tag:0,optional"`
}

type messageImprint struct {
	HashAlgorithm pkix.AlgorithmIdentifier
	HashedMessage []byte
}

// 2.4.2. Response Format
type response struct {
	Status         pkiStatusInfo
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

type pkiStatusInfo struct {
	Status       Status
	StatusString []string       `asn1:"optional,utf8"`
	FailInfo     asn1.BitString `asn1:"optional"`
}

func (s pkiStatusInfo) FailureInfo() FailureInfo {
	fi := []FailureInfo{BadAlgorithm, BadRequest, BadDataFormat, TimeNotAvailable,
		UnacceptedPolicy, UnacceptedExtension, AddInfoNotAvailable, SystemFailure}

	for _, f := range fi {
		if s.FailInfo.At(int(f)) != 0 {
			return f
		}
	}

	return UnknownFailureInfo
}

// eContent within SignedData is TSTInfo
type tstInfo struct {
	Version        int
	Policy         asn1.ObjectIdentifier
	MessageImprint messageImprint
	SerialNumber   *big.Int
	Time           time.Time        `asn1:"generalized"`
	Accuracy       accuracy         `asn1:"optional"`
	Ordering       bool             `asn1:"optional,default:false"`
	Nonce          *big.Int         `asn1:"optional"`
	TSA            asn1.RawValue    `asn1:"tag:0,optional"`
	Extensions     []pkix.Extension `asn1:"tag:1,optional"`
}

// accuracy within TSTInfo
type accuracy struct {
	Seconds      int64 `asn1:"optional"`
	Milliseconds int64 `asn1:"tag:0,optional"`
	Microseconds int64 `asn1:"tag:1,optional"`
}

type qcStatement struct {
	StatementID   asn1.ObjectIdentifier
	StatementInfo asn1.RawValue `asn1:"optional"`
}
