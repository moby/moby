/*
	Copyright The containerd Authors.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/
package containerdbackport

// From: https://github.com/containerd/containerd/blob/6b6f53cb1e0bac29d9fc8792266ed56a684a012e/core/remotes/docker/resolver.go

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

import (
	"github.com/containerd/log"
	"github.com/pkg/errors"
)

// NewHTTPFallback returns http.RoundTripper which allows fallback from https to
// http for registry endpoints with configurations for both http and TLS,
// such as defaulted localhost endpoints.
func NewHTTPFallback(transport http.RoundTripper) http.RoundTripper {
	return &httpFallback{
		super: transport,
	}
}

type httpFallback struct {
	super http.RoundTripper
	host  string
	mu    sync.Mutex
}

func (f *httpFallback) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	fallback := f.host == r.URL.Host
	f.mu.Unlock()

	// only fall back if the same host had previously fell back
	if !fallback {
		ctx := r.Context()
		resp, err := f.super.RoundTrip(r)
		if r.URL == nil {
			log.G(ctx).WithFields(log.Fields{
				"error": err,
			}).Warn("RoundTrip turned r.URL to nil")
		}
		if !isTLSError(err) && !isPortError(err, r.URL.Host) {
			return resp, err
		}
	}

	plainHTTPUrl := *r.URL
	plainHTTPUrl.Scheme = "http"

	plainHTTPRequest := *r
	plainHTTPRequest.URL = &plainHTTPUrl

	if !fallback {
		f.mu.Lock()
		if f.host != r.URL.Host {
			f.host = r.URL.Host
		}
		f.mu.Unlock()

		// update body on the second attempt
		if r.Body != nil && r.GetBody != nil {
			body, err := r.GetBody()
			if err != nil {
				return nil, err
			}
			plainHTTPRequest.Body = body
		}
	}

	return f.super.RoundTrip(&plainHTTPRequest)
}

func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	var tlsErr tls.RecordHeaderError
	if errors.As(err, &tlsErr) && string(tlsErr.RecordHeader[:]) == "HTTP/" {
		return true
	}
	if strings.Contains(err.Error(), "TLS handshake timeout") {
		return true
	}

	return false
}

func isPortError(err error, host string) bool {
	if isConnError(err) || os.IsTimeout(err) {
		if _, port, _ := net.SplitHostPort(host); port != "" {
			// Port is specified, will not retry on different port with scheme change
			return false
		}
		return true
	}

	return false
}
