/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var logger = logrus.WithField("module", "builder")

func isSignalKilled(err error) bool {
	return strings.Contains(err.Error(), "signal: killed")
}

type PackOption struct {
	BuilderPath string

	BootstrapPath    string
	BlobPath         string
	FsVersion        string
	SourcePath       string
	ChunkDictPath    string
	PrefetchPatterns string
	Compressor       string
	OCIRef           bool
	AlignedChunk     bool
	ChunkSize        string
	BatchSize        string
	Timeout          *time.Duration

	Features Features
}

type MergeOption struct {
	BuilderPath string

	SourceBootstrapPaths []string
	RafsBlobDigests      []string
	RafsBlobTOCDigests   []string
	RafsBlobSizes        []int64

	TargetBootstrapPath string
	ChunkDictPath       string
	ParentBootstrapPath string
	PrefetchPatterns    string
	OutputJSONPath      string
	Timeout             *time.Duration
}

type UnpackOption struct {
	BuilderPath       string
	BootstrapPath     string
	BlobPath          string
	BackendConfigPath string
	TarPath           string
	Timeout           *time.Duration
}

type outputJSON struct {
	Blobs []string
}

func buildPackArgs(option PackOption) []string {
	if option.FsVersion == "" {
		option.FsVersion = "6"
	}

	args := []string{
		"create",
		"--log-level",
		"warn",
		"--prefetch-policy",
		"fs",
		"--blob",
		option.BlobPath,
		"--whiteout-spec",
		"none",
		"--fs-version",
		option.FsVersion,
	}

	if option.Features.Contains(FeatureTar2Rafs) {
		args = append(
			args,
			"--type",
			"tar-rafs",
			"--blob-inline-meta",
		)
		if option.FsVersion == "6" {
			args = append(
				args,
				"--features",
				"blob-toc",
			)
		}
	} else {
		args = append(
			args,
			"--source-type",
			"directory",
			// Sames with `--blob-inline-meta`, it's used for compatibility
			// with the old nydus-image builder.
			"--inline-bootstrap",
		)
	}

	if option.ChunkDictPath != "" {
		args = append(args, "--chunk-dict", fmt.Sprintf("bootstrap=%s", option.ChunkDictPath))
	}
	if option.PrefetchPatterns == "" {
		option.PrefetchPatterns = "/"
	}
	if option.Compressor != "" {
		args = append(args, "--compressor", option.Compressor)
	}
	if option.AlignedChunk {
		args = append(args, "--aligned-chunk")
	}
	if option.ChunkSize != "" {
		args = append(args, "--chunk-size", option.ChunkSize)
	}
	if option.BatchSize != "" {
		args = append(args, "--batch-size", option.BatchSize)
	}
	args = append(args, option.SourcePath)

	return args
}

func Pack(option PackOption) error {
	if option.OCIRef {
		return packRef(option)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}

	args := buildPackArgs(option)
	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()
	cmd.Stdin = strings.NewReader(option.PrefetchPatterns)

	if err := cmd.Run(); err != nil {
		if isSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return err
	}

	return nil
}

func packRef(option PackOption) error {
	args := []string{
		"create",
		"--log-level",
		"warn",
		"--type",
		"targz-ref",
		"--blob-inline-meta",
		"--features",
		"blob-toc",
		"--blob",
		option.BlobPath,
	}
	args = append(args, option.SourcePath)

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}

	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()

	if err := cmd.Run(); err != nil {
		if isSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return err
	}

	return nil
}

func Merge(option MergeOption) ([]digest.Digest, error) {
	args := []string{
		"merge",
		"--log-level",
		"warn",
		"--prefetch-policy",
		"fs",
		"--output-json",
		option.OutputJSONPath,
		"--bootstrap",
		option.TargetBootstrapPath,
	}
	if option.ChunkDictPath != "" {
		args = append(args, "--chunk-dict", fmt.Sprintf("bootstrap=%s", option.ChunkDictPath))
	}
	if option.ParentBootstrapPath != "" {
		args = append(args, "--parent-bootstrap", option.ParentBootstrapPath)
	}
	if option.PrefetchPatterns == "" {
		option.PrefetchPatterns = "/"
	}
	args = append(args, option.SourceBootstrapPaths...)
	if len(option.RafsBlobDigests) > 0 {
		args = append(args, "--blob-digests", strings.Join(option.RafsBlobDigests, ","))
	}
	if len(option.RafsBlobTOCDigests) > 0 {
		args = append(args, "--blob-toc-digests", strings.Join(option.RafsBlobTOCDigests, ","))
	}
	if len(option.RafsBlobSizes) > 0 {
		sizes := []string{}
		for _, size := range option.RafsBlobSizes {
			sizes = append(sizes, fmt.Sprintf("%d", size))
		}
		args = append(args, "--blob-sizes", strings.Join(sizes, ","))
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}
	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()
	cmd.Stdin = strings.NewReader(option.PrefetchPatterns)

	if err := cmd.Run(); err != nil {
		if isSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return nil, errors.Wrap(err, "run merge command")
	}

	outputBytes, err := os.ReadFile(option.OutputJSONPath)
	if err != nil {
		return nil, errors.Wrapf(err, "read file %s", option.OutputJSONPath)
	}
	var output outputJSON
	err = json.Unmarshal(outputBytes, &output)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshal output json file %s", option.OutputJSONPath)
	}

	blobDigests := []digest.Digest{}
	for _, blobID := range output.Blobs {
		blobDigests = append(blobDigests, digest.NewDigestFromHex(string(digest.SHA256), blobID))
	}

	return blobDigests, nil
}

func Unpack(option UnpackOption) error {
	args := []string{
		"unpack",
		"--log-level",
		"warn",
		"--bootstrap",
		option.BootstrapPath,
		"--output",
		option.TarPath,
	}

	if option.BackendConfigPath != "" {
		args = append(args, "--backend-config", option.BackendConfigPath)
	} else if option.BlobPath != "" {
		args = append(args, "--blob", option.BlobPath)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}

	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()

	if err := cmd.Run(); err != nil {
		if isSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return err
	}

	return nil
}
