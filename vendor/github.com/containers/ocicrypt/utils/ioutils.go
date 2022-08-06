/*
   Copyright The ocicrypt Authors.

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

package utils

import (
	"bytes"
	"io"
	"os/exec"
	"github.com/pkg/errors"
)

// FillBuffer fills the given buffer with as many bytes from the reader as possible. It returns
// EOF if an EOF was encountered or any other error.
func FillBuffer(reader io.Reader, buffer []byte) (int, error) {
	n, err := io.ReadFull(reader, buffer)
	if err == io.ErrUnexpectedEOF {
		return n, io.EOF
	}
	return n, err
}

// first argument is the command, like cat or echo,
// the second is the list of args to pass to it
type CommandExecuter interface {
	Exec(string, []string, []byte) ([]byte, error)
}

type Runner struct{}

// ExecuteCommand is used to execute a linux command line command and return the output of the command with an error if it exists.
func (r Runner) Exec(cmdName string, args []string, input []byte) ([]byte, error) {
	var out bytes.Buffer
	stdInputBuffer := bytes.NewBuffer(input)
	cmd := exec.Command(cmdName, args...)
	cmd.Stdin = stdInputBuffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, errors.Wrapf(err, "Error while running command: %s", cmdName)
	}
	return out.Bytes(), nil
}
