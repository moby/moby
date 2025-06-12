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

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code below was largely copied from golang.org/x/mod@v0.22;
// https://github.com/golang/mod/blob/v0.22.0/internal/lazyregexp/lazyre.go
// with some additional methods added.

// Package lazyregexp is a thin wrapper over regexp, allowing the use of global
// regexp variables without forcing them to be compiled at init.
package lazyregexp

import (
	"os"
	"regexp"
	"strings"
	"sync"
)

// Regexp is a wrapper around [regexp.Regexp], where the underlying regexp will be
// compiled the first time it is needed.
type Regexp struct {
	str  string
	once sync.Once
	rx   *regexp.Regexp
}

func (re *Regexp) re() *regexp.Regexp {
	re.once.Do(re.build)
	return re.rx
}

func (re *Regexp) build() {
	re.rx = regexp.MustCompile(re.str)
	re.str = ""
}

func (re *Regexp) FindStringIndex(s string) (loc []int) {
	return re.re().FindStringIndex(s)
}

func (re *Regexp) FindStringSubmatch(s string) []string {
	return re.re().FindStringSubmatch(s)
}

func (re *Regexp) MatchString(s string) bool {
	return re.re().MatchString(s)
}

func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	return re.re().ReplaceAll(src, repl)
}

func (re *Regexp) String() string {
	return re.re().String()
}

var inTest = len(os.Args) > 0 && strings.HasSuffix(strings.TrimSuffix(os.Args[0], ".exe"), ".test")

// New creates a new lazy regexp, delaying the compiling work until it is first
// needed. If the code is being run as part of tests, the regexp compiling will
// happen immediately.
func New(str string) *Regexp {
	lr := &Regexp{str: str}
	if inTest {
		// In tests, always compile the regexps early.
		lr.re()
	}
	return lr
}
