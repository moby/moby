// Copyright 2016 Google LLC. All Rights Reserved.
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

// Package jsonclient provides a simple client for fetching and parsing
// JSON CT structures from a log.
package jsonclient

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/x509"
	"golang.org/x/net/context/ctxhttp"
	"k8s.io/klog/v2"
)

const maxJitter = 250 * time.Millisecond

type backoffer interface {
	// set adjusts/increases the current backoff interval (typically on retryable failure);
	// if the optional parameter is provided, this will be used as the interval if it is greater
	// than the currently set interval.  Returns the current wait period so that it can be
	// logged along with any error message.
	set(*time.Duration) time.Duration
	// decreaseMultiplier reduces the current backoff multiplier, typically on success.
	decreaseMultiplier()
	// until returns the time until which the client should wait before making a request,
	// it may be in the past in which case it should be ignored.
	until() time.Time
}

// JSONClient provides common functionality for interacting with a JSON server
// that uses cryptographic signatures.
type JSONClient struct {
	uri        string                // the base URI of the server. e.g. https://ct.googleapis/pilot
	httpClient *http.Client          // used to interact with the server via HTTP
	Verifier   *ct.SignatureVerifier // nil for no verification (e.g. no public key available)
	logger     Logger                // interface to use for logging warnings and errors
	backoff    backoffer             // object used to store and calculate backoff information
	userAgent  string                // If set, this is sent as the UserAgent header.
}

// Logger is a simple logging interface used to log internal errors and warnings
type Logger interface {
	// Printf formats and logs a message
	Printf(string, ...interface{})
}

// Options are the options for creating a new JSONClient.
type Options struct {
	// Interface to use for logging warnings and errors, if nil the
	// standard library log package will be used.
	Logger Logger
	// PEM format public key to use for signature verification.
	PublicKey string
	// DER format public key to use for signature verification.
	PublicKeyDER []byte
	// UserAgent, if set, will be sent as the User-Agent header with each request.
	UserAgent string
}

// ParsePublicKey parses and returns the public key contained in opts.
// If both opts.PublicKey and opts.PublicKeyDER are set, PublicKeyDER is used.
// If neither is set, nil will be returned.
func (opts *Options) ParsePublicKey() (crypto.PublicKey, error) {
	if len(opts.PublicKeyDER) > 0 {
		return x509.ParsePKIXPublicKey(opts.PublicKeyDER)
	}

	if opts.PublicKey != "" {
		pubkey, _ /* keyhash */, rest, err := ct.PublicKeyFromPEM([]byte(opts.PublicKey))
		if err != nil {
			return nil, err
		}
		if len(rest) > 0 {
			return nil, errors.New("extra data found after PEM key decoded")
		}
		return pubkey, nil
	}

	return nil, nil
}

type basicLogger struct{}

func (bl *basicLogger) Printf(msg string, args ...interface{}) {
	log.Printf(msg, args...)
}

// RspError represents an error that occurred when processing a response from a server,
// and also includes key details from the http.Response that triggered the error.
type RspError struct {
	Err        error
	StatusCode int
	Body       []byte
}

// Error formats the RspError instance, focusing on the error.
func (e RspError) Error() string {
	return e.Err.Error()
}

// New constructs a new JSONClient instance, for the given base URI, using the
// given http.Client object (if provided) and the Options object.
// If opts does not specify a public key, signatures will not be verified.
func New(uri string, hc *http.Client, opts Options) (*JSONClient, error) {
	pubkey, err := opts.ParsePublicKey()
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %v", err)
	}

	var verifier *ct.SignatureVerifier
	if pubkey != nil {
		var err error
		verifier, err = ct.NewSignatureVerifier(pubkey)
		if err != nil {
			return nil, err
		}
	}

	if hc == nil {
		hc = new(http.Client)
	}
	logger := opts.Logger
	if logger == nil {
		logger = &basicLogger{}
	}
	return &JSONClient{
		uri:        strings.TrimRight(uri, "/"),
		httpClient: hc,
		Verifier:   verifier,
		logger:     logger,
		backoff:    &backoff{},
		userAgent:  opts.UserAgent,
	}, nil
}

// BaseURI returns the base URI that the JSONClient makes queries to.
func (c *JSONClient) BaseURI() string {
	return c.uri
}

// GetAndParse makes a HTTP GET call to the given path, and attempts to parse
// the response as a JSON representation of the rsp structure.  Returns the
// http.Response, the body of the response, and an error (which may be of
// type RspError if the HTTP response was available).
func (c *JSONClient) GetAndParse(ctx context.Context, path string, params map[string]string, rsp interface{}) (*http.Response, []byte, error) {
	if ctx == nil {
		return nil, nil, errors.New("context.Context required")
	}
	// Build a GET request with URL-encoded parameters.
	vals := url.Values{}
	for k, v := range params {
		vals.Add(k, v)
	}
	fullURI := fmt.Sprintf("%s%s?%s", c.uri, path, vals.Encode())
	klog.V(2).Infof("GET %s", fullURI)
	httpReq, err := http.NewRequest(http.MethodGet, fullURI, nil)
	if err != nil {
		return nil, nil, err
	}
	if len(c.userAgent) != 0 {
		httpReq.Header.Set("User-Agent", c.userAgent)
	}

	httpRsp, err := ctxhttp.Do(ctx, c.httpClient, httpReq)
	if err != nil {
		return nil, nil, err
	}

	// Read everything now so http.Client can reuse the connection.
	body, err := io.ReadAll(httpRsp.Body)
	httpRsp.Body.Close()
	if err != nil {
		return nil, nil, RspError{Err: fmt.Errorf("failed to read response body: %v", err), StatusCode: httpRsp.StatusCode, Body: body}
	}

	if httpRsp.StatusCode != http.StatusOK {
		return nil, nil, RspError{Err: fmt.Errorf("got HTTP Status %q", httpRsp.Status), StatusCode: httpRsp.StatusCode, Body: body}
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(rsp); err != nil {
		return nil, nil, RspError{Err: err, StatusCode: httpRsp.StatusCode, Body: body}
	}

	return httpRsp, body, nil
}

// PostAndParse makes a HTTP POST call to the given path, including the request
// parameters, and attempts to parse the response as a JSON representation of
// the rsp structure. Returns the http.Response, the body of the response, and
// an error (which may be of type RspError if the HTTP response was available).
func (c *JSONClient) PostAndParse(ctx context.Context, path string, req, rsp interface{}) (*http.Response, []byte, error) {
	if ctx == nil {
		return nil, nil, errors.New("context.Context required")
	}
	// Build a POST request with JSON body.
	postBody, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	fullURI := fmt.Sprintf("%s%s", c.uri, path)
	klog.V(2).Infof("POST %s", fullURI)
	httpReq, err := http.NewRequest(http.MethodPost, fullURI, bytes.NewReader(postBody))
	if err != nil {
		return nil, nil, err
	}
	if len(c.userAgent) != 0 {
		httpReq.Header.Set("User-Agent", c.userAgent)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpRsp, err := ctxhttp.Do(ctx, c.httpClient, httpReq)

	// Read all of the body, if there is one, so that the http.Client can do Keep-Alive.
	var body []byte
	if httpRsp != nil {
		body, err = io.ReadAll(httpRsp.Body)
		httpRsp.Body.Close()
	}
	if err != nil {
		if httpRsp != nil {
			return nil, nil, RspError{StatusCode: httpRsp.StatusCode, Body: body, Err: err}
		}
		return nil, nil, err
	}

	if httpRsp.StatusCode == http.StatusOK {
		if err = json.Unmarshal(body, &rsp); err != nil {
			return nil, nil, RspError{StatusCode: httpRsp.StatusCode, Body: body, Err: err}
		}
	}
	return httpRsp, body, nil
}

// waitForBackoff blocks until the defined backoff interval or context has expired, if the returned
// not before time is in the past it returns immediately.
func (c *JSONClient) waitForBackoff(ctx context.Context) error {
	dur := time.Until(c.backoff.until().Add(time.Millisecond * time.Duration(rand.Intn(int(maxJitter.Seconds()*1000)))))
	if dur < 0 {
		dur = 0
	}
	backoffTimer := time.NewTimer(dur)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-backoffTimer.C:
	}
	return nil
}

// PostAndParseWithRetry makes a HTTP POST call, but retries (with backoff) on
// retryable errors; the caller should set a deadline on the provided context
// to prevent infinite retries.  Return values are as for PostAndParse.
func (c *JSONClient) PostAndParseWithRetry(ctx context.Context, path string, req, rsp interface{}) (*http.Response, []byte, error) {
	if ctx == nil {
		return nil, nil, errors.New("context.Context required")
	}
	for {
		httpRsp, body, err := c.PostAndParse(ctx, path, req, rsp)
		if err != nil {
			// Don't retry context errors.
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil, nil, err
			}
			wait := c.backoff.set(nil)
			c.logger.Printf("Request to %s failed, backing-off %s: %s", c.uri, wait, err)
		} else {
			switch {
			case httpRsp.StatusCode == http.StatusOK:
				return httpRsp, body, nil
			case httpRsp.StatusCode == http.StatusRequestTimeout:
				// Request timeout, retry immediately
				c.logger.Printf("Request to %s timed out, retrying immediately", c.uri)
			case httpRsp.StatusCode == http.StatusServiceUnavailable:
				fallthrough
			case httpRsp.StatusCode == http.StatusTooManyRequests:
				var backoff *time.Duration
				// Retry-After may be either a number of seconds as a int or a RFC 1123
				// date string (RFC 7231 Section 7.1.3)
				if retryAfter := httpRsp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, err := strconv.Atoi(retryAfter); err == nil {
						b := time.Duration(seconds) * time.Second
						backoff = &b
					} else if date, err := time.Parse(time.RFC1123, retryAfter); err == nil {
						b := time.Until(date)
						backoff = &b
					}
				}
				wait := c.backoff.set(backoff)
				c.logger.Printf("Request to %s failed, backing-off for %s: got HTTP status %s", c.uri, wait, httpRsp.Status)
			default:
				return nil, nil, RspError{
					StatusCode: httpRsp.StatusCode,
					Body:       body,
					Err:        fmt.Errorf("got HTTP status %q", httpRsp.Status)}
			}
		}
		if err := c.waitForBackoff(ctx); err != nil {
			return nil, nil, err
		}
	}
}
