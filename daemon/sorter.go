package daemon

import "sort"

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
