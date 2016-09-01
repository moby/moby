// Copyright 2016 The Linux Foundation
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

package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/opencontainers/image-spec/schema"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func findIndex(w walker, d *v1.Descriptor) (*v1.Index, error) {
	var index v1.Index
	ipath := filepath.Join("blobs", string(d.Digest.Algorithm()), d.Digest.Hex())

	switch err := w.walk(func(path string, info os.FileInfo, r io.Reader) error {
		if info.IsDir() || filepath.Clean(path) != ipath {
			return nil
		}

		buf, err := ioutil.ReadAll(r)
		if err != nil {
			return errors.Wrapf(err, "%s: error reading index", path)
		}

		if err := schema.ValidatorMediaTypeImageIndex.Validate(bytes.NewReader(buf)); err != nil {
			return errors.Wrapf(err, "%s: index validation failed", path)
		}

		if err := json.Unmarshal(buf, &index); err != nil {
			return err
		}

		return errEOW
	}); err {
	case errEOW:
		return &index, nil
	case nil:
		return nil, fmt.Errorf("index not found")
	default:
		return nil, err
	}
}

func validateIndex(index *v1.Index, w walker) error {
	for _, manifest := range index.Manifests {
		if err := validateDescriptor(&manifest, w, []string{v1.MediaTypeImageManifest}); err != nil {
			return errors.Wrap(err, "manifest validation failed")
		}
	}
	return nil
}
