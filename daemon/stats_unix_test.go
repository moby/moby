//go:build !windows

package daemon

import (
	_ "embed"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

//go:embed testdata/stat
var statData string

func TestGetSystemCPUUsageParsing(t *testing.T) {
	input := strings.NewReader(statData)
	cpuUsage, cpuNum, _ := readSystemCPUUsage(input)
	assert.Check(t, is.Equal(cpuUsage, uint64(65647090000000)))
	assert.Check(t, is.Equal(cpuNum, uint32(128)))
}
