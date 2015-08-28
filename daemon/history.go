package daemon

import (
	"sort"
)

// History is a convenience type for storing a list of containers,
// ordered by creation date.
type History []*Container

func (history *History) Len() int {
	return len(*history)
}

func (history *History) Less(i, j int) bool {
	containers := *history
	return containers[j].Created.Before(containers[i].Created)
}

func (history *History) Swap(i, j int) {
	containers := *history
	containers[i], containers[j] = containers[j], containers[i]
}

// Add the given container to history.
func (history *History) Add(container *Container) {
	*history = append(*history, container)
}

func (history *History) sort() {
	sort.Sort(history)
}
