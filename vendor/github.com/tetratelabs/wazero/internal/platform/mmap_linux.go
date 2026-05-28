package platform

import (
	"math/bits"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

var hugePagesConfigs []hugePagesConfig

type hugePagesConfig struct {
	size int
	flag int
}

func (hpc *hugePagesConfig) match(size int) bool {
	return (size & (hpc.size - 1)) == 0
}

func init() {
	dirents, err := os.ReadDir("/sys/kernel/mm/hugepages/")
	if err != nil {
		return
	}

	for _, dirent := range dirents {
		name := dirent.Name()
		if !strings.HasPrefix(name, "hugepages-") {
			continue
		}
		if !strings.HasSuffix(name, "kB") {
			continue
		}
		n, err := strconv.ParseUint(name[10:len(name)-2], 10, 64)
		if err != nil {
			continue
		}
		if bits.OnesCount64(n) != 1 {
			continue
		}
		n *= 1024
		hugePagesConfigs = append(hugePagesConfigs, hugePagesConfig{
			size: int(n),
			flag: int(bits.TrailingZeros64(n)<<unix.MAP_HUGE_SHIFT) | unix.MAP_HUGETLB,
		})
	}

	sort.Slice(hugePagesConfigs, func(i, j int) bool {
		return hugePagesConfigs[i].size > hugePagesConfigs[j].size
	})
}

func mmapCodeSegment(size int) ([]byte, error) {
	flag := unix.MAP_ANON | unix.MAP_PRIVATE
	prot := unix.PROT_READ | unix.PROT_WRITE

	for _, hugePagesConfig := range hugePagesConfigs {
		if hugePagesConfig.match(size) {
			b, err := unix.Mmap(-1, 0, size, prot, flag|hugePagesConfig.flag)
			if err != nil {
				continue
			}
			return b, nil
		}
	}

	return unix.Mmap(-1, 0, size, prot, flag)
}
