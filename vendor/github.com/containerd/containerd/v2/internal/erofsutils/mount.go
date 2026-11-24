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

package erofsutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/opencontainers/go-digest"

	"github.com/containerd/containerd/v2/core/mount"
)

// CacheBlobPath returns the layer content cache path for a diffID, following the
// <dir>/<algo>/<xx>/<hex>.erofs layout, where <xx> is the first two characters of
// the encoded digest. The extra prefix directory shards blobs so no single
// directory grows unwieldy for a large cache. It is the single source of the
// layout, shared by the cache producer (ctr build-erofs-cache) and the erofs
// snapshotter that reads it, so the two never drift.
func CacheBlobPath(dir string, diffID digest.Digest) string {
	enc := diffID.Encoded()
	return filepath.Join(dir, diffID.Algorithm().String(), enc[:2], enc+".erofs")
}

// IsErofsMediaType returns true if the media type is an EROFS layer type.
func IsErofsMediaType(mt string) bool {
	return strings.HasPrefix(mt, "application/vnd.erofs.layer")
}

func mkfsEnv(layerPath string) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	env := os.Environ()
	tmpdir := filepath.Dir(layerPath)
	env = append(env, "TMPDIR="+tmpdir)
	return env
}

func ConvertTarErofs(ctx context.Context, r io.Reader, layerPath, uuid string, mkfsExtraOpts []string) error {
	args := append([]string{"--tar=f", "--aufs", "--quiet", "-Enoinline_data"}, mkfsExtraOpts...)
	if uuid != "" {
		args = append(args, []string{"-U", uuid}...)
	}
	args = append(args, layerPath)
	cmd := exec.CommandContext(ctx, "mkfs.erofs", args...)
	cmd.Stdin = r
	cmd.Env = mkfsEnv(layerPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("erofs apply failed: %s: %w", out, err)
	}
	log.G(ctx).Debugf("running %s %s %v", cmd.Path, cmd.Args, string(out))
	return nil
}

// GenerateTarIndexAndAppendTar calculates tar index using --tar=i option
// and appends the original tar content to create a combined EROFS layer.
//
// The `--tar=i` option instructs mkfs.erofs to only generate the tar index
// for the tar content. The resulting file structure is:
// [Tar index][Original tar content]
func GenerateTarIndexAndAppendTar(ctx context.Context, r io.Reader, layerPath, uuid string, mkfsExtraOpts []string) error {
	// Create a temporary file for storing the tar content
	tarFile, err := os.CreateTemp("", "erofs-tar-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary tar file: %w", err)
	}
	defer os.Remove(tarFile.Name())
	defer tarFile.Close()

	// Use TeeReader to process the input once while saving it to disk
	teeReader := io.TeeReader(r, tarFile)

	// Generate tar index directly to layerPath using --tar=i option
	args := append([]string{"--tar=i", "--aufs", "--quiet"}, mkfsExtraOpts...)
	if uuid != "" {
		args = append(args, []string{"-U", uuid}...)
	}
	args = append(args, layerPath)
	cmd := exec.CommandContext(ctx, "mkfs.erofs", args...)
	cmd.Stdin = teeReader
	cmd.Env = mkfsEnv(layerPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar index generation failed with command 'mkfs.erofs %s': %s: %w",
			strings.Join(args, " "), out, err)
	}
	log.G(ctx).Debugf("running %s %v %s", cmd.Path, cmd.Args, string(out))

	// Open layerPath for appending
	f, err := os.OpenFile(layerPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open layer file for appending: %w", err)
	}
	defer f.Close()

	// Rewind the temporary file
	if _, err := tarFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek to the beginning of tar file: %w", err)
	}

	// Append tar content
	if _, err := io.Copy(f, tarFile); err != nil {
		return fmt.Errorf("failed to append tar to layer: %w", err)
	}

	log.G(ctx).Infof("Successfully generated EROFS layer with tar index and tar content: %s", layerPath)

	return nil
}

// AddDefaultMkfsOpts adds default options for mkfs.erofs
func AddDefaultMkfsOpts(mkfsExtraOpts []string) []string {
	// Check if -b argument is already present
	for _, opt := range mkfsExtraOpts {
		if strings.HasPrefix(opt, "-b") {
			return mkfsExtraOpts
		}
	}

	// Default to a 4K block size so images mount on any page size.
	return append([]string{"-b4096"}, mkfsExtraOpts...)
}

func ConvertErofs(ctx context.Context, layerPath string, srcDir string, mkfsExtraOpts []string) error {
	args := append([]string{"--quiet", "-Enoinline_data"}, mkfsExtraOpts...)
	args = append(args, layerPath, srcDir)
	cmd := exec.CommandContext(ctx, "mkfs.erofs", args...)
	cmd.Env = mkfsEnv(layerPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("erofs apply failed: %s: %w", out, err)
	}
	log.G(ctx).Infof("running %s %s %v", cmd.Path, cmd.Args, string(out))
	return nil
}

// MountsToLayer returns the snapshot layer directory in order to generate
// EROFS-formatted blobs;
//
// The candidate will be checked with ".erofslayer" to make sure this active
// snapshot is really generated by the EROFS snapshotter instead of others.
func MountsToLayer(mounts []mount.Mount) (string, error) {
	var layer string

	// If mount[0].Type is prefixed with "mkfs/", it should be always the snapshot layer
	if strings.HasPrefix(mounts[0].Type, "mkfs/") {
		layer = filepath.Dir(mounts[0].Source)
	} else {
		// Otherwise, let's check the last mount entry
		mnt := mounts[len(mounts)-1]
		mt := strings.Split(mnt.Type, "/")

		switch mt[len(mt)-1] {
		case "bind", "erofs":
			layer = filepath.Dir(mnt.Source)
		case "overlay":
			var topLower string
			for _, o := range mnt.Options {
				if k, v, ok := strings.Cut(o, "="); ok {
					switch k {
					case "upperdir":
						layer = filepath.Dir(v)
					case "lowerdir":
						// Use the first mount source for the top lower layer
						topLower = filepath.Dir(mounts[0].Source)
					}
				}
			}
			if layer == "" {
				if topLower == "" {
					return "", fmt.Errorf("unsupported overlay layer for erofs differ: %w", errdefs.ErrNotImplemented)
				}
				layer = topLower
			}
		default:
			return "", fmt.Errorf("invalid filesystem type %q for erofs differ: %w", mnt.Type, errdefs.ErrNotImplemented)
		}
	}
	// If the layer is not prepared by the EROFS snapshotter, fall back to the next differ
	if _, err := os.Stat(filepath.Join(layer, ".erofslayer")); err != nil {
		return "", fmt.Errorf("mount layer type must be erofs-layer: %w", errdefs.ErrNotImplemented)
	}
	return layer, nil
}

// SupportGenerateFromTar checks if the installed version of mkfs.erofs supports
// the tar mode (--tar option).
func SupportGenerateFromTar() (bool, error) {
	cmd := exec.Command("mkfs.erofs", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to run mkfs.erofs --help: %w", err)
	}

	return bytes.Contains(output, []byte("--tar=")), nil
}
