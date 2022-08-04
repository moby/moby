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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package config

const (
	// TargetSkipVerifyLabel is a snapshot label key that indicates to skip content
	// verification for the layer.
	TargetSkipVerifyLabel = "containerd.io/snapshot/remote/stargz.skipverify"

	// TargetPrefetchSizeLabel is a snapshot label key that indicates size to prefetch
	// the layer. If the layer is eStargz and contains prefetch landmarks, these config
	// will be respeced.
	TargetPrefetchSizeLabel = "containerd.io/snapshot/remote/stargz.prefetch"
)

type Config struct {
	HTTPCacheType string `toml:"http_cache_type"`
	FSCacheType   string `toml:"filesystem_cache_type"`
	// ResolveResultEntryTTLSec is TTL (in sec) to cache resolved layers for
	// future use. (default 120s)
	ResolveResultEntryTTLSec int   `toml:"resolve_result_entry_ttl_sec"`
	ResolveResultEntry       int   `toml:"resolve_result_entry"` // deprecated
	PrefetchSize             int64 `toml:"prefetch_size"`
	PrefetchTimeoutSec       int64 `toml:"prefetch_timeout_sec"`
	NoPrefetch               bool  `toml:"noprefetch"`
	NoBackgroundFetch        bool  `toml:"no_background_fetch"`
	Debug                    bool  `toml:"debug"`
	AllowNoVerification      bool  `toml:"allow_no_verification"`
	DisableVerification      bool  `toml:"disable_verification"`
	MaxConcurrency           int64 `toml:"max_concurrency"`
	NoPrometheus             bool  `toml:"no_prometheus"`

	// BlobConfig is config for layer blob management.
	BlobConfig `toml:"blob"`

	// DirectoryCacheConfig is config for directory-based cache.
	DirectoryCacheConfig `toml:"directory_cache"`

	FuseConfig `toml:"fuse"`
}

type BlobConfig struct {
	ValidInterval int64 `toml:"valid_interval"`
	CheckAlways   bool  `toml:"check_always"`
	// ChunkSize is the granularity at which background fetch and on-demand reads
	// are fetched from the remote registry.
	ChunkSize            int64 `toml:"chunk_size"`
	FetchTimeoutSec      int64 `toml:"fetching_timeout_sec"`
	ForceSingleRangeMode bool  `toml:"force_single_range_mode"`
	// PrefetchChunkSize is the maximum bytes transferred per http GET from remote registry
	// during prefetch. It is recommended to have PrefetchChunkSize > ChunkSize.
	// If PrefetchChunkSize < ChunkSize prefetch bytes will be fetched as a single http GET,
	// else total GET requests for prefetch = ceil(PrefetchSize / PrefetchChunkSize).
	PrefetchChunkSize int64 `toml:"prefetch_chunk_size"`

	MaxRetries  int `toml:"max_retries"`
	MinWaitMSec int `toml:"min_wait_msec"`
	MaxWaitMSec int `toml:"max_wait_msec"`
}

type DirectoryCacheConfig struct {
	MaxLRUCacheEntry int  `toml:"max_lru_cache_entry"`
	MaxCacheFds      int  `toml:"max_cache_fds"`
	SyncAdd          bool `toml:"sync_add"`
	Direct           bool `toml:"direct" default:"true"`
}

type FuseConfig struct {
	// AttrTimeout defines overall timeout attribute for a file system in seconds.
	AttrTimeout int64 `toml:"attr_timeout"`

	// EntryTimeout defines TTL for directory, name lookup in seconds.
	EntryTimeout int64 `toml:"entry_timeout"`
}
