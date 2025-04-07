//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetSystemCPUUsageParsing(t *testing.T) {
	dummyFilePath := filepath.Join("testdata", "stat")
	expectedCpuUsage := uint64(65647090000000)
	expectedCpuNum := uint32(128)

	origStatPath := procStatPath
	procStatPath = dummyFilePath
	defer func() { procStatPath = origStatPath }()

	_, err := os.Stat(dummyFilePath)
	assert.NilError(t, err)

	cpuUsage, cpuNum, err := getSystemCPUUsage()

	assert.Equal(t, cpuUsage, expectedCpuUsage)
	assert.Equal(t, cpuNum, expectedCpuNum)
	assert.NilError(t, err)
}
