package overlay

import (
	"strconv"

	"github.com/docker/libnetwork/osl/kernel"
)

var ovConfig = map[string]*kernel.OSValue{
	"net.ipv4.neigh.default.gc_thresh1": {Value: "8192", CheckFn: checkHigher},
	"net.ipv4.neigh.default.gc_thresh2": {Value: "49152", CheckFn: checkHigher},
	"net.ipv4.neigh.default.gc_thresh3": {Value: "65536", CheckFn: checkHigher},
}

func checkHigher(val1, val2 string) bool {
	val1Int, _ := strconv.ParseInt(val1, 10, 32)
	val2Int, _ := strconv.ParseInt(val2, 10, 32)
	return val1Int < val2Int
}

func applyOStweaks() {
	kernel.ApplyOSTweaks(ovConfig)
}
