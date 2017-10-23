package cgroups

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewHugetlb(root string) (*hugetlbController, error) {
	sizes, err := hugePageSizes()
	if err != nil {
		return nil, err
	}

	return &hugetlbController{
		root:  filepath.Join(root, string(Hugetlb)),
		sizes: sizes,
	}, nil
}

type hugetlbController struct {
	root  string
	sizes []string
}

func (h *hugetlbController) Name() Name {
	return Hugetlb
}

func (h *hugetlbController) Path(path string) string {
	return filepath.Join(h.root, path)
}

func (h *hugetlbController) Create(path string, resources *specs.LinuxResources) error {
	if err := os.MkdirAll(h.Path(path), defaultDirPerm); err != nil {
		return err
	}
	for _, limit := range resources.HugepageLimits {
		if err := ioutil.WriteFile(
			filepath.Join(h.Path(path), strings.Join([]string{"hugetlb", limit.Pagesize, "limit_in_bytes"}, ".")),
			[]byte(strconv.FormatUint(limit.Limit, 10)),
			defaultFilePerm,
		); err != nil {
			return err
		}
	}
	return nil
}

func (h *hugetlbController) Stat(path string, stats *Metrics) error {
	for _, size := range h.sizes {
		s, err := h.readSizeStat(path, size)
		if err != nil {
			return err
		}
		stats.Hugetlb = append(stats.Hugetlb, s)
	}
	return nil
}

func (h *hugetlbController) readSizeStat(path, size string) (*HugetlbStat, error) {
	s := HugetlbStat{
		Pagesize: size,
	}
	for _, t := range []struct {
		name  string
		value *uint64
	}{
		{
			name:  "usage_in_bytes",
			value: &s.Usage,
		},
		{
			name:  "max_usage_in_bytes",
			value: &s.Max,
		},
		{
			name:  "failcnt",
			value: &s.Failcnt,
		},
	} {
		v, err := readUint(filepath.Join(h.Path(path), strings.Join([]string{"hugetlb", size, t.name}, ".")))
		if err != nil {
			return nil, err
		}
		*t.value = v
	}
	return &s, nil
}
