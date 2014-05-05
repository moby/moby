package fs

import (
	"testing"
)

func TestCpuStats(t *testing.T) {
	helper := NewCgroupTestUtil("cpu", t)
	defer helper.cleanup()
	cpuStatContent := `nr_periods 2000
	nr_throttled 200
	throttled_time 42424242424`
	helper.writeFileContents(map[string]string{
		"cpu.stat": cpuStatContent,
	})

	cpu := &cpuGroup{}
	stats, err := cpu.Stats(helper.CgroupData)
	if err != nil {
		t.Fatal(err)
	}

	expected_stats := map[string]float64{
		"nr_periods":     2000.0,
		"nr_throttled":   200.0,
		"throttled_time": 42424242424.0,
	}
	expectStats(t, expected_stats, stats)
}

func TestNoCpuStatFile(t *testing.T) {
	helper := NewCgroupTestUtil("cpu", t)
	defer helper.cleanup()

	cpu := &cpuGroup{}
	_, err := cpu.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected to fail, but did not.")
	}
}

func TestInvalidCpuStat(t *testing.T) {
	helper := NewCgroupTestUtil("cpu", t)
	defer helper.cleanup()
	cpuStatContent := `nr_periods 2000
	nr_throttled 200
	throttled_time fortytwo`
	helper.writeFileContents(map[string]string{
		"cpu.stat": cpuStatContent,
	})

	cpu := &cpuGroup{}
	_, err := cpu.Stats(helper.CgroupData)
	if err == nil {
		t.Fatal("Expected failed stat parsing.")
	}
}
