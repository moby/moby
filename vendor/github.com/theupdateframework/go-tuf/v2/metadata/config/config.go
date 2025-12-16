// Copyright 2024 The Update Framework Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License
//
// SPDX-License-Identifier: Apache-2.0
//

package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

type UpdaterConfig struct {
	// TUF configuration
	MaxRootRotations   int64
	MaxDelegations     int
	RootMaxLength      int64
	TimestampMaxLength int64
	SnapshotMaxLength  int64
	TargetsMaxLength   int64
	// Updater configuration
	Fetcher               fetcher.Fetcher
	LocalTrustedRoot      []byte
	LocalMetadataDir      string
	LocalTargetsDir       string
	RemoteMetadataURL     string
	RemoteTargetsURL      string
	DisableLocalCache     bool
	PrefixTargetsWithHash bool
	// UnsafeLocalMode only uses the metadata as written on disk
	// if the metadata is incomplete, calling updater.Refresh will fail
	UnsafeLocalMode bool
}

// New creates a new UpdaterConfig instance used by the Updater to
// store configuration
func New(remoteURL string, rootBytes []byte) (*UpdaterConfig, error) {
	// Default URL for target files - <metadata-url>/targets
	targetsURL, err := url.JoinPath(remoteURL, "targets")
	if err != nil {
		return nil, err
	}

	return &UpdaterConfig{
		// TUF configuration
		MaxRootRotations:   256,
		MaxDelegations:     32,
		RootMaxLength:      512000,  // bytes
		TimestampMaxLength: 16384,   // bytes
		SnapshotMaxLength:  2000000, // bytes
		TargetsMaxLength:   5000000, // bytes
		// Updater configuration
		Fetcher:               fetcher.NewDefaultFetcher(), // use the default built-in download fetcher
		LocalTrustedRoot:      rootBytes,                   // trusted root.json
		RemoteMetadataURL:     remoteURL,                   // URL of where the TUF metadata is
		RemoteTargetsURL:      targetsURL,                  // URL of where the target files should be downloaded from
		DisableLocalCache:     false,                       // enable local caching of trusted metadata
		PrefixTargetsWithHash: true,                        // use hash-prefixed target files with consistent snapshots
		UnsafeLocalMode:       false,
	}, nil
}

func (cfg *UpdaterConfig) EnsurePathsExist() error {
	if cfg.DisableLocalCache {
		return nil
	}

	for _, path := range []string{cfg.LocalMetadataDir, cfg.LocalTargetsDir} {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func (cfg *UpdaterConfig) SetDefaultFetcherHTTPClient(client *http.Client) error {
	// Check if the configured fetcher is the default fetcher
	// since we are only configuring a http.Client value for the default fetcher
	df, ok := cfg.Fetcher.(*fetcher.DefaultFetcher)
	if !ok {
		return fmt.Errorf("fetcher is not type fetcher.DefaultFetcher")
	}
	df.SetHTTPClient(client)
	cfg.Fetcher = df
	return nil
}

func (cfg *UpdaterConfig) SetDefaultFetcherTransport(rt http.RoundTripper) error {
	// Check if the configured fetcher is the default fetcher
	// since we are only configuring a Transport value for the default fetcher
	df, ok := cfg.Fetcher.(*fetcher.DefaultFetcher)
	if !ok {
		return fmt.Errorf("fetcher is not type fetcher.DefaultFetcher")
	}
	if err := df.SetTransport(rt); err != nil {
		return err
	}
	cfg.Fetcher = df
	return nil
}

// SetDefaultFetcherRetry sets the constant retry interval and count for the default fetcher
func (cfg *UpdaterConfig) SetDefaultFetcherRetry(retryInterval time.Duration, retryCount uint) error {
	// Check if the configured fetcher is the default fetcher
	// since we are only configuring the retry interval and count for the default fetcher
	df, ok := cfg.Fetcher.(*fetcher.DefaultFetcher)
	if !ok {
		return fmt.Errorf("fetcher is not type fetcher.DefaultFetcher")
	}
	df.SetRetry(retryInterval, retryCount)
	cfg.Fetcher = df
	return nil
}

func (cfg *UpdaterConfig) SetRetryOptions(retryOptions ...backoff.RetryOption) error {
	// Check if the configured fetcher is the default fetcher
	// since we are only configuring retry options for the default fetcher
	df, ok := cfg.Fetcher.(*fetcher.DefaultFetcher)
	if !ok {
		return fmt.Errorf("fetcher is not type fetcher.DefaultFetcher")
	}
	df.SetRetryOptions(retryOptions...)
	cfg.Fetcher = df
	return nil
}
