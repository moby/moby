// +build linux
package cpuhotplug

import (
	"strconv"
	"strings"
	"testing"
)

func TestNewCpuset(t *testing.T) {

	tables := []struct {
		currentCpusset  string
		containerCpuset string
		expectedResult  string
	}{
		{"0-5", "0-5", "0-5"},
		{"1,3", "3", "3"},
		{"1,3,5", "2,4", ""},
		{"1-3,5-8", "0-9", "1-3,5-8"},
		{"1", "9", ""},
		{"0-6", "1,3,5", "1,3,5"},
		{"0-3,5-7,9-13", "0-3,5-7,9-13", "0-3,5-7,9-13"},
		{"0,2-4", "0-2", "0,2"},
		{"0-3", "0-3", "0-3"},
	}
	maxCpu := getMaxCpuNumber() - 1
	for _, table := range tables {
		var cpusetMax, contMax int
		tmp1 := strings.Split(strings.TrimSpace(string(table.currentCpusset)), "-")
		tmp2 := strings.Split(strings.TrimSpace(string(table.containerCpuset)), "-")
		split1 := strings.Split(tmp1[len(tmp1)-1], ",")
		split2 := strings.Split(tmp2[len(tmp2)-1], ",")
		cpusetMax, _ = strconv.Atoi(split1[len(split1)-1])
		contMax, _ = strconv.Atoi(split2[len(split2)-1])

		if cpusetMax >= maxCpu || contMax >= maxCpu {
			continue
		}
		result := NewCpusetRestrictedCont(table.currentCpusset, table.containerCpuset)
		if result != table.expectedResult {
			t.Errorf("NewCpuset of (%s+%s) was incorrect, got: %s, want: %s.", table.currentCpusset, table.containerCpuset, result, table.expectedResult)
		}
	}
}
