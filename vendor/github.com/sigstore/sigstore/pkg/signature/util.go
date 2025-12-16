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

package signature

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/sigstore/sigstore/pkg/signature/options"
	sigpayload "github.com/sigstore/sigstore/pkg/signature/payload"
)

// SignImage signs a container manifest using the specified signer object
func SignImage(signer SignerVerifier, image name.Digest, optionalAnnotations map[string]interface{}) (payload, signature []byte, err error) {
	imgPayload := sigpayload.Cosign{
		Image:       image,
		Annotations: optionalAnnotations,
	}
	payload, err = json.Marshal(imgPayload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}
	signature, err = signer.SignMessage(bytes.NewReader(payload))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign payload: %w", err)
	}
	return payload, signature, nil
}

// VerifyImageSignature verifies a signature over a container manifest
func VerifyImageSignature(signer SignerVerifier, payload, signature []byte) (image name.Digest, annotations map[string]interface{}, err error) {
	if err := signer.VerifySignature(bytes.NewReader(signature), bytes.NewReader(payload)); err != nil {
		return name.Digest{}, nil, fmt.Errorf("signature verification failed: %w", err)
	}
	var imgPayload sigpayload.Cosign
	if err := json.Unmarshal(payload, &imgPayload); err != nil {
		return name.Digest{}, nil, fmt.Errorf("could not deserialize image payload: %w", err)
	}
	return imgPayload.Image, imgPayload.Annotations, nil
}

// GetOptsFromAlgorithmDetails returns a list of LoadOptions that are
// appropriate for the given algorithm details. It ignores the hash type because
// that can be retrieved from the algorithm details.
func GetOptsFromAlgorithmDetails(algorithmDetails AlgorithmDetails, opts ...LoadOption) []LoadOption {
	res := []LoadOption{options.WithHash(algorithmDetails.hashType)}
	for _, opt := range opts {
		var useED25519ph bool
		var rsaPSSOptions *rsa.PSSOptions
		opt.ApplyED25519ph(&useED25519ph)
		opt.ApplyRSAPSS(&rsaPSSOptions)
		if useED25519ph || rsaPSSOptions != nil {
			res = append(res, opt)
		}
	}
	return res
}
