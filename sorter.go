package docker

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

func sortPorts(ports []Port, predicate func(i, j Port) bool) {
	s := &portSorter{ports, predicate}
	sort.Sort(s)
}

type containerSorter struct {
	containers []*Container
	by         func(i, j *Container) bool
}

func (s *containerSorter) Len() int {
	return len(s.containers)
}

func (s *containerSorter) Swap(i, j int) {
	s.containers[i], s.containers[j] = s.containers[j], s.containers[i]
}

func (s *containerSorter) Less(i, j int) bool {
	return s.by(s.containers[i], s.containers[j])
}

func sortContainers(containers []*Container, predicate func(i, j *Container) bool) {
	s := &containerSorter{containers, predicate}
	sort.Sort(s)
}
