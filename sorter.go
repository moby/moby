package docker

import "sort"

type imageSorter struct {
	images []APIImages
	by     func(i1, i2 *APIImages) bool // Closure used in the Less method.
}

// Len is part of sort.Interface.
func (s *imageSorter) Len() int {
	return len(s.images)
}

// Swap is part of sort.Interface.
func (s *imageSorter) Swap(i, j int) {
	s.images[i], s.images[j] = s.images[j], s.images[i]
}

// Less is part of sort.Interface. It is implemented by calling the "by" closure in the sorter.
func (s *imageSorter) Less(i, j int) bool {
	return s.by(&s.images[i], &s.images[j])
}

// Sort []ApiImages by most recent creation date and tag name.
func sortImagesByCreationAndTag(images []APIImages) {
	creationAndTag := func(i1, i2 *APIImages) bool {
		return i1.Created > i2.Created || (i1.Created == i2.Created && i2.Tag > i1.Tag)
	}

	sorter := &imageSorter{
		images: images,
		by:     creationAndTag}

	sort.Sort(sorter)
}

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

type apiLinkSorter struct {
	links []APILink
	by    func(i, j APILink) bool
}

func (s *apiLinkSorter) Len() int {
	return len(s.links)
}

func (s *apiLinkSorter) Swap(i, j int) {
	s.links[i], s.links[j] = s.links[j], s.links[i]
}

func (s *apiLinkSorter) Less(i, j int) bool {
	return s.by(s.links[i], s.links[j])
}

func sortLinks(links []APILink, predicate func(i, j APILink) bool) {
	s := &apiLinkSorter{links, predicate}
	sort.Sort(s)
}
