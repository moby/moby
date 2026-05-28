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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/moby/sys/signal"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// StopSignalLabel is a well-known containerd label for storing the stop
// signal specified in the OCI image config
const StopSignalLabel = "io.containerd.image.config.stop-signal"

// GetStopSignal retrieves the container stop signal, specified by the
// well-known containerd label (StopSignalLabel)
func GetStopSignal(ctx context.Context, container Container, defaultSignal syscall.Signal) (syscall.Signal, error) {
	labels, err := container.Labels(ctx)
	if err != nil {
		return -1, err
	}

	if stopSignal, ok := labels[StopSignalLabel]; ok {
		return signal.ParseSignal(stopSignal)
	}

	return defaultSignal, nil
}

// GetOCIStopSignal retrieves the stop signal specified in the OCI image config
func GetOCIStopSignal(ctx context.Context, image Image, defaultSignal string) (string, error) {
	_, err := signal.ParseSignal(defaultSignal)
	if err != nil {
		return "", err
	}
	ic, err := image.Config(ctx)
	if err != nil {
		return "", err
	}
	if !images.IsConfigType(ic.MediaType) {
		return "", fmt.Errorf("unknown image config media type %s", ic.MediaType)
	}

	var (
		ociimage v1.Image
		config   v1.ImageConfig
	)
	p, err := content.ReadBlob(ctx, image.ContentStore(), ic)
	if err != nil {
		return "", err
	}

	if err = json.Unmarshal(p, &ociimage); err != nil {
		return "", err
	}
	config = ociimage.Config

	if config.StopSignal == "" {
		return defaultSignal, nil
	}

	return config.StopSignal, nil
}
