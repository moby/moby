package oci

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func FuzzAppendDevicePermissionsFromCgroupRules(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		sp := make([]specs.LinuxDeviceCgroup, 0)
		noOfRecords, err := ff.GetInt()
		if err != nil {
			return
		}
		for i := 0; i < noOfRecords%40; i++ {
			s := specs.LinuxDeviceCgroup{}
			err := ff.GenerateStruct(&s)
			if err != nil {
				return
			}
			sp = append(sp, s)
		}
		rules := make([]string, 0)
		ff.CreateSlice(&rules)
		_, _ = AppendDevicePermissionsFromCgroupRules(sp, rules)
	})
}
