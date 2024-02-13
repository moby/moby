// Copyright 2014 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package client is a CT log client implementation and contains types and code
// for interacting with RFC6962-compliant CT Log instances.
// See http://tools.ietf.org/html/rfc6962 for details
package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/tls"
)

// LogClient represents a client for a given CT Log instance
type LogClient struct {
	jsonclient.JSONClient
}

// CheckLogClient is an interface that allows (just) checking of various log contents.
type CheckLogClient interface {
	BaseURI() string
	GetSTH(context.Context) (*ct.SignedTreeHead, error)
	GetSTHConsistency(ctx context.Context, first, second uint64) ([][]byte, error)
	GetProofByHash(ctx context.Context, hash []byte, treeSize uint64) (*ct.GetProofByHashResponse, error)
}

// New constructs a new LogClient instance.
// |uri| is the base URI of the CT log instance to interact with, e.g.
// https://ct.googleapis.com/pilot
// |hc| is the underlying client to be used for HTTP requests to the CT log.
// |opts| can be used to provide a custom logger interface and a public key
// for signature verification.
func New(uri string, hc *http.Client, opts jsonclient.Options) (*LogClient, error) {
	logClient, err := jsonclient.New(uri, hc, opts)
	if err != nil {
		return nil, err
	}
	return &LogClient{*logClient}, err
}

// RspError represents a server error including HTTP information.
type RspError = jsonclient.RspError

// Attempts to add |chain| to the log, using the api end-point specified by
// |path|. If provided context expires before submission is complete an
// error will be returned.
func (c *LogClient) addChainWithRetry(ctx context.Context, ctype ct.LogEntryType, path string, chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	var resp ct.AddChainResponse
	var req ct.AddChainRequest
	for _, link := range chain {
		req.Chain = append(req.Chain, link.Data)
	}

	httpRsp, body, err := c.PostAndParseWithRetry(ctx, path, &req, &resp)
	if err != nil {
		return nil, err
	}

	var ds ct.DigitallySigned
	if rest, err := tls.Unmarshal(resp.Signature, &ds); err != nil {
		return nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
	} else if len(rest) > 0 {
		return nil, RspError{
			Err:        fmt.Errorf("trailing data (%d bytes) after DigitallySigned", len(rest)),
			StatusCode: httpRsp.StatusCode,
			Body:       body,
		}
	}

	exts, err := base64.StdEncoding.DecodeString(resp.Extensions)
	if err != nil {
		return nil, RspError{
			Err:        fmt.Errorf("invalid base64 data in Extensions (%q): %v", resp.Extensions, err),
			StatusCode: httpRsp.StatusCode,
			Body:       body,
		}
	}

	var logID ct.LogID
	copy(logID.KeyID[:], resp.ID)
	sct := &ct.SignedCertificateTimestamp{
		SCTVersion: resp.SCTVersion,
		LogID:      logID,
		Timestamp:  resp.Timestamp,
		Extensions: ct.CTExtensions(exts),
		Signature:  ds,
	}
	if err := c.VerifySCTSignature(*sct, ctype, chain); err != nil {
		return nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
	}
	return sct, nil
}

// AddChain adds the (DER represented) X509 |chain| to the log.
func (c *LogClient) AddChain(ctx context.Context, chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	return c.addChainWithRetry(ctx, ct.X509LogEntryType, ct.AddChainPath, chain)
}

// AddPreChain adds the (DER represented) Precertificate |chain| to the log.
func (c *LogClient) AddPreChain(ctx context.Context, chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	return c.addChainWithRetry(ctx, ct.PrecertLogEntryType, ct.AddPreChainPath, chain)
}

// GetSTH retrieves the current STH from the log.
// Returns a populated SignedTreeHead, or a non-nil error (which may be of type
// RspError if a raw http.Response is available).
func (c *LogClient) GetSTH(ctx context.Context) (*ct.SignedTreeHead, error) {
	var resp ct.GetSTHResponse
	httpRsp, body, err := c.GetAndParse(ctx, ct.GetSTHPath, nil, &resp)
	if err != nil {
		return nil, err
	}

	sth, err := resp.ToSignedTreeHead()
	if err != nil {
		return nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
	}

	if err := c.VerifySTHSignature(*sth); err != nil {
		return nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
	}
	return sth, nil
}

// VerifySTHSignature checks the signature in sth, returning any error encountered or nil if verification is
// successful.
func (c *LogClient) VerifySTHSignature(sth ct.SignedTreeHead) error {
	if c.Verifier == nil {
		// Can't verify signatures without a verifier
		return nil
	}
	return c.Verifier.VerifySTHSignature(sth)
}

// VerifySCTSignature checks the signature in sct for the given LogEntryType, with associated certificate chain.
func (c *LogClient) VerifySCTSignature(sct ct.SignedCertificateTimestamp, ctype ct.LogEntryType, certData []ct.ASN1Cert) error {
	if c.Verifier == nil {
		// Can't verify signatures without a verifier
		return nil
	}
	leaf, err := ct.MerkleTreeLeafFromRawChain(certData, ctype, sct.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to build MerkleTreeLeaf: %v", err)
	}
	entry := ct.LogEntry{Leaf: *leaf}
	return c.Verifier.VerifySCTSignature(sct, entry)
}

// GetSTHConsistency retrieves the consistency proof between two snapshots.
func (c *LogClient) GetSTHConsistency(ctx context.Context, first, second uint64) ([][]byte, error) {
	base10 := 10
	params := map[string]string{
		"first":  strconv.FormatUint(first, base10),
		"second": strconv.FormatUint(second, base10),
	}
	var resp ct.GetSTHConsistencyResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetSTHConsistencyPath, params, &resp); err != nil {
		return nil, err
	}
	return resp.Consistency, nil
}

// GetProofByHash returns an audit path for the hash of an SCT.
func (c *LogClient) GetProofByHash(ctx context.Context, hash []byte, treeSize uint64) (*ct.GetProofByHashResponse, error) {
	b64Hash := base64.StdEncoding.EncodeToString(hash)
	base10 := 10
	params := map[string]string{
		"tree_size": strconv.FormatUint(treeSize, base10),
		"hash":      b64Hash,
	}
	var resp ct.GetProofByHashResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetProofByHashPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAcceptedRoots retrieves the set of acceptable root certificates for a log.
func (c *LogClient) GetAcceptedRoots(ctx context.Context) ([]ct.ASN1Cert, error) {
	var resp ct.GetRootsResponse
	httpRsp, body, err := c.GetAndParse(ctx, ct.GetRootsPath, nil, &resp)
	if err != nil {
		return nil, err
	}
	var roots []ct.ASN1Cert
	for _, cert64 := range resp.Certificates {
		cert, err := base64.StdEncoding.DecodeString(cert64)
		if err != nil {
			return nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
		}
		roots = append(roots, ct.ASN1Cert{Data: cert})
	}
	return roots, nil
}

// GetEntryAndProof returns a log entry and audit path for the index of a leaf.
func (c *LogClient) GetEntryAndProof(ctx context.Context, index, treeSize uint64) (*ct.GetEntryAndProofResponse, error) {
	base10 := 10
	params := map[string]string{
		"leaf_index": strconv.FormatUint(index, base10),
		"tree_size":  strconv.FormatUint(treeSize, base10),
	}
	var resp ct.GetEntryAndProofResponse
	if _, _, err := c.GetAndParse(ctx, ct.GetEntryAndProofPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
