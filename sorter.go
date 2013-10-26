package docker

import "sort"

type APIPortSorter struct {
	APIPorts []APIPort
	by       func(port1, port2 *APIPort) bool
}

func (portSorter *APIPortSorter) Len() int {
	return len(portSorter.APIPorts)
}

func (portSorter *APIPortSorter) Swap(i, j int) {
	portSorter.APIPorts[i], portSorter.APIPorts[j] = portSorter.APIPorts[j], portSorter.APIPorts[i]
}

func (portSorter *APIPortSorter) Less(i, j int) bool {
	return portSorter.by(&portSorter.APIPorts[i], &portSorter.APIPorts[j])
}

type By func(port1, port2 *APIPort) bool

func (by By) Sort(APIPorts []APIPort) {
	portSorter := &APIPortSorter{
		APIPorts: APIPorts,
		by:       by,
	}
	sort.Sort(portSorter)
}

// APIPortSlice is a type to allow sorting functions on a slice of APIPort
type APIPortSlice []APIPort

func (ports APIPortSlice) sortByPrivatePort() []APIPort {
	privatePort := func(port1, port2 *APIPort) bool {
		return port1.PrivatePort < port2.PrivatePort
	}
	By(privatePort).Sort(ports)
	return ports
}

func (ports APIPortSlice) sortByPublicPort() []APIPort {
	publicPort := func(port1, port2 *APIPort) bool {
		return port1.PublicPort < port2.PublicPort
	}
	By(publicPort).Sort(ports)
	return ports
}

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
		return i1.Created > i2.Created
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
