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

import (
	"context"
	"crypto"
	"crypto/rsa"
	"io"
)

// NoOpOptionImpl implements the RPCOption, SignOption, VerifyOption interfaces as no-ops.
type NoOpOptionImpl struct{}

// ApplyContext is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyContext(_ *context.Context) {}

// ApplyCryptoSignerOpts is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyCryptoSignerOpts(_ *crypto.SignerOpts) {}

// ApplyDigest is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyDigest(_ *[]byte) {}

// ApplyRand is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyRand(_ *io.Reader) {}

// ApplyRemoteVerification is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyRemoteVerification(_ *bool) {}

// ApplyRPCAuthOpts is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyRPCAuthOpts(_ *RPCAuth) {}

// ApplyKeyVersion is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyKeyVersion(_ *string) {}

// ApplyKeyVersionUsed is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyKeyVersionUsed(_ **string) {}

// ApplyHash is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyHash(_ *crypto.Hash) {}

// ApplyED25519ph is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyED25519ph(_ *bool) {}

// ApplyRSAPSS is a no-op required to fully implement the requisite interfaces
func (NoOpOptionImpl) ApplyRSAPSS(_ **rsa.PSSOptions) {}
