//go:build windows
// +build windows

/*
 * Copyright (c) 2022. Nydus Developers. All rights reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package converter

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images/converter"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func Pack(ctx context.Context, dest io.Writer, opt PackOption) (io.WriteCloser, error) {
	panic("not implemented")
}

func Merge(ctx context.Context, layers []Layer, dest io.Writer, opt MergeOption) error {
	panic("not implemented")
}

func Unpack(ctx context.Context, ia content.ReaderAt, dest io.Writer, opt UnpackOption) error {
	panic("not implemented")
}

func IsNydusBlobAndExists(ctx context.Context, cs content.Store, desc ocispec.Descriptor) bool {
	panic("not implemented")
}

func IsNydusBlob(ctx context.Context, desc ocispec.Descriptor) bool {
	panic("not implemented")
}

func LayerConvertFunc(opt PackOption) converter.ConvertFunc {
	panic("not implemented")
}

func ConvertHookFunc(opt MergeOption) converter.ConvertHookFunc {
	panic("not implemented")
}

func MergeLayers(ctx context.Context, cs content.Store, descs []ocispec.Descriptor, opt MergeOption) (*ocispec.Descriptor, error) {
	panic("not implemented")
}
