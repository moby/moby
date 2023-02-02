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
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/containerd/nydus-snapshotter/pkg/errdefs"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var logger = logrus.WithField("module", "builder")

type PackOption struct {
	BuilderPath string

	BootstrapPath    string
	BlobPath         string
	FsVersion        string
	SourcePath       string
	ChunkDictPath    string
	PrefetchPatterns string
	Compressor       string
	Timeout          *time.Duration
}

type MergeOption struct {
	BuilderPath string

	SourceBootstrapPaths []string
	TargetBootstrapPath  string
	ChunkDictPath        string
	PrefetchPatterns     string
	OutputJSONPath       string
	Timeout              *time.Duration
}

type UnpackOption struct {
	BuilderPath   string
	BootstrapPath string
	BlobPath      string
	TarPath       string
	Timeout       *time.Duration
}

type outputJSON struct {
	Blobs []string
}

func Pack(option PackOption) error {
	if option.FsVersion == "" {
		option.FsVersion = "5"
	}

	args := []string{
		"create",
		"--log-level",
		"warn",
		"--prefetch-policy",
		"fs",
		"--blob",
		option.BlobPath,
		"--source-type",
		"directory",
		"--whiteout-spec",
		"none",
		"--fs-version",
		option.FsVersion,
		"--inline-bootstrap",
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
	args = append(args, option.SourcePath)

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}

	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args[:], " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()
	cmd.Stdin = strings.NewReader(option.PrefetchPatterns)

	if err := cmd.Run(); err != nil {
		if errdefs.IsSignalKilled(err) && option.Timeout != nil {
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
	if option.PrefetchPatterns == "" {
		option.PrefetchPatterns = "/"
	}
	args = append(args, option.SourceBootstrapPaths...)

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}
	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args[:], " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()
	cmd.Stdin = strings.NewReader(option.PrefetchPatterns)

	if err := cmd.Run(); err != nil {
		if errdefs.IsSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return nil, errors.Wrap(err, "run merge command")
	}

	outputBytes, err := ioutil.ReadFile(option.OutputJSONPath)
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
	if option.BlobPath != "" {
		args = append(args, "--blob", option.BlobPath)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if option.Timeout != nil {
		ctx, cancel = context.WithTimeout(ctx, *option.Timeout)
		defer cancel()
	}

	logrus.Debugf("\tCommand: %s %s", option.BuilderPath, strings.Join(args[:], " "))

	cmd := exec.CommandContext(ctx, option.BuilderPath, args...)
	cmd.Stdout = logger.Writer()
	cmd.Stderr = logger.Writer()

	if err := cmd.Run(); err != nil {
		if errdefs.IsSignalKilled(err) && option.Timeout != nil {
			logrus.WithError(err).Errorf("fail to run %v %+v, possibly due to timeout %v", option.BuilderPath, args, *option.Timeout)
		} else {
			logrus.WithError(err).Errorf("fail to run %v %+v", option.BuilderPath, args)
		}
		return err
	}

	return nil
}
