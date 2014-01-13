package lxc

import (
	"testing"
)

const (
	CGROUP_MEMORY_STAT_CONTENTS = `cache 6225920
rss 69144576
mapped_file 253952
swap 0
pgpgin 34273
pgpgout 15872
pgfault 43560
pgmajfault 68
inactive_anon 39628800
active_anon 29687808
inactive_file 5275648
active_file 778240
unevictable 0
hierarchical_memory_limit 9223372036854775807
hierarchical_memsw_limit 9223372036854775807
total_cache 6225920
total_rss 69144576
total_mapped_file 253952
total_swap 0
total_pgpgin 34273
total_pgpgout 15872
total_pgfault 43560
total_pgmajfault 68
total_inactive_anon 39628800
total_active_anon 29687808
total_inactive_file 5275648
total_active_file 778240
total_unevictable 0`

	CGROUP_CPUACCT_USAGE_CONTENTS = `140050479335`
)

func TestParseMemoryFile(t *testing.T) {
	memory, err := parseMemoryStatFile(CGROUP_MEMORY_STAT_CONTENTS)
	if err != nil {
		t.Fatal(err)
	}
	expectedValue := int64(69144576)
	if memory.Rss != expectedValue {
		t.Fatal("Expected rss is %d, actual is %d", expectedValue, memory.Rss)
	}
}

func TestParseCpuAcctUsageFile(t *testing.T) {
	cpuUsage, err := parseCpuAcctUsageFile(CGROUP_CPUACCT_USAGE_CONTENTS)
	if err != nil {
		t.Fatal(err)
	}
	expectedValue := int64(140050479335)
	if cpuUsage != expectedValue {
		t.Fatal("Expected cpuUsage is %d, actual is %d", expectedValue, cpuUsage)
	}
}
