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

package x509util

import (
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/certificate-transparency-go/x509"
)

// ReadPossiblePEMFile loads data from a file which may be in DER format
// or may be in PEM format (with the given blockname).
func ReadPossiblePEMFile(filename, blockname string) ([][]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read data: %v", filename, err)
	}
	return dePEM(data, blockname), nil
}

// ReadPossiblePEMURL attempts to determine if the given target is a local file or a
// URL, and return the file contents regardless. It also copes with either PEM or DER
// format data.
func ReadPossiblePEMURL(target, blockname string) ([][]byte, error) {
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		// Assume it's a filename
		return ReadPossiblePEMFile(target, blockname)
	}

	rsp, err := http.Get(target)
	if err != nil {
		return nil, fmt.Errorf("failed to http.Get(%q): %v", target, err)
	}
	data, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to io.ReadAll(%q): %v", target, err)
	}
	return dePEM(data, blockname), nil
}

func dePEM(data []byte, blockname string) [][]byte {
	var results [][]byte
	if strings.Contains(string(data), "BEGIN "+blockname) {
		rest := data
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if block.Type == blockname {
				results = append(results, block.Bytes)
			}
		}
	} else {
		results = append(results, data)
	}
	return results
}

// ReadFileOrURL returns the data from a target which may be either a filename
// or an HTTP(S) URL.
func ReadFileOrURL(target string, client *http.Client) ([]byte, error) {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return os.ReadFile(target)
	}

	rsp, err := client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to http.Get(%q): %v", target, err)
	}
	return io.ReadAll(rsp.Body)
}

// GetIssuer attempts to retrieve the issuer for a certificate, by examining
// the cert's Authority Information Access extension (if present) for the
// issuer's URL and retrieving from there.
func GetIssuer(cert *x509.Certificate, client *http.Client) (*x509.Certificate, error) {
	if len(cert.IssuingCertificateURL) == 0 {
		return nil, nil
	}
	issuerURL := cert.IssuingCertificateURL[0]
	rsp, err := client.Get(issuerURL)
	if err != nil || rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get issuer from %q: %v", issuerURL, err)
	}
	defer rsp.Body.Close()
	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read issuer from %q: %v", issuerURL, err)
	}
	issuers, err := x509.ParseCertificates(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer cert: %v", err)
	}
	return issuers[0], nil
}
