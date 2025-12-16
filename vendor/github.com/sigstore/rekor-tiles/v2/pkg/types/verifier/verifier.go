// Copyright 2025 The Sigstore Authors
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

package verifier

import (
	"fmt"

	pb "github.com/sigstore/rekor-tiles/v2/pkg/generated/protobuf"
)

// Validate validates there are no missing field in a Verifier protobuf
func Validate(v *pb.Verifier) error {
	publicKey := v.GetPublicKey()
	x509Cert := v.GetX509Certificate()
	if publicKey == nil && x509Cert == nil {
		return fmt.Errorf("missing signature public key or X.509 certificate")
	}
	if publicKey != nil {
		if len(publicKey.GetRawBytes()) == 0 {
			return fmt.Errorf("missing public key raw bytes")
		}
	}
	if x509Cert != nil {
		if len(x509Cert.GetRawBytes()) == 0 {
			return fmt.Errorf("missing X.509 certificate raw bytes")
		}
	}
	return nil
}
