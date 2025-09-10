package portbinding

import (
	"sort"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/types/container"
)

type portMapEntry struct {
	port    container.PortRangeProto
	binding container.PortBinding
}

type portMapSorter []portMapEntry

func (s portMapSorter) Len() int      { return len(s) }
func (s portMapSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less sorts the port so that the order is:
// 1. port with larger specified bindings
// 2. larger port
// 3. port with tcp protocol
func (s portMapSorter) Less(i, j int) bool {
	pi, pj := s[i].port, s[j].port
	hpi, hpj := toInt(s[i].binding.HostPort), toInt(s[j].binding.HostPort)
	return hpi > hpj || pi.Int() > pj.Int() || (pi.Int() == pj.Int() && strings.ToLower(pi.Proto()) == "tcp")
}

// SortPortMap sorts the list of ports and their respected mapping. The ports
// will explicit HostPort will be placed first.
func SortPortMap(ports []container.PortRangeProto, bindings container.PortMap) {
	s := portMapSorter{}
	for _, p := range ports {
		if binding, ok := bindings[p]; ok && len(binding) > 0 {
			for _, b := range binding {
				s = append(s, portMapEntry{port: p, binding: b})
			}
			bindings[p] = []container.PortBinding{}
		} else {
			s = append(s, portMapEntry{port: p})
		}
	}

	sort.Sort(s)
	var (
		i  int
		pm = make(map[container.PortRangeProto]struct{})
	)
	// reorder ports
	for _, entry := range s {
		if _, ok := pm[entry.port]; !ok {
			ports[i] = entry.port
			pm[entry.port] = struct{}{}
			i++
		}
		// reorder bindings for this port
		if _, ok := bindings[entry.port]; ok {
			bindings[entry.port] = append(bindings[entry.port], entry.binding)
		}
	}
}

func toInt(s string) uint64 {
	i, _, err := nat.ParsePortRange(s)
	if err != nil {
		i = 0
	}
	return i
}
