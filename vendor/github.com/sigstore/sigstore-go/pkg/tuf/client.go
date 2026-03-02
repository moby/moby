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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"

	"github.com/sigstore/sigstore-go/pkg/util"
)

// Client is a Sigstore TUF client
type Client struct {
	cfg  *config.UpdaterConfig
	up   *updater.Updater
	opts *Options
}

// New returns a new client with custom options
func New(opts *Options) (*Client, error) {
	var c = Client{
		opts: opts,
	}
	dir := filepath.Join(opts.CachePath, URLToPath(opts.RepositoryBaseURL))
	var err error

	if c.cfg, err = config.New(opts.RepositoryBaseURL, opts.Root); err != nil {
		return nil, fmt.Errorf("failed to create TUF client: %w", err)
	}

	c.cfg.LocalMetadataDir = dir
	c.cfg.LocalTargetsDir = filepath.Join(dir, "targets")
	c.cfg.DisableLocalCache = c.opts.DisableLocalCache
	c.cfg.PrefixTargetsWithHash = !c.opts.DisableConsistentSnapshot

	if c.cfg.DisableLocalCache {
		c.opts.CachePath = ""
		c.opts.CacheValidity = 0
		c.opts.ForceCache = false
	}

	if opts.Fetcher != nil {
		c.cfg.Fetcher = opts.Fetcher
	} else {
		fetcher := fetcher.NewDefaultFetcher()
		fetcher.SetHTTPUserAgent(util.ConstructUserAgent())
		c.cfg.Fetcher = fetcher
	}

	// Upon client creation, we may not perform a full TUF update,
	// based on the cache control configuration. Start with a local
	// client (only reads content on disk) and then decide if we
	// must perform a full TUF update.
	tmpCfg := *c.cfg
	// Create a temporary config for the first use where UnsafeLocalMode
	// is true. This means that when we first initialize the client,
	// we are guaranteed to only read the metadata on disk.
	// Based on that metadata we take a decision if a full TUF
	// refresh should be done or not. As so, the tmpCfg is only needed
	// here and not in future invocations.
	tmpCfg.UnsafeLocalMode = true
	c.up, err = updater.New(&tmpCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial TUF updater: %w", err)
	}
	if err = c.loadMetadata(); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	return &c, nil
}

// DefaultClient returns a Sigstore TUF client for the public good instance
func DefaultClient() (*Client, error) {
	opts := DefaultOptions()

	return New(opts)
}

// loadMetadata controls if the client actually should perform a TUF refresh.
// The TUF specification mandates so, but for certain Sigstore clients, it
// may be beneficial to rely on the cache, or in air-gapped deployments it
// it may not even be possible.
func (c *Client) loadMetadata() error {
	// Load the metadata into memory and verify it
	if err := c.up.Refresh(); err != nil {
		// this is most likely due to the lack of metadata files
		// on disk. Perform a full update and return.
		return c.Refresh()
	}

	if c.opts.ForceCache {
		return nil
	} else if c.opts.CacheValidity > 0 {
		cfg, err := LoadConfig(c.configPath())
		if err != nil {
			// Config may not exist, don't error
			// create a new empty config
			cfg = &Config{}
		}

		cacheValidUntil := cfg.LastTimestamp.AddDate(0, 0, c.opts.CacheValidity)
		if time.Now().Before(cacheValidUntil) {
			// No need to update
			return nil
		}
	}

	return c.Refresh()
}

func (c *Client) configPath() string {
	var p = filepath.Join(
		c.opts.CachePath,
		fmt.Sprintf("%s.json", URLToPath(c.opts.RepositoryBaseURL)),
	)

	return p
}

// Refresh forces a refresh of the underlying TUF client.
// As the tuf client updater does not support multiple refreshes during
// its life-time, this will replace the TUF client updater with a new one.
func (c *Client) Refresh() error {
	var err error

	c.up, err = updater.New(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to create tuf updater: %w", err)
	}
	err = c.up.Refresh()
	if err != nil {
		return fmt.Errorf("tuf refresh failed: %w", err)
	}
	// If cache is disabled, we don't need to persist the last timestamp
	if c.cfg.DisableLocalCache {
		return nil
	}
	// Update config with last update
	cfg, err := LoadConfig(c.configPath())
	if err != nil {
		// Likely config file did not exit, create it
		cfg = &Config{}
	}
	cfg.LastTimestamp = time.Now()
	// ignore error writing update config file
	_ = cfg.Persist(c.configPath())

	return nil
}

// GetTarget returns a target file from the TUF repository
func (c *Client) GetTarget(target string) ([]byte, error) {
	// Set filepath to the empty string. When we get targets,
	// we rely in the target info struct instead.
	const filePath = ""
	ti, err := c.up.GetTargetInfo(target)
	if err != nil {
		return nil, fmt.Errorf("getting info for target \"%s\": %w", target, err)
	}

	path, tb, err := c.up.FindCachedTarget(ti, filePath)
	if err != nil {
		return nil, fmt.Errorf("getting target cache: %w", err)
	}
	if path != "" {
		// Cached version found
		return tb, nil
	}

	// Download of target is needed
	// Ignore targetsBaseURL, set to empty string
	const targetsBaseURL = ""
	_, tb, err = c.up.DownloadTarget(ti, filePath, targetsBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download target file %s - %w", target, err)
	}

	return tb, nil
}

// URLToPath converts a URL to a filename-compatible string
func URLToPath(url string) string {
	// Strip scheme, replace slashes with dashes
	// e.g. https://github.github.com/prod-tuf-root -> github.github.com-prod-tuf-root
	fn := url
	fn, _ = strings.CutPrefix(fn, "https://")
	fn, _ = strings.CutPrefix(fn, "http://")
	fn = strings.ReplaceAll(fn, "/", "-")
	fn = strings.ReplaceAll(fn, ":", "-")

	return strings.ToLower(fn)
}
