package ca

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/cloudflare/cfssl/api"
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
		cert, err = makeExternalSignRequest(ctx, client, url, csrJSON)
		if err == nil {
			return eca.rootCA.AppendFirstRootPEM(cert)
		}

		logrus.Debugf("unable to proxy certificate signing request to %s: %s", url, err)
	}

	return nil, err
}

func makeExternalSignRequest(ctx context.Context, client *http.Client, url string, csrJSON []byte) (cert []byte, err error) {
	resp, err := ctxhttp.Post(ctx, client, url, "application/json", bytes.NewReader(csrJSON))
	if err != nil {
		return nil, recoverableErr{err: errors.Wrap(err, "unable to perform certificate signing request")}
	}

	doneReading := make(chan struct{})
	bodyClosed := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-doneReading:
		}
		resp.Body.Close()
		close(bodyClosed)
	}()

	body, err := ioutil.ReadAll(resp.Body)
	close(doneReading)
	<-bodyClosed
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
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
