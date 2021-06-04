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

package diff

import (
	"context"
	"io"
	"os"

	"github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/images"
	"github.com/gogo/protobuf/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	handlers []Handler

	// ErrNoProcessor is returned when no stream processor is available for a media-type
	ErrNoProcessor = errors.New("no processor for media-type")
)

func init() {
	// register the default compression handler
	RegisterProcessor(compressedHandler)
}

// RegisterProcessor registers a stream processor for media-types
func RegisterProcessor(handler Handler) {
	handlers = append(handlers, handler)
}

// GetProcessor returns the processor for a media-type
func GetProcessor(ctx context.Context, stream StreamProcessor, payloads map[string]*types.Any) (StreamProcessor, error) {
	// reverse this list so that user configured handlers come up first
	for i := len(handlers) - 1; i >= 0; i-- {
		processor, ok := handlers[i](ctx, stream.MediaType())
		if ok {
			return processor(ctx, stream, payloads)
		}
	}
	return nil, ErrNoProcessor
}

// Handler checks a media-type and initializes the processor
type Handler func(ctx context.Context, mediaType string) (StreamProcessorInit, bool)

// StaticHandler returns the processor init func for a static media-type
func StaticHandler(expectedMediaType string, fn StreamProcessorInit) Handler {
	return func(ctx context.Context, mediaType string) (StreamProcessorInit, bool) {
		if mediaType == expectedMediaType {
			return fn, true
		}
		return nil, false
	}
}

// StreamProcessorInit returns the initialized stream processor
type StreamProcessorInit func(ctx context.Context, stream StreamProcessor, payloads map[string]*types.Any) (StreamProcessor, error)

// RawProcessor provides access to direct fd for processing
type RawProcessor interface {
	// File returns the fd for the read stream of the underlying processor
	File() *os.File
}

// StreamProcessor handles processing a content stream and transforming it into a different media-type
type StreamProcessor interface {
	io.ReadCloser

	// MediaType is the resulting media-type that the processor processes the stream into
	MediaType() string
}

func compressedHandler(ctx context.Context, mediaType string) (StreamProcessorInit, bool) {
	compressed, err := images.DiffCompression(ctx, mediaType)
	if err != nil {
		return nil, false
	}
	if compressed != "" {
		return func(ctx context.Context, stream StreamProcessor, payloads map[string]*types.Any) (StreamProcessor, error) {
			ds, err := compression.DecompressStream(stream)
			if err != nil {
				return nil, err
			}

			return &compressedProcessor{
				rc: ds,
			}, nil
		}, true
	}
	return func(ctx context.Context, stream StreamProcessor, payloads map[string]*types.Any) (StreamProcessor, error) {
		return &stdProcessor{
			rc: stream,
		}, nil
	}, true
}

// NewProcessorChain initialized the root StreamProcessor
func NewProcessorChain(mt string, r io.Reader) StreamProcessor {
	return &processorChain{
		mt: mt,
		rc: r,
	}
}

type processorChain struct {
	mt string
	rc io.Reader
}

func (c *processorChain) MediaType() string {
	return c.mt
}

func (c *processorChain) Read(p []byte) (int, error) {
	return c.rc.Read(p)
}

func (c *processorChain) Close() error {
	return nil
}

type stdProcessor struct {
	rc StreamProcessor
}

func (c *stdProcessor) MediaType() string {
	return ocispec.MediaTypeImageLayer
}

func (c *stdProcessor) Read(p []byte) (int, error) {
	return c.rc.Read(p)
}

func (c *stdProcessor) Close() error {
	return nil
}

type compressedProcessor struct {
	rc io.ReadCloser
}

func (c *compressedProcessor) MediaType() string {
	return ocispec.MediaTypeImageLayer
}

func (c *compressedProcessor) Read(p []byte) (int, error) {
	return c.rc.Read(p)
}

func (c *compressedProcessor) Close() error {
	return c.rc.Close()
}

// BinaryHandler creates a new stream processor handler which calls out to the given binary.
// The id is used to identify the stream processor and allows the caller to send
// payloads specific for that stream processor (i.e. decryption keys for decrypt stream processor).
// The binary will be called for the provided mediaTypes and return the given media type.
func BinaryHandler(id, returnsMediaType string, mediaTypes []string, path string, args, env []string) Handler {
	set := make(map[string]struct{}, len(mediaTypes))
	for _, m := range mediaTypes {
		set[m] = struct{}{}
	}
	return func(_ context.Context, mediaType string) (StreamProcessorInit, bool) {
		if _, ok := set[mediaType]; ok {
			return func(ctx context.Context, stream StreamProcessor, payloads map[string]*types.Any) (StreamProcessor, error) {
				payload := payloads[id]
				return NewBinaryProcessor(ctx, mediaType, returnsMediaType, stream, path, args, env, payload)
			}, true
		}
		return nil, false
	}
}

const mediaTypeEnvVar = "STREAM_PROCESSOR_MEDIATYPE"
