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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xeipuuv/gojsonreference"
)

// JSON loader interface

type JSONLoader interface {
	jsonSource() interface{}
	loadJSON() (interface{}, error)
	loadSchema() (*Schema, error)
}

// JSON Reference loader
// references are used to load JSONs from files and HTTP

type jsonReferenceLoader struct {
	source string
}

func (l *jsonReferenceLoader) jsonSource() interface{} {
	return l.source
}

func NewReferenceLoader(source string) *jsonReferenceLoader {
	return &jsonReferenceLoader{source: source}
}

func (l *jsonReferenceLoader) loadJSON() (interface{}, error) {

	var err error

	reference, err := gojsonreference.NewJsonReference(l.jsonSource().(string))
	if err != nil {
		return nil, err
	}

	refToUrl := reference
	refToUrl.GetUrl().Fragment = ""

	var document interface{}

	if reference.HasFileScheme {

		filename := strings.Replace(refToUrl.String(), "file://", "", -1)
		if runtime.GOOS == "windows" {
			// on Windows, a file URL may have an extra leading slash, use slashes
			// instead of backslashes, and have spaces escaped
			if strings.HasPrefix(filename, "/") {
				filename = filename[1:]
			}
			filename = filepath.FromSlash(filename)
			filename = strings.Replace(filename, "%20", " ", -1)
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

func (l *jsonReferenceLoader) loadSchema() (*Schema, error) {

	var err error

	d := Schema{}
	d.pool = newSchemaPool()
	d.referencePool = newSchemaReferencePool()

	d.documentReference, err = gojsonreference.NewJsonReference(l.jsonSource().(string))
	if err != nil {
		return nil, err
	}

	spd, err := d.pool.GetDocument(d.documentReference)
	if err != nil {
		return nil, err
	}

	err = d.parse(spd.Document)
	if err != nil {
		return nil, err
	}

	return &d, nil

}

func (l *jsonReferenceLoader) loadFromHTTP(address string) (interface{}, error) {

	resp, err := http.Get(address)
	if err != nil {
		return nil, err
	}

	// must return HTTP Status 200 OK
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(formatErrorDescription(Locale.httpBadStatus(), ErrorDetails{"status": resp.Status}))
	}

	bodyBuff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(bodyBuff))

}

func (l *jsonReferenceLoader) loadFromFile(path string) (interface{}, error) {

	bodyBuff, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(bodyBuff))

}

// JSON string loader

type jsonStringLoader struct {
	source string
}

func (l *jsonStringLoader) jsonSource() interface{} {
	return l.source
}

func NewStringLoader(source string) *jsonStringLoader {
	return &jsonStringLoader{source: source}
}

func (l *jsonStringLoader) loadJSON() (interface{}, error) {

	return decodeJsonUsingNumber(strings.NewReader(l.jsonSource().(string)))

}

func (l *jsonStringLoader) loadSchema() (*Schema, error) {

	var err error

	document, err := l.loadJSON()
	if err != nil {
		return nil, err
	}

	d := Schema{}
	d.pool = newSchemaPool()
	d.referencePool = newSchemaReferencePool()
	d.documentReference, err = gojsonreference.NewJsonReference("#")
	d.pool.SetStandaloneDocument(document)
	if err != nil {
		return nil, err
	}

	err = d.parse(document)
	if err != nil {
		return nil, err
	}

	return &d, nil

}

// JSON Go (types) loader
// used to load JSONs from the code as maps, interface{}, structs ...

type jsonGoLoader struct {
	source interface{}
}

func (l *jsonGoLoader) jsonSource() interface{} {
	return l.source
}

func NewGoLoader(source interface{}) *jsonGoLoader {
	return &jsonGoLoader{source: source}
}

func (l *jsonGoLoader) loadJSON() (interface{}, error) {

	// convert it to a compliant JSON first to avoid types "mismatches"

	jsonBytes, err := json.Marshal(l.jsonSource())
	if err != nil {
		return nil, err
	}

	return decodeJsonUsingNumber(bytes.NewReader(jsonBytes))

}

func (l *jsonGoLoader) loadSchema() (*Schema, error) {

	var err error

	document, err := l.loadJSON()
	if err != nil {
		return nil, err
	}

	d := Schema{}
	d.pool = newSchemaPool()
	d.referencePool = newSchemaReferencePool()
	d.documentReference, err = gojsonreference.NewJsonReference("#")
	d.pool.SetStandaloneDocument(document)
	if err != nil {
		return nil, err
	}

	err = d.parse(document)
	if err != nil {
		return nil, err
	}

	return &d, nil

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
