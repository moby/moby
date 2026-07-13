package oci

import (
	"encoding/binary"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func FuzzAppendDevicePermissionsFromCgroupRules(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, noOfRecords uint8, ruleData []byte) {
		sp := make([]specs.LinuxDeviceCgroup, 0)
		for range noOfRecords % 40 {
			var s specs.LinuxDeviceCgroup
			data = generateDeviceCgroup(data, &s)
			sp = append(sp, s)
			if len(data) == 0 {
				break
			}
		}

		rules := generateRules(ruleData)

		// Exercise AppendDevicePermissionsFromCgroupRules with arbitrary
		// cgroup rules and rule strings. Errors are expected for some inputs;
		// the fuzz target is only checking that it doesn't panic.
		_, _ = AppendDevicePermissionsFromCgroupRules(sp, rules)
	})
}

func generateRules(data []byte) []string {
	const maxRules = 40
	rules := make([]string, 0, min(maxRules, len(data)))
	for len(data) > 0 && len(rules) < maxRules {
		n := int(data[0] % 64)
		data = data[1:]

		if n > len(data) {
			n = len(data)
		}
		rules = append(rules, string(data[:n]))
		data = data[n:]
	}
	return rules
}

func generateDeviceCgroup(data []byte, s *specs.LinuxDeviceCgroup) []byte {
	if len(data) < 14 {
		return nil
	}

	s.Allow = data[0]%2 != 0

	switch data[1] % 5 {
	case 0:
		s.Type = "a"
	case 1:
		s.Type = "b"
	case 2:
		s.Type = "c"
	case 3:
		s.Type = ""
	default:
		s.Type = string(data[1:2])
	}

	if data[2]%2 != 0 {
		v := int64(int32(binary.LittleEndian.Uint32(data[6:10])))
		s.Major = &v
	}
	if data[3]%2 != 0 {
		v := int64(int32(binary.LittleEndian.Uint32(data[10:14])))
		s.Minor = &v
	}

	switch data[4] % 9 {
	case 0:
		s.Access = "r"
	case 1:
		s.Access = "w"
	case 2:
		s.Access = "m"
	case 3:
		s.Access = "rw"
	case 4:
		s.Access = "rm"
	case 5:
		s.Access = "wm"
	case 6:
		s.Access = "rwm"
	case 7:
		s.Access = ""
	default:
		s.Access = string(data[5:6])
	}

	return data[14:]
}
