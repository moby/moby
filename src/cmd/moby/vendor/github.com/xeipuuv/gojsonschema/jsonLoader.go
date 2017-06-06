// Copyright 2015 xeipuuv ( https://github.com/xeipuuv )
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// author           xeipuuv
// author-github    https://github.com/xeipuuv
// author-mail      xeipuuv@gmail.com
//
// repository-name  gojsonschema
// repository-desc  An implementation of JSON Schema, based on IETF's draft v4 - Go language.
//
// description		Different strategies to load JSON files.
// 					Includes References (file and HTTP), JSON strings and Go types.
//
// created          01-02-2015

package gojsonschema

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xeipuuv/gojsonreference"
)

var osFS = osFileSystem(os.Open)

// JSON loader interface

type JSONLoader interface {
	JsonSource() interface{}
	LoadJSON() (interface{}, error)
	JsonReference() (gojsonreference.JsonReference, error)
	LoaderFactory() JSONLoaderFactory
}

type JSONLoaderFactory interface {
	New(source string) JSONLoader
}

type DefaultJSONLoaderFactory struct {
}

type FileSystemJSONLoaderFactory struct {
	fs http.FileSystem
}

func (d DefaultJSONLoaderFactory) New(source string) JSONLoader {
	return &jsonReferenceLoader{
		fs:     osFS,
		source: source,
	}
}

func (f FileSystemJSONLoaderFactory) New(source string) JSONLoader {
	return &jsonReferenceLoader{
		fs:     f.fs,
		source: source,
	}
}

// osFileSystem is a functional wrapper for os.Open that implements http.FileSystem.
type osFileSystem func(string) (*os.File, error)

func (o osFileSystem) Open(name string) (http.File, error) {
	return o(name)
}

// JSON Reference loader
// references are used to load JSONs from files and HTTP

type jsonReferenceLoader struct {
	fs     http.FileSystem
	source string
}

func (l *jsonReferenceLoader) JsonSource() interface{} {
	return l.source
}

func (l *jsonReferenceLoader) JsonReference() (gojsonreference.JsonReference, error) {
	return gojsonreference.NewJsonReference(l.JsonSource().(string))
}

func (l *jsonReferenceLoader) LoaderFactory() JSONLoaderFactory {
	return &FileSystemJSONLoaderFactory{
		fs: l.fs,
	}
}

// NewReferenceLoader returns a JSON reference loader using the given source and the local OS file system.
func NewReferenceLoader(source string) *jsonReferenceLoader {
	return &jsonReferenceLoader{
		fs:     osFS,
		source: source,
	}
}

// NewReferenceLoaderFileSystem returns a JSON reference loader using the given source and file system.
func NewReferenceLoaderFileSystem(source string, fs http.FileSystem) *jsonReferenceLoader {
	return &jsonReferenceLoader{
		fs:     fs,
		source: source,
	}
}

func (l *jsonReferenceLoader) LoadJSON() (interface{}, error) {

	var err error

	reference, err := gojsonreference.NewJsonReference(l.JsonSource().(string))
	if err != nil {
		return nil, err
	}

	refToUrl := reference
	refToUrl.GetUrl().Fragment = ""

	var document interface{}

	if reference.HasFileScheme {

		filename := strings.Replace(refToUrl.GetUrl().Path, "file://", "", -1)
		if runtime.GOOS == "windows" {
			// on Windows, a file URL may have an extra leading slash, use slashes
			// instead of backslashes, and have spaces escaped
			if strings.HasPrefix(filename, "/") {
				filename = filename[1:]
			}
			filename = filepath.FromSlash(filename)
		}

		document, err = l.loadFromFile(filename)
		if err != nil {
			return nil, err
		}

	} else {

		document, err = l.loadFromHTTP(refToUrl.String())
		if err != nil {
			return nil, err
		}

	}

	return document, nil

}

func (l *jsonReferenceLoader) loadFromHTTP(address string) (interface{}, error) {

	resp, err := http.Get(address)
	if err != nil {
		return nil, err
	}

	// must return HTTP Status 200 OK
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(formatErrorDescription(Locale.HttpBadStatus(), ErrorDetails{"status": resp.Status}))
	}

	bodyBuff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(bodyBuff))

}

func (l *jsonReferenceLoader) loadFromFile(path string) (interface{}, error) {
	f, err := l.fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bodyBuff, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(bodyBuff))

}

// JSON string loader

type jsonStringLoader struct {
	source string
}

func (l *jsonStringLoader) JsonSource() interface{} {
	return l.source
}

func (l *jsonStringLoader) JsonReference() (gojsonreference.JsonReference, error) {
	return gojsonreference.NewJsonReference("#")
}

func (l *jsonStringLoader) LoaderFactory() JSONLoaderFactory {
	return &DefaultJSONLoaderFactory{}
}

func NewStringLoader(source string) *jsonStringLoader {
	return &jsonStringLoader{source: source}
}

func (l *jsonStringLoader) LoadJSON() (interface{}, error) {

	return decodeJsonUsingNumber(strings.NewReader(l.JsonSource().(string)))

}

// JSON bytes loader

type jsonBytesLoader struct {
	source []byte
}

func (l *jsonBytesLoader) JsonSource() interface{} {
	return l.source
}

func (l *jsonBytesLoader) JsonReference() (gojsonreference.JsonReference, error) {
	return gojsonreference.NewJsonReference("#")
}

func (l *jsonBytesLoader) LoaderFactory() JSONLoaderFactory {
	return &DefaultJSONLoaderFactory{}
}

func NewBytesLoader(source []byte) *jsonBytesLoader {
	return &jsonBytesLoader{source: source}
}

func (l *jsonBytesLoader) LoadJSON() (interface{}, error) {
	return decodeJsonUsingNumber(bytes.NewReader(l.JsonSource().([]byte)))
}

// JSON Go (types) loader
// used to load JSONs from the code as maps, interface{}, structs ...

type jsonGoLoader struct {
	source interface{}
}

func (l *jsonGoLoader) JsonSource() interface{} {
	return l.source
}

func (l *jsonGoLoader) JsonReference() (gojsonreference.JsonReference, error) {
	return gojsonreference.NewJsonReference("#")
}

func (l *jsonGoLoader) LoaderFactory() JSONLoaderFactory {
	return &DefaultJSONLoaderFactory{}
}

func NewGoLoader(source interface{}) *jsonGoLoader {
	return &jsonGoLoader{source: source}
}

func (l *jsonGoLoader) LoadJSON() (interface{}, error) {

	// convert it to a compliant JSON first to avoid types "mismatches"

	jsonBytes, err := json.Marshal(l.JsonSource())
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(jsonBytes))

}

type jsonIOLoader struct {
	buf *bytes.Buffer
}

func NewReaderLoader(source io.Reader) (*jsonIOLoader, io.Reader) {
	buf := &bytes.Buffer{}
	return &jsonIOLoader{buf: buf}, io.TeeReader(source, buf)
}

func NewWriterLoader(source io.Writer) (*jsonIOLoader, io.Writer) {
	buf := &bytes.Buffer{}
	return &jsonIOLoader{buf: buf}, io.MultiWriter(source, buf)
}

func (l *jsonIOLoader) JsonSource() interface{} {
	return l.buf.String()
}

func (l *jsonIOLoader) LoadJSON() (interface{}, error) {
	return decodeJsonUsingNumber(l.buf)
}

func (l *jsonIOLoader) JsonReference() (gojsonreference.JsonReference, error) {
	return gojsonreference.NewJsonReference("#")
}

func (l *jsonIOLoader) LoaderFactory() JSONLoaderFactory {
	return &DefaultJSONLoaderFactory{}
}

func decodeJsonUsingNumber(r io.Reader) (interface{}, error) {

	var document interface{}

	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	err := decoder.Decode(&document)
	if err != nil {
		return nil, err
	}

	return document, nil

}
