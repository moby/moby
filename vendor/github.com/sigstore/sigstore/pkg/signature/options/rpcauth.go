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

// RPCAuthOpts includes authentication settings for RPC calls
type RPCAuthOpts struct {
	NoOpOptionImpl
	opts RPCAuth
}

// RPCAuth provides credentials for RPC calls, empty fields are ignored
type RPCAuth struct {
	Address string // address is the remote server address, e.g. https://vault:8200
	Path    string // path for the RPC, in vault this is the transit path which default to "transit"
	Token   string // token used for RPC, in vault this is the VAULT_TOKEN value
	OIDC    RPCAuthOIDC
}

// RPCAuthOIDC is used to perform the RPC login using OIDC instead of a fixed token
type RPCAuthOIDC struct {
	Path  string // path defaults to "jwt" for vault
	Role  string // role is required for jwt logins
	Token string // token is a jwt with vault
}

// ApplyRPCAuthOpts sets the RPCAuth as a function option
func (r RPCAuthOpts) ApplyRPCAuthOpts(opts *RPCAuth) {
	if r.opts.Address != "" {
		opts.Address = r.opts.Address
	}
	if r.opts.Path != "" {
		opts.Path = r.opts.Path
	}
	if r.opts.Token != "" {
		opts.Token = r.opts.Token
	}
	if r.opts.OIDC.Token != "" {
		opts.OIDC = r.opts.OIDC
	}
}

// WithRPCAuthOpts specifies RPCAuth settings to be used with RPC logins
func WithRPCAuthOpts(opts RPCAuth) RPCAuthOpts {
	return RPCAuthOpts{opts: opts}
}
