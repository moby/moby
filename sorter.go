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

// Sort []ApiImages by most recent creation date.
func sortImagesByCreation(images []APIImages) {
	creation := func(i1, i2 *APIImages) bool {
		return i1.Created > i2.Created
	}

	sorter := &imageSorter{
		images: images,
		by:     creation}

	sort.Sort(sorter)
}
