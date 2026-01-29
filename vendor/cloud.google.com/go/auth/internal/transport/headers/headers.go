// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package headers

import (
	"net/http"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/internal"
)

// SetAuthHeader uses the provided token to set the Authorization and trust
// boundary headers on a request. If the token.Type is empty, the type is
// assumed to be Bearer.
func SetAuthHeader(token *auth.Token, req *http.Request) {
	typ := token.Type
	if typ == "" {
		typ = internal.TokenTypeBearer
	}
	req.Header.Set("Authorization", typ+" "+token.Value)

	if headerVal, setHeader := getTrustBoundaryHeader(token); setHeader {
		req.Header.Set("x-allowed-locations", headerVal)
	}
}

// SetAuthMetadata uses the provided token to set the Authorization and trust
// boundary metadata. If the token.Type is empty, the type is assumed to be
// Bearer.
func SetAuthMetadata(token *auth.Token, m map[string]string) {
	typ := token.Type
	if typ == "" {
		typ = internal.TokenTypeBearer
	}
	m["authorization"] = typ + " " + token.Value

	if headerVal, setHeader := getTrustBoundaryHeader(token); setHeader {
		m["x-allowed-locations"] = headerVal
	}
}

func getTrustBoundaryHeader(token *auth.Token) (val string, present bool) {
	if data, ok := token.Metadata[internal.TrustBoundaryDataKey]; ok {
		if tbd, ok := data.(internal.TrustBoundaryData); ok {
			return tbd.TrustBoundaryHeader()
		}
	}
	return "", false
}
