// Copyright 2023 The Sigstore Authors.
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

package tuf

import (
	"embed"
	"math"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore-go/pkg/util"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

//go:embed repository
var embeddedRepo embed.FS

const (
	DefaultMirror = "https://tuf-repo-cdn.sigstore.dev"
	StagingMirror = "https://tuf-repo-cdn.sigstage.dev"

	// The following caching values can be used for the CacheValidity option
	NoCache  = 0
	MaxCache = math.MaxInt
)

// Options represent the various options for a Sigstore TUF Client
type Options struct {
	// CacheValidity period in days (default 0). The client will persist a
	// timestamp with the cache after refresh. Note that the client will
	// always refresh the cache if the metadata is expired or if the client is
	// unable to find a persisted timestamp, so this is not an optimal control
	// for air-gapped environments. Use const MaxCache to update the cache when
	// the metadata is expired, though the first initialization will still
	// refresh the cache.
	CacheValidity int
	// ForceCache controls if the cache should be used without update
	// as long as the metadata is valid. Use ForceCache over CacheValidity
	// if you want to always use the cache up until its expiration. Note that
	// the client will refresh the cache once the metadata has expired, so this
	// is not an optimal control for air-gapped environments. Clients instead
	// should provide a trust root file directly to the client to bypass TUF.
	ForceCache bool
	// Root is the TUF trust anchor
	Root []byte
	// CachePath is the location on disk for TUF cache
	// (default $HOME/.sigstore/tuf)
	CachePath string
	// RepositoryBaseURL is the TUF repository location URL
	// (default https://tuf-repo-cdn.sigstore.dev)
	RepositoryBaseURL string
	// DisableLocalCache mode allows a client to work on a read-only
	// files system if this is set, cache path is ignored.
	DisableLocalCache bool
	// DisableConsistentSnapshot
	DisableConsistentSnapshot bool
	// Fetcher is the metadata fetcher
	Fetcher fetcher.Fetcher
}

// WithCacheValidity sets the cache validity period in days
func (o *Options) WithCacheValidity(days int) *Options {
	o.CacheValidity = days
	return o
}

// WithForceCache forces the client to use the cache without updating
func (o *Options) WithForceCache() *Options {
	o.ForceCache = true
	return o
}

// WithRoot sets the TUF trust anchor
func (o *Options) WithRoot(root []byte) *Options {
	o.Root = root
	return o
}

// WithCachePath sets the location on disk for TUF cache
func (o *Options) WithCachePath(path string) *Options {
	o.CachePath = path
	return o
}

// WithRepositoryBaseURL sets the TUF repository location URL
func (o *Options) WithRepositoryBaseURL(url string) *Options {
	o.RepositoryBaseURL = url
	return o
}

// WithDisableLocalCache sets the client to work on a read-only file system
func (o *Options) WithDisableLocalCache() *Options {
	o.DisableLocalCache = true
	return o
}

// WithDisableConsistentSnapshot sets the client to disable consistent snapshot
func (o *Options) WithDisableConsistentSnapshot() *Options {
	o.DisableConsistentSnapshot = true
	return o
}

// WithFetcher sets the metadata fetcher
func (o *Options) WithFetcher(f fetcher.Fetcher) *Options {
	o.Fetcher = f
	return o
}

// DefaultOptions returns an options struct for the public good instance
func DefaultOptions() *Options {
	var opts Options
	var err error

	opts.Root = DefaultRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to using a TUF repository in the temp location
		home = os.TempDir()
	}
	opts.CachePath = filepath.Join(home, ".sigstore", "root")
	opts.RepositoryBaseURL = DefaultMirror
	fetcher := fetcher.NewDefaultFetcher()
	fetcher.SetHTTPUserAgent(util.ConstructUserAgent())
	opts.Fetcher = fetcher

	return &opts
}

// DefaultRoot returns the root.json for the public good instance
func DefaultRoot() []byte {
	// The embed file system always uses forward slashes as path separators,
	// even on Windows
	p := "repository/root.json"

	b, err := embeddedRepo.ReadFile(p)
	if err != nil {
		// This should never happen.
		// ReadFile from an embedded FS will never fail as long as
		// the path is correct. If it fails, it would mean
		// that the binary is not assembled as it should, and there
		// is no way to recover from that.
		panic(err)
	}

	return b
}

// StagingRoot returns the root.json for the staging instance
func StagingRoot() []byte {
	// The embed file system always uses forward slashes as path separators,
	// even on Windows
	p := "repository/staging_root.json"

	b, err := embeddedRepo.ReadFile(p)
	if err != nil {
		// This should never happen.
		// ReadFile from an embedded FS will never fail as long as
		// the path is correct. If it fails, it would mean
		// that the binary is not assembled as it should, and there
		// is no way to recover from that.
		panic(err)
	}

	return b
}
