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

// Package cap provides Linux capability utility
package cap

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// FromNumber returns a cap string like "CAP_SYS_ADMIN"
// that corresponds to the given number like 21.
//
// FromNumber returns an empty string for unknown cap number.
func FromNumber(num int) string {
	if num < 0 || num > len(capsLatest)-1 {
		return ""
	}
	return capsLatest[num]
}

// FromBitmap parses an uint64 bitmap into string slice like
// []{"CAP_SYS_ADMIN", ...}.
//
// Unknown cap numbers are returned as []int.
func FromBitmap(v uint64) ([]string, []int) {
	var (
		res     []string
		unknown []int
	)
	for i := 0; i <= 63; i++ {
		if b := (v >> i) & 0x1; b == 0x1 {
			if s := FromNumber(i); s != "" {
				res = append(res, s)
			} else {
				unknown = append(unknown, i)
			}
		}
	}
	return res, unknown
}

// Type is the type of capability
type Type int

const (
	// Effective is CapEff
	Effective Type = 1 << iota
	// Permitted is CapPrm
	Permitted
	// Inheritable is CapInh
	Inheritable
	// Bounding is CapBnd
	Bounding
	// Ambient is CapAmb
	Ambient
)

// ParseProcPIDStatus returns uint64 bitmap value from /proc/<PID>/status file
func ParseProcPIDStatus(r io.Reader) (map[Type]uint64, error) {
	res := make(map[Type]uint64)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		pair := strings.SplitN(line, ":", 2)
		if len(pair) != 2 {
			continue
		}
		k := strings.TrimSpace(pair[0])
		v := strings.TrimSpace(pair[1])
		switch k {
		case "CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb":
			ui64, err := strconv.ParseUint(v, 16, 64)
			if err != nil {
				return nil, errors.Errorf("failed to parse line %q", line)
			}
			switch k {
			case "CapInh":
				res[Inheritable] = ui64
			case "CapPrm":
				res[Permitted] = ui64
			case "CapEff":
				res[Effective] = ui64
			case "CapBnd":
				res[Bounding] = ui64
			case "CapAmb":
				res[Ambient] = ui64
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// Current returns the list of the effective and the known caps of
// the current process.
//
// The result is like []string{"CAP_SYS_ADMIN", ...}.
//
// The result does not contain caps that are not recognized by
// the "github.com/syndtr/gocapability" library.
func Current() ([]string, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	caps, err := ParseProcPIDStatus(f)
	if err != nil {
		return nil, err
	}
	capEff := caps[Effective]
	names, _ := FromBitmap(capEff)
	return names, nil
}

var (
	// caps35 is the caps of kernel 3.5 (37 entries)
	caps35 = []string{
		"CAP_CHOWN",            // 2.2
		"CAP_DAC_OVERRIDE",     // 2.2
		"CAP_DAC_READ_SEARCH",  // 2.2
		"CAP_FOWNER",           // 2.2
		"CAP_FSETID",           // 2.2
		"CAP_KILL",             // 2.2
		"CAP_SETGID",           // 2.2
		"CAP_SETUID",           // 2.2
		"CAP_SETPCAP",          // 2.2
		"CAP_LINUX_IMMUTABLE",  // 2.2
		"CAP_NET_BIND_SERVICE", // 2.2
		"CAP_NET_BROADCAST",    // 2.2
		"CAP_NET_ADMIN",        // 2.2
		"CAP_NET_RAW",          // 2.2
		"CAP_IPC_LOCK",         // 2.2
		"CAP_IPC_OWNER",        // 2.2
		"CAP_SYS_MODULE",       // 2.2
		"CAP_SYS_RAWIO",        // 2.2
		"CAP_SYS_CHROOT",       // 2.2
		"CAP_SYS_PTRACE",       // 2.2
		"CAP_SYS_PACCT",        // 2.2
		"CAP_SYS_ADMIN",        // 2.2
		"CAP_SYS_BOOT",         // 2.2
		"CAP_SYS_NICE",         // 2.2
		"CAP_SYS_RESOURCE",     // 2.2
		"CAP_SYS_TIME",         // 2.2
		"CAP_SYS_TTY_CONFIG",   // 2.2
		"CAP_MKNOD",            // 2.4
		"CAP_LEASE",            // 2.4
		"CAP_AUDIT_WRITE",      // 2.6.11
		"CAP_AUDIT_CONTROL",    // 2.6.11
		"CAP_SETFCAP",          // 2.6.24
		"CAP_MAC_OVERRIDE",     // 2.6.25
		"CAP_MAC_ADMIN",        // 2.6.25
		"CAP_SYSLOG",           // 2.6.37
		"CAP_WAKE_ALARM",       // 3.0
		"CAP_BLOCK_SUSPEND",    // 3.5
	}
	// caps316 is the caps of kernel 3.16 (38 entries)
	caps316 = append(caps35, "CAP_AUDIT_READ")
	// caps58 is the caps of kernel 5.8 (40 entries)
	caps58 = append(caps316, []string{"CAP_PERFMON", "CAP_BPF"}...)
	// caps59 is the caps of kernel 5.9 (41 entries)
	caps59     = append(caps58, "CAP_CHECKPOINT_RESTORE")
	capsLatest = caps59
)

// Known returns the known cap strings of the latest kernel.
// The current latest kernel is 5.9.
func Known() []string {
	return capsLatest
}
