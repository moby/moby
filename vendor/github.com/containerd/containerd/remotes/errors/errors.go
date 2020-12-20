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

package errors

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

var _ error = ErrUnexpectedStatus{}

// ErrUnexpectedStatus is returned if a registry API request returned with unexpected HTTP status
type ErrUnexpectedStatus struct {
	Status     string
	StatusCode int
	Body       []byte
}

func (e ErrUnexpectedStatus) Error() string {
	return fmt.Sprintf("unexpected status: %s", e.Status)
}

// NewUnexpectedStatusErr creates an ErrUnexpectedStatus from HTTP response
func NewUnexpectedStatusErr(resp *http.Response) error {
	var b []byte
	if resp.Body != nil {
		b, _ = ioutil.ReadAll(io.LimitReader(resp.Body, 64000)) // 64KB
	}
	return ErrUnexpectedStatus{Status: resp.Status, StatusCode: resp.StatusCode, Body: b}
}
