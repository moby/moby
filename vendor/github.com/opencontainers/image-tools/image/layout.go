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
	"strings"

	"github.com/opencontainers/image-spec/schema"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func layoutValidate(w walker) error {
	var blobsExist, indexExist, layoutExist bool

	if err := w.walk(func(path string, info os.FileInfo, r io.Reader) error {
		if strings.EqualFold(filepath.Base(path), "blobs") {
			blobsExist = true
			if !info.IsDir() {
				return fmt.Errorf("blobs is not a directory")
			}

			return nil
		}

		if strings.EqualFold(filepath.Base(path), "index.json") {
			indexExist = true
			if info.IsDir() {
				return fmt.Errorf("index.json is a directory")
			}

			var index v1.Index
			buf, err := ioutil.ReadAll(r)
			if err != nil {
				return errors.Wrap(err, "error reading index.json")
			}

			if err := json.Unmarshal(buf, &index); err != nil {
				return errors.Wrap(err, "index.json format mismatch")
			}

			return nil
		}

		if strings.EqualFold(filepath.Base(path), "oci-layout") {
			layoutExist = true
			if info.IsDir() {
				return fmt.Errorf("oci-layout is a directory")
			}

			var imageLayout v1.ImageLayout
			buf, err := ioutil.ReadAll(r)
			if err != nil {
				return errors.Wrap(err, "error reading oci-layout")
			}

			if err := schema.ValidatorMediaTypeLayoutHeader.Validate(bytes.NewReader(buf)); err != nil {
				return errors.Wrap(err, "oci-layout: imageLayout validation failed")
			}

			if err := json.Unmarshal(buf, &imageLayout); err != nil {
				return errors.Wrap(err, "oci-layout format mismatch")
			}

			return nil
		}

		return nil
	}); err != nil {
		return err
	}

	if !blobsExist {
		return fmt.Errorf("image layout must contain blobs directory")
	}

	if !indexExist {
		return fmt.Errorf("image layout must contain index.json file")
	}

	if !layoutExist {
		return fmt.Errorf("image layout must contain oci-layout file")
	}

	return nil
}
