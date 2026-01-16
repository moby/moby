//
// Copyright 2021 The Sigstore Authors.
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

package cryptoutils

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// PassFunc is a type of function that takes a boolean (representing whether confirmation is desired) and returns the password as read, along with an error if one occurred
type PassFunc func(bool) ([]byte, error)

// Read is for fuzzing
var Read = readPasswordFn

// readPasswordFn reads the password from the following sources, in order of preference:
//
// - COSIGN_PASSWORD environment variable
//
// - user input from from terminal (if present)
//
// - provided to stdin from pipe
func readPasswordFn() func() ([]byte, error) {
	if pw, ok := os.LookupEnv("COSIGN_PASSWORD"); ok {
		return func() ([]byte, error) {
			return []byte(pw), nil
		}
	}
	if term.IsTerminal(0) {
		return func() ([]byte, error) {
			return term.ReadPassword(0)
		}
	}
	// Handle piped in passwords.
	return func() ([]byte, error) {
		return io.ReadAll(os.Stdin)
	}
}

// StaticPasswordFunc returns a PassFunc which returns the provided password.
func StaticPasswordFunc(pw []byte) PassFunc {
	return func(bool) ([]byte, error) {
		return pw, nil
	}
}

// SkipPassword is a PassFunc that does not interact with a user, but
// simply returns nil for both the password result and error struct.
func SkipPassword(_ bool) ([]byte, error) {
	return nil, nil
}

// GetPasswordFromStdIn gathers the password from stdin with an
// optional confirmation step.
func GetPasswordFromStdIn(confirm bool) ([]byte, error) {
	read := Read()
	fmt.Fprint(os.Stderr, "Enter password for private key: ")
	pw1, err := read()
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	if !confirm {
		return pw1, nil
	}
	fmt.Fprint(os.Stderr, "Enter again: ")
	pw2, err := read()
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}

	if string(pw1) != string(pw2) {
		return nil, errors.New("passwords do not match")
	}
	return pw1, nil
}
