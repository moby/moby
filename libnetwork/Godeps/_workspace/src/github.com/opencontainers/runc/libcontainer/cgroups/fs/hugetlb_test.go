package fs

import (
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	hugetlbUsageContents    = "128\n"
	hugetlbMaxUsageContents = "256\n"
	hugetlbFailcnt          = "100\n"
)

var (
	hugePageSize, _ = cgroups.GetHugePageSize()
	usage           = strings.Join([]string{"hugetlb", hugePageSize[0], "usage_in_bytes"}, ".")
	limit           = strings.Join([]string{"hugetlb", hugePageSize[0], "limit_in_bytes"}, ".")
	maxUsage        = strings.Join([]string{"hugetlb", hugePageSize[0], "max_usage_in_bytes"}, ".")
	failcnt         = strings.Join([]string{"hugetlb", hugePageSize[0], "failcnt"}, ".")
)

func TestHugetlbSetHugetlb(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()

	const (
		hugetlbBefore = 256
		hugetlbAfter  = 512
	)

	helper.writeFileContents(map[string]string{
		limit: strconv.Itoa(hugetlbBefore),
	})

	helper.CgroupData.c.HugetlbLimit = []*configs.HugepageLimit{
		{
			Pagesize: hugePageSize[0],
			Limit:    hugetlbAfter,
		},
	}
	hugetlb := &HugetlbGroup{}
	if err := hugetlb.Set(helper.CgroupPath, helper.CgroupData.c); err != nil {
		t.Fatal(err)
	}

	value, err := getCgroupParamUint(helper.CgroupPath, limit)
	if err != nil {
		t.Fatalf("Failed to parse %s - %s", limit, err)
	}
	if value != hugetlbAfter {
		t.Fatalf("Set hugetlb.limit_in_bytes failed. Expected: %v, Got: %v", hugetlbAfter, value)
	}
}

func TestHugetlbStats(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		usage:    hugetlbUsageContents,
		maxUsage: hugetlbMaxUsageContents,
		failcnt:  hugetlbFailcnt,
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(helper.CgroupPath, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.HugetlbStats{Usage: 128, MaxUsage: 256, Failcnt: 100}
	expectHugetlbStatEquals(t, expectedStats, actualStats.HugetlbStats[hugePageSize[0]])
}

func TestHugetlbStatsNoUsageFile(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		maxUsage: hugetlbMaxUsageContents,
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(helper.CgroupPath, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsNoMaxUsageFile(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		usage: hugetlbUsageContents,
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(helper.CgroupPath, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsBadUsageFile(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		usage:    "bad",
		maxUsage: hugetlbMaxUsageContents,
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(helper.CgroupPath, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsBadMaxUsageFile(t *testing.T) {
	helper := NewCgroupTestUtil("hugetlb", t)
	defer helper.cleanup()
	helper.writeFileContents(map[string]string{
		usage:    hugetlbUsageContents,
		maxUsage: "bad",
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(helper.CgroupPath, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}
