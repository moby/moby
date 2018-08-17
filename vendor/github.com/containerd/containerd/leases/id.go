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

package leases

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"time"
)

// WithRandomID sets the lease ID to a random unique value
func WithRandomID() Opt {
	return func(l *Lease) error {
		t := time.Now()
		var b [3]byte
		rand.Read(b[:])
		l.ID = fmt.Sprintf("%d-%s", t.Nanosecond(), base64.URLEncoding.EncodeToString(b[:]))
		return nil
	}
}

// WithID sets the ID for the lease
func WithID(id string) Opt {
	return func(l *Lease) error {
		l.ID = id
		return nil
	}
}
