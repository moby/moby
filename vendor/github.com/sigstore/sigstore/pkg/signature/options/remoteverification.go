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

package options

// RequestRemoteVerification implements the functional option pattern for remotely verifiying signatures when possible
type RequestRemoteVerification struct {
	NoOpOptionImpl
	remoteVerification bool
}

// ApplyRemoteVerification sets remote verification as a functional option
func (r RequestRemoteVerification) ApplyRemoteVerification(remoteVerification *bool) {
	*remoteVerification = r.remoteVerification
}

// WithRemoteVerification specifies that the verification operation should be performed remotely (vs in the process of the caller)
func WithRemoteVerification(remoteVerification bool) RequestRemoteVerification {
	return RequestRemoteVerification{remoteVerification: remoteVerification}
}
