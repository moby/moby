package client

func WithFilter(f []string) Filter {
	return Filter(f)
}

type Filter []string

func (f Filter) SetDiskUsageOption(di *DiskUsageInfo) {
	di.Filter = f
}

func (f Filter) SetPruneOption(pi *PruneInfo) {
	pi.Filter = f
}

func (f Filter) SetListWorkersOption(lwi *ListWorkersInfo) {
	lwi.Filter = f
}
