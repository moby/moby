// Package client is a CT log client implementation and contains types and code
// for interacting with RFC6962-compliant CT Log instances.
// See http://tools.ietf.org/html/rfc6962 for details
package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	ct "github.com/google/certificate-transparency/go"
	"golang.org/x/net/context"
)

// URI paths for CT Log endpoints
const (
	AddChainPath          = "/ct/v1/add-chain"
	AddPreChainPath       = "/ct/v1/add-pre-chain"
	AddJSONPath           = "/ct/v1/add-json"
	GetSTHPath            = "/ct/v1/get-sth"
	GetEntriesPath        = "/ct/v1/get-entries"
	GetProofByHashPath    = "/ct/v1/get-proof-by-hash"
	GetSTHConsistencyPath = "/ct/v1/get-sth-consistency"
)

// LogClient represents a client for a given CT Log instance
type LogClient struct {
	uri        string                // the base URI of the log. e.g. http://ct.googleapis/pilot
	httpClient *http.Client          // used to interact with the log via HTTP
	verifier   *ct.SignatureVerifier // nil if no public key for log available
}

//////////////////////////////////////////////////////////////////////////////////
// JSON structures follow.
// These represent the structures returned by the CT Log server.
//////////////////////////////////////////////////////////////////////////////////

// addChainRequest represents the JSON request body sent to the add-chain CT
// method.
type addChainRequest struct {
	Chain [][]byte `json:"chain"`
}

// addChainResponse represents the JSON response to the add-chain CT method.
// An SCT represents a Log's promise to integrate a [pre-]certificate into the
// log within a defined period of time.
type addChainResponse struct {
	SCTVersion ct.Version `json:"sct_version"` // SCT structure version
	ID         []byte     `json:"id"`          // Log ID
	Timestamp  uint64     `json:"timestamp"`   // Timestamp of issuance
	Extensions string     `json:"extensions"`  // Holder for any CT extensions
	Signature  []byte     `json:"signature"`   // Log signature for this SCT
}

// addJSONRequest represents the JSON request body sent to the add-json CT
// method.
type addJSONRequest struct {
	Data interface{} `json:"data"`
}

// getSTHResponse respresents the JSON response to the get-sth CT method
type getSTHResponse struct {
	TreeSize          uint64 `json:"tree_size"`           // Number of certs in the current tree
	Timestamp         uint64 `json:"timestamp"`           // Time that the tree was created
	SHA256RootHash    []byte `json:"sha256_root_hash"`    // Root hash of the tree
	TreeHeadSignature []byte `json:"tree_head_signature"` // Log signature for this STH
}

// getConsistencyProofResponse represents the JSON response to the get-consistency-proof CT method
type getConsistencyProofResponse struct {
	Consistency [][]byte `json:"consistency"`
}

// getAuditProofResponse represents the JSON response to the CT get-audit-proof method
type getAuditProofResponse struct {
	Hash     []string `json:"hash"`      // the hashes which make up the proof
	TreeSize uint64   `json:"tree_size"` // the tree size against which this proof is constructed
}

// getAcceptedRootsResponse represents the JSON response to the CT get-roots method.
type getAcceptedRootsResponse struct {
	Certificates []string `json:"certificates"`
}

// getEntryAndProodReponse represents the JSON response to the CT get-entry-and-proof method
type getEntryAndProofResponse struct {
	LeafInput string   `json:"leaf_input"` // the entry itself
	ExtraData string   `json:"extra_data"` // any chain provided when the entry was added to the log
	AuditPath []string `json:"audit_path"` // the corresponding proof
}

// GetProofByHashResponse represents the JSON response to the CT get-proof-by-hash method.
type GetProofByHashResponse struct {
	LeafIndex int64    `json:"leaf_index"` // The 0-based index of the end entity corresponding to the "hash" parameter.
	AuditPath [][]byte `json:"audit_path"` // An array of base64-encoded Merkle Tree nodes proving the inclusion of the chosen certificate.
}

// New constructs a new LogClient instance.
// |uri| is the base URI of the CT log instance to interact with, e.g.
// http://ct.googleapis.com/pilot
// |hc| is the underlying client to be used for HTTP requests to the CT log.
func New(uri string, hc *http.Client) *LogClient {
	if hc == nil {
		hc = new(http.Client)
	}
	return &LogClient{uri: uri, httpClient: hc}
}

// NewWithPubKey constructs a new LogClient instance that includes public
// key information for the log; this instance will check signatures on
// responses from the log.
func NewWithPubKey(uri string, hc *http.Client, pemEncodedKey string) (*LogClient, error) {
	pubkey, _, rest, err := ct.PublicKeyFromPEM([]byte(pemEncodedKey))
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, errors.New("extra data found after PEM key decoded")
	}

	verifier, err := ct.NewSignatureVerifier(pubkey)
	if err != nil {
		return nil, err
	}

	if hc == nil {
		hc = new(http.Client)
	}
	return &LogClient{uri: uri, httpClient: hc, verifier: verifier}, nil
}

// Makes a HTTP call to |uri|, and attempts to parse the response as a
// JSON representation of the structure in |res|. Uses |ctx| to
// control the HTTP call (so it can have a timeout or be cancelled by
// the caller), and |httpClient| to make the actual HTTP call.
// Returns a non-nil |error| if there was a problem.
func fetchAndParse(ctx context.Context, httpClient *http.Client, uri string, res interface{}) error {
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return err
	}
	req.Cancel = ctx.Done()
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Make sure everything is read, so http.Client can reuse the connection.
	defer ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("got HTTP Status %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		return err
	}

	return nil
}

// Makes a HTTP POST call to |uri|, and attempts to parse the response as a JSON
// representation of the structure in |res|.
// Returns a non-nil |error| if there was a problem.
func (c *LogClient) postAndParse(uri string, req interface{}, res interface{}) (*http.Response, string, error) {
	postBody, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}
	httpReq, err := http.NewRequest(http.MethodPost, uri, bytes.NewReader(postBody))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	// Read all of the body, if there is one, so that the http.Client can do
	// Keep-Alive:
	var body []byte
	if resp != nil {
		body, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}
	if err != nil {
		return resp, string(body), err
	}
	if resp.StatusCode == 200 {
		if err != nil {
			return resp, string(body), err
		}
		if err = json.Unmarshal(body, &res); err != nil {
			return resp, string(body), err
		}
	}
	return resp, string(body), nil
}

func backoffForRetry(ctx context.Context, d time.Duration) error {
	backoffTimer := time.NewTimer(d)
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-backoffTimer.C:
		}
	} else {
		<-backoffTimer.C
	}
	return nil
}

// Attempts to add |chain| to the log, using the api end-point specified by
// |path|. If provided context expires before submission is complete an
// error will be returned.
func (c *LogClient) addChainWithRetry(ctx context.Context, ctype ct.LogEntryType, path string, chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	var resp addChainResponse
	var req addChainRequest
	for _, link := range chain {
		req.Chain = append(req.Chain, link)
	}
	httpStatus := "Unknown"
	backoffSeconds := 0
	done := false
	for !done {
		if backoffSeconds > 0 {
			log.Printf("Got %s, backing-off %d seconds", httpStatus, backoffSeconds)
		}
		err := backoffForRetry(ctx, time.Second*time.Duration(backoffSeconds))
		if err != nil {
			return nil, err
		}
		if backoffSeconds > 0 {
			backoffSeconds = 0
		}
		httpResp, _, err := c.postAndParse(c.uri+path, &req, &resp)
		if err != nil {
			backoffSeconds = 10
			continue
		}
		switch {
		case httpResp.StatusCode == 200:
			done = true
		case httpResp.StatusCode == 408:
			// request timeout, retry immediately
		case httpResp.StatusCode == 503:
			// Retry
			backoffSeconds = 10
			if retryAfter := httpResp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					backoffSeconds = seconds
				}
			}
		default:
			return nil, fmt.Errorf("got HTTP Status %s", httpResp.Status)
		}
		httpStatus = httpResp.Status
	}

	ds, err := ct.UnmarshalDigitallySigned(bytes.NewReader(resp.Signature))
	if err != nil {
		return nil, err
	}

	var logID ct.SHA256Hash
	copy(logID[:], resp.ID)
	sct := &ct.SignedCertificateTimestamp{
		SCTVersion: resp.SCTVersion,
		LogID:      logID,
		Timestamp:  resp.Timestamp,
		Extensions: ct.CTExtensions(resp.Extensions),
		Signature:  *ds}
	err = c.VerifySCTSignature(*sct, ctype, chain)
	if err != nil {
		return nil, err
	}
	return sct, nil
}

// AddChain adds the (DER represented) X509 |chain| to the log.
func (c *LogClient) AddChain(chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	return c.addChainWithRetry(nil, ct.X509LogEntryType, AddChainPath, chain)
}

// AddPreChain adds the (DER represented) Precertificate |chain| to the log.
func (c *LogClient) AddPreChain(chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	return c.addChainWithRetry(nil, ct.PrecertLogEntryType, AddPreChainPath, chain)
}

// AddChainWithContext adds the (DER represented) X509 |chain| to the log and
// fails if the provided context expires before the chain is submitted.
func (c *LogClient) AddChainWithContext(ctx context.Context, chain []ct.ASN1Cert) (*ct.SignedCertificateTimestamp, error) {
	return c.addChainWithRetry(ctx, ct.X509LogEntryType, AddChainPath, chain)
}

// AddJSON submits arbitrary data to to XJSON server.
func (c *LogClient) AddJSON(data interface{}) (*ct.SignedCertificateTimestamp, error) {
	req := addJSONRequest{
		Data: data,
	}
	var resp addChainResponse
	_, _, err := c.postAndParse(c.uri+AddJSONPath, &req, &resp)
	if err != nil {
		return nil, err
	}
	ds, err := ct.UnmarshalDigitallySigned(bytes.NewReader(resp.Signature))
	if err != nil {
		return nil, err
	}
	var logID ct.SHA256Hash
	copy(logID[:], resp.ID)
	return &ct.SignedCertificateTimestamp{
		SCTVersion: resp.SCTVersion,
		LogID:      logID,
		Timestamp:  resp.Timestamp,
		Extensions: ct.CTExtensions(resp.Extensions),
		Signature:  *ds}, nil
}

// GetSTH retrieves the current STH from the log.
// Returns a populated SignedTreeHead, or a non-nil error.
func (c *LogClient) GetSTH() (sth *ct.SignedTreeHead, err error) {
	var resp getSTHResponse
	if err = fetchAndParse(context.TODO(), c.httpClient, c.uri+GetSTHPath, &resp); err != nil {
		return
	}
	sth = &ct.SignedTreeHead{
		TreeSize:  resp.TreeSize,
		Timestamp: resp.Timestamp,
	}

	if len(resp.SHA256RootHash) != sha256.Size {
		return nil, fmt.Errorf("sha256_root_hash is invalid length, expected %d got %d", sha256.Size, len(resp.SHA256RootHash))
	}
	copy(sth.SHA256RootHash[:], resp.SHA256RootHash)

	ds, err := ct.UnmarshalDigitallySigned(bytes.NewReader(resp.TreeHeadSignature))
	if err != nil {
		return nil, err
	}
	sth.TreeHeadSignature = *ds
	err = c.VerifySTHSignature(*sth)
	if err != nil {
		return nil, err
	}
	return
}

// VerifySTHSignature checks the signature in sth, returning any error encountered or nil if verification is
// successful.
func (c *LogClient) VerifySTHSignature(sth ct.SignedTreeHead) error {
	if c.verifier == nil {
		// Can't verify signatures without a verifier
		return nil
	}
	return c.verifier.VerifySTHSignature(sth)
}

// VerifySCTSignature checks the signature in sct for the given LogEntryType, with associated certificate chain.
func (c *LogClient) VerifySCTSignature(sct ct.SignedCertificateTimestamp, ctype ct.LogEntryType, certData []ct.ASN1Cert) error {
	if c.verifier == nil {
		// Can't verify signatures without a verifier
		return nil
	}

	if ctype == ct.PrecertLogEntryType {
		// TODO(drysdale): cope with pre-certs, which need to have the
		// following fields set:
		//    leaf.PrecertEntry.TBSCertificate
		//    leaf.PrecertEntry.IssuerKeyHash  (SHA-256 of issuer's public key)
		return errors.New("SCT verification for pre-certificates unimplemented")
	}
	// Build enough of a Merkle tree leaf for the verifier to work on.
	leaf := ct.MerkleTreeLeaf{
		Version:  sct.SCTVersion,
		LeafType: ct.TimestampedEntryLeafType,
		TimestampedEntry: ct.TimestampedEntry{
			Timestamp:  sct.Timestamp,
			EntryType:  ctype,
			X509Entry:  certData[0],
			Extensions: sct.Extensions}}
	entry := ct.LogEntry{Leaf: leaf}
	return c.verifier.VerifySCTSignature(sct, entry)
}

// GetSTHConsistency retrieves the consistency proof between two snapshots.
func (c *LogClient) GetSTHConsistency(ctx context.Context, first, second uint64) ([][]byte, error) {
	u := fmt.Sprintf("%s%s?first=%d&second=%d", c.uri, GetSTHConsistencyPath, first, second)
	var resp getConsistencyProofResponse
	if err := fetchAndParse(ctx, c.httpClient, u, &resp); err != nil {
		return nil, err
	}
	return resp.Consistency, nil
}

// GetProofByHash returns an audit path for the hash of an SCT.
func (c *LogClient) GetProofByHash(ctx context.Context, hash []byte, treeSize uint64) (*GetProofByHashResponse, error) {
	b64Hash := url.QueryEscape(base64.StdEncoding.EncodeToString(hash))
	u := fmt.Sprintf("%s%s?tree_size=%d&hash=%v", c.uri, GetProofByHashPath, treeSize, b64Hash)
	var resp GetProofByHashResponse
	if err := fetchAndParse(ctx, c.httpClient, u, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
