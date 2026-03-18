// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
)

// ResourceAttributesGetter abstracts environment lookup methods to query for environment variables, metadata attributes and file content.
type ResourceAttributesGetter interface {
	EnvVar(name string) string
	Metadata(path string) string
	ReadAll(path string) string
}

var getter ResourceAttributesGetter = &defaultResourceGetter{
	metaClient: metadata.NewClient(&http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   1 * time.Second,
				KeepAlive: 10 * time.Second,
			}).Dial,
		},
	})}

// ResourceAttributes provides read-only access to the ResourceAtttributesGetter interface implementation.
func ResourceAttributes() ResourceAttributesGetter {
	return getter
}

type defaultResourceGetter struct {
	metaClient *metadata.Client
}

// EnvVar uses os.LookupEnv() to lookup for environment variable by name.
func (g *defaultResourceGetter) EnvVar(name string) string {
	return os.Getenv(name)
}

// Metadata uses metadata package Client.Get() to lookup for metadata attributes by path.
func (g *defaultResourceGetter) Metadata(path string) string {
	val, err := g.metaClient.Get(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// ReadAll reads all content of the file as a string.
func (g *defaultResourceGetter) ReadAll(path string) string {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bytes)
}
