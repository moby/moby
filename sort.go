package nat

import "sort"

type portSorter struct {
	ports []Port
	by    func(i, j Port) bool
}

func (s *portSorter) Len() int {
	return len(s.ports)
}

func (s *portSorter) Swap(i, j int) {
	s.ports[i], s.ports[j] = s.ports[j], s.ports[i]
}

func (s *portSorter) Less(i, j int) bool {
	ip := s.ports[i]
	jp := s.ports[j]

	return s.by(ip, jp)
}

func Sort(ports []Port, predicate func(i, j Port) bool) {
	s := &portSorter{ports, predicate}
	sort.Sort(s)
}
