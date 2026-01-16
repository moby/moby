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
	"context"
	"crypto"
	"crypto/rsa"
	"io"

	"github.com/sigstore/sigstore/pkg/signature/options"
)

// RPCOption specifies options to be used when performing RPC
type RPCOption interface {
	ApplyContext(*context.Context)
	ApplyRemoteVerification(*bool)
	ApplyRPCAuthOpts(opts *options.RPCAuth)
	ApplyKeyVersion(keyVersion *string)
}

// PublicKeyOption specifies options to be used when obtaining a public key
type PublicKeyOption interface {
	RPCOption
}

// MessageOption specifies options to be used when processing messages during signing or verification
type MessageOption interface {
	ApplyDigest(*[]byte)
	ApplyCryptoSignerOpts(*crypto.SignerOpts)
}

// SignOption specifies options to be used when signing a message
type SignOption interface {
	RPCOption
	MessageOption
	ApplyRand(*io.Reader)
	ApplyKeyVersionUsed(**string)
}

// VerifyOption specifies options to be used when verifying a signature
type VerifyOption interface {
	RPCOption
	MessageOption
}

// LoadOption specifies options to be used when creating a Signer/Verifier
type LoadOption interface {
	ApplyHash(*crypto.Hash)
	ApplyED25519ph(*bool)
	ApplyRSAPSS(**rsa.PSSOptions)
}
