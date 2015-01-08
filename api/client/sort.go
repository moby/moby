package client

import "sort"

func sortStatsByName(cStats map[string]containerStats) []containerStats {
	sStats := []containerStats{}
	for _, s := range cStats {
		sStats = append(sStats, s)
	}
	sorter := &statSorter{sStats}
	sort.Sort(sorter)
	return sStats
}

type statSorter struct {
	stats []containerStats
}

func (s *statSorter) Len() int {
	return len(s.stats)
}

func (s *statSorter) Swap(i, j int) {
	s.stats[i], s.stats[j] = s.stats[j], s.stats[i]
}

func (s *statSorter) Less(i, j int) bool {
	return s.stats[i].Name < s.stats[j].Name
}
