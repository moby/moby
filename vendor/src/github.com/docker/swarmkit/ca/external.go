package ca

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/cloudflare/cfssl/api"
	"github.com/cloudflare/cfssl/signer"
)

// ErrNoExternalCAURLs is an error used it indicate that an ExternalCA is
// configured with no URLs to which it can proxy certificate signing requests.
var ErrNoExternalCAURLs = errors.New("no external CA URLs")

// ExternalCA is able to make certificate signing requests to one of a list
// remote CFSSL API endpoints.
type ExternalCA struct {
	mu     sync.Mutex
	rootCA *RootCA
	urls   []string
	client *http.Client
}

// NewExternalCA creates a new ExternalCA which uses the given tlsConfig to
// authenticate to any of the given URLS of CFSSL API endpoints.
func NewExternalCA(rootCA *RootCA, tlsConfig *tls.Config, urls ...string) *ExternalCA {
	return &ExternalCA{
		rootCA: rootCA,
		urls:   urls,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
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
func (eca *ExternalCA) Sign(req signer.SignRequest) (cert []byte, err error) {
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
		return nil, fmt.Errorf("unable to JSON-encode CFSSL signing request: %s", err)
	}

	// Try each configured proxy URL. Return after the first success. If
	// all fail then the last error will be returned.
	for _, url := range urls {
		cert, err = makeExternalSignRequest(client, url, csrJSON)
		if err == nil {
			return eca.rootCA.AppendFirstRootPEM(cert)
		}

		log.Debugf("unable to proxy certificate signing request to %s: %s", url, err)
	}

	return nil, err
}

func makeExternalSignRequest(client *http.Client, url string, csrJSON []byte) (cert []byte, err error) {
	resp, err := client.Post(url, "application/json", bytes.NewReader(csrJSON))
	if err != nil {
		return nil, fmt.Errorf("unable to perform certificate signing request: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read CSR response body: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code in CSR response: %d - %s", resp.StatusCode, string(body))
	}

	var apiResponse api.Response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		log.Debugf("unable to JSON-parse CFSSL API response body: %s", string(body))
		return nil, fmt.Errorf("unable to parse JSON response: %s", err)
	}

	if !apiResponse.Success || apiResponse.Result == nil {
		if len(apiResponse.Errors) > 0 {
			return nil, fmt.Errorf("response errors: %v", apiResponse.Errors)
		}

		return nil, fmt.Errorf("certificate signing request failed")
	}

	result, ok := apiResponse.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result type: %T", apiResponse.Result)
	}

	certPEM, ok := result["certificate"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid result certificate field type: %T", result["certificate"])
	}

	return []byte(certPEM), nil
}
