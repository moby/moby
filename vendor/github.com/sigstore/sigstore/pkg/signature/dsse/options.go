//
// Copyright 2026 The Sigstore Authors.
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

package dsse

// Option is a functional option for WrapVerifier, WrapSignerVerifier,
// WrapMultiVerifierWithOpts, and WrapMultiSignerVerifierWithOpts.
type Option func(*wrapConfig)

type wrapConfig struct {
	decodedPayload      *[]byte
	expectedPayloadType string
}

// WithDecodedPayload returns an Option that causes the verifier to write
// the decoded envelope payload into the provided byte slice pointer. This
// avoids a redundant base64 decode when callers need the payload after
// verification.
func WithDecodedPayload(p *[]byte) Option {
	return func(c *wrapConfig) {
		c.decodedPayload = p
	}
}

// WithExpectedPayloadType returns an Option that causes the verifier to
// check the envelope's payload type against the expected value before
// verifying. If the types do not match, verification fails immediately.
// When this option is not set, any payload type is accepted.
func WithExpectedPayloadType(t string) Option {
	return func(c *wrapConfig) {
		c.expectedPayloadType = t
	}
}

func applyWrapOpts(opts []Option) wrapConfig {
	var cfg wrapConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}
