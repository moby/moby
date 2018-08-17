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

package runc

import (
	"bytes"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// ReadPidFile reads the pid file at the provided path and returns
// the pid or an error if the read and conversion is unsuccessful
func ReadPidFile(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(string(data))
}

const exitSignalOffset = 128

// exitStatus returns the correct exit status for a process based on if it
// was signaled or exited cleanly
func exitStatus(status syscall.WaitStatus) int {
	if status.Signaled() {
		return exitSignalOffset + int(status.Signal())
	}
	return status.ExitStatus()
}

var bytesBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(nil)
	},
}

func getBuf() *bytes.Buffer {
	return bytesBufferPool.Get().(*bytes.Buffer)
}

func putBuf(b *bytes.Buffer) {
	b.Reset()
	bytesBufferPool.Put(b)
}

// fieldsASCII is similar to strings.Fields but only allows ASCII whitespaces
func fieldsASCII(s string) []string {
	fn := func(r rune) bool {
		switch r {
		case '\t', '\n', '\f', '\r', ' ':
			return true
		}
		return false
	}
	return strings.FieldsFunc(s, fn)
}

// ParsePSOutput parses the runtime's ps raw output and returns a TopResults
func ParsePSOutput(output []byte) (*TopResults, error) {
	topResults := &TopResults{}

	lines := strings.Split(string(output), "\n")
	topResults.Headers = fieldsASCII(lines[0])

	pidIndex := -1
	for i, name := range topResults.Headers {
		if name == "PID" {
			pidIndex = i
		}
	}

	for _, line := range lines[1:] {
		if len(line) == 0 {
			continue
		}

		fields := fieldsASCII(line)

		if fields[pidIndex] == "-" {
			continue
		}

		process := fields[:len(topResults.Headers)-1]
		process = append(process, strings.Join(fields[len(topResults.Headers)-1:], " "))
		topResults.Processes = append(topResults.Processes, process)

	}
	return topResults, nil
}
