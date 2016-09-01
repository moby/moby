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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const indexPath = "index.json"

func listReferences(w walker) (map[string]*v1.Descriptor, error) {
	refs := make(map[string]*v1.Descriptor)
	var index v1.Index

	if err := w.walk(func(path string, info os.FileInfo, r io.Reader) error {
		if info.IsDir() || filepath.Clean(path) != indexPath {
			return nil
		}

		if err := json.NewDecoder(r).Decode(&index); err != nil {
			return err
		}

		for i := 0; i < len(index.Manifests); i++ {
			if index.Manifests[i].Annotations[v1.AnnotationRefName] != "" {
				refs[index.Manifests[i].Annotations[v1.AnnotationRefName]] = &index.Manifests[i]
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}
	return refs, nil
}

func findDescriptor(w walker, name string) (*v1.Descriptor, error) {
	var d v1.Descriptor
	var index v1.Index

	switch err := w.walk(func(path string, info os.FileInfo, r io.Reader) error {
		if info.IsDir() || filepath.Clean(path) != indexPath {
			return nil
		}

		if err := json.NewDecoder(r).Decode(&index); err != nil {
			return err
		}

		for i := 0; i < len(index.Manifests); i++ {
			if index.Manifests[i].Annotations[v1.AnnotationRefName] == name {
				d = index.Manifests[i]
				return errEOW
			}
		}

		return nil
	}); err {
	case nil:
		return nil, fmt.Errorf("index.json: descriptor %q not found", name)
	case errEOW:
		return &d, nil
	default:
		return nil, err
	}
}

func validateDescriptor(d *v1.Descriptor, w walker, mts []string) error {
	var found bool
	for _, mt := range mts {
		if d.MediaType == mt {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid descriptor MediaType %q", d.MediaType)
	}

	if err := d.Digest.Validate(); err != nil {
		return err
	}

	// Copy the contents of the layer in to the verifier
	verifier := d.Digest.Verifier()
	numBytes, err := w.get(*d, verifier)
	if err != nil {
		return err
	}

	if err != nil {
		return errors.Wrap(err, "error generating hash")
	}

	if numBytes != d.Size {
		return errors.New("size mismatch")
	}

	if !verifier.Verified() {
		return errors.New("digest mismatch")
	}

	return nil
}
