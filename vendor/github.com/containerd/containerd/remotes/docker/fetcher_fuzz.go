//go:build gofuzz

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

package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
)

func FuzzFetcher(data []byte) int {
	dataLen := len(data)
	if dataLen == 0 {
		return -1
	}

	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-range", fmt.Sprintf("bytes %d-%d/%d", 0, dataLen-1, dataLen))
		rw.Header().Set("content-length", fmt.Sprintf("%d", dataLen))
		rw.Write(data)
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	if err != nil {
		return 0
	}

	f := dockerFetcher{&dockerBase{
		repository: "nonempty",
	}}
	host := RegistryHost{
		Client: s.Client(),
		Host:   u.Host,
		Scheme: u.Scheme,
		Path:   u.Path,
	}

	ctx := context.Background()
	req := f.request(host, http.MethodGet)
	rc, err := f.open(ctx, req, "", 0)
	if err != nil {
		return 0
	}
	b, err := io.ReadAll(rc)
	if err != nil {
		return 0
	}

	expected := data
	if len(b) != len(expected) {
		panic("len of request is not equal to len of expected but should be")
	}
	return 1
}
