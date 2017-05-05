package ca

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cloudflare/cfssl/api"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/signer"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

// ErrNoExternalCAURLs is an error used it indicate that an ExternalCA is
// configured with no URLs to which it can proxy certificate signing requests.
var ErrNoExternalCAURLs = errors.New("no external CA URLs")

// ExternalCA is able to make certificate signing requests to one of a list
// remote CFSSL API endpoints.
type ExternalCA struct {
	ExternalRequestTimeout time.Duration

	mu     sync.Mutex
	rootCA *RootCA
	urls   []string
	client *http.Client
}

// NewExternalCA creates a new ExternalCA which uses the given tlsConfig to
// authenticate to any of the given URLS of CFSSL API endpoints.
func NewExternalCA(rootCA *RootCA, tlsConfig *tls.Config, urls ...string) *ExternalCA {
	return &ExternalCA{
		ExternalRequestTimeout: 5 * time.Second,
		rootCA:                 rootCA,
		urls:                   urls,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

// Copy returns a copy of the external CA that can be updated independently
func (eca *ExternalCA) Copy() *ExternalCA {
	eca.mu.Lock()
	defer eca.mu.Unlock()

	return &ExternalCA{
		ExternalRequestTimeout: eca.ExternalRequestTimeout,
		rootCA:                 eca.rootCA,
		urls:                   eca.urls,
		client:                 eca.client,
	}
}

// UpdateTLSConfig updates the HTTP Client for this ExternalCA by creating
// a new client which uses the given tlsConfig.
func (eca *ExternalCA) UpdateTLSConfig(tlsConfig *tls.Config) {
	eca.mu.Lock()
	defer eca.mu.Unlock()

	eca.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

// UpdateURLs updates the list of CSR API endpoints by setting it to the given
// urls.
func (eca *ExternalCA) UpdateURLs(urls ...string) {
	eca.mu.Lock()
	defer eca.mu.Unlock()

	eca.urls = urls
}

// Sign signs a new certificate by proxying the given certificate signing
// request to an external CFSSL API server.
func (eca *ExternalCA) Sign(ctx context.Context, req signer.SignRequest) (cert []byte, err error) {
	// Get the current HTTP client and list of URLs in a small critical
	// section. We will use these to make certificate signing requests.
	eca.mu.Lock()
	urls := eca.urls
	client := eca.client
	eca.mu.Unlock()

	if len(urls) == 0 {
		return nil, ErrNoExternalCAURLs
	}

	csrJSON, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "unable to JSON-encode CFSSL signing request")
	}

	// Try each configured proxy URL. Return after the first success. If
	// all fail then the last error will be returned.
	for _, url := range urls {
		requestCtx, cancel := context.WithTimeout(ctx, eca.ExternalRequestTimeout)
		cert, err = makeExternalSignRequest(requestCtx, client, url, csrJSON)
		cancel()
		if err == nil {
			return append(cert, eca.rootCA.Intermediates...), err
		}
		logrus.Debugf("unable to proxy certificate signing request to %s: %s", url, err)
	}

	return nil, err
}

// CrossSignRootCA takes a RootCA object, generates a CA CSR, sends a signing request with the CA CSR to the external
// CFSSL API server in order to obtain a cross-signed root
func (eca *ExternalCA) CrossSignRootCA(ctx context.Context, rca RootCA) ([]byte, error) {
	// ExtractCertificateRequest generates a new key request, and we want to continue to use the old
	// key.  However, ExtractCertificateRequest will also convert the pkix.Name to csr.Name, which we
	// need in order to generate a signing request
	rcaSigner, err := rca.Signer()
	if err != nil {
		return nil, err
	}
	rootCert := rcaSigner.parsedCert
	cfCSRObj := csr.ExtractCertificateRequest(rootCert)

	der, err := x509.CreateCertificateRequest(cryptorand.Reader, &x509.CertificateRequest{
		RawSubjectPublicKeyInfo: rootCert.RawSubjectPublicKeyInfo,
		RawSubject:              rootCert.RawSubject,
		PublicKeyAlgorithm:      rootCert.PublicKeyAlgorithm,
		Subject:                 rootCert.Subject,
		Extensions:              rootCert.Extensions,
		DNSNames:                rootCert.DNSNames,
		EmailAddresses:          rootCert.EmailAddresses,
		IPAddresses:             rootCert.IPAddresses,
	}, rcaSigner.cryptoSigner)
	if err != nil {
		return nil, err
	}
	req := signer.SignRequest{
		Request: string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE REQUEST",
			Bytes: der,
		})),
		Subject: &signer.Subject{
			CN:    rootCert.Subject.CommonName,
			Names: cfCSRObj.Names,
		},
	}
	// cfssl actually ignores non subject alt name extensions in the CSR, so we have to add the CA extension in the signing
	// request as well
	for _, ext := range rootCert.Extensions {
		if ext.Id.Equal(BasicConstraintsOID) {
			req.Extensions = append(req.Extensions, signer.Extension{
				ID:       config.OID(ext.Id),
				Critical: ext.Critical,
				Value:    hex.EncodeToString(ext.Value),
			})
		}
	}
	return eca.Sign(ctx, req)
}

func makeExternalSignRequest(ctx context.Context, client *http.Client, url string, csrJSON []byte) (cert []byte, err error) {
	resp, err := ctxhttp.Post(ctx, client, url, "application/json", bytes.NewReader(csrJSON))
	if err != nil {
		return nil, recoverableErr{err: errors.Wrap(err, "unable to perform certificate signing request")}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, recoverableErr{err: errors.Wrap(err, "unable to read CSR response body")}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, recoverableErr{err: errors.Errorf("unexpected status code in CSR response: %d - %s", resp.StatusCode, string(body))}
	}

	var apiResponse api.Response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		logrus.Debugf("unable to JSON-parse CFSSL API response body: %s", string(body))
		return nil, recoverableErr{err: errors.Wrap(err, "unable to parse JSON response")}
	}

	if !apiResponse.Success || apiResponse.Result == nil {
		if len(apiResponse.Errors) > 0 {
			return nil, errors.Errorf("response errors: %v", apiResponse.Errors)
		}

		return nil, errors.New("certificate signing request failed")
	}

	result, ok := apiResponse.Result.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("invalid result type: %T", apiResponse.Result)
	}

	certPEM, ok := result["certificate"].(string)
	if !ok {
		return nil, errors.Errorf("invalid result certificate field type: %T", result["certificate"])
	}

	return []byte(certPEM), nil
}
