//
// Copyright 2021 The Sigstore Authors.
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

package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FileOrURLReadCloser reads content either from a URL or a byte slice
// Note: Caller is responsible for closing the returned ReadCloser
// Note: This must never be called from any server codepath to prevent SSRF
func FileOrURLReadCloser(ctx context.Context, url string, content []byte) (io.ReadCloser, error) {
	var dataReader io.ReadCloser
	if url != "" {
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("error received while fetching artifact '%v': %v", url, resp.Status)
		}

		dataReader = resp.Body
	} else {
		dataReader = io.NopCloser(bytes.NewReader(content))
	}
	return dataReader, nil
}
