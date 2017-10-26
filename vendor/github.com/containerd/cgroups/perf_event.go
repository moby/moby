package cgroups

import "path/filepath"

func NewPerfEvent(root string) *PerfEventController {
	return &PerfEventController{
		root: filepath.Join(root, string(PerfEvent)),
	}
}

type PerfEventController struct {
	root string
}

func (p *PerfEventController) Name() Name {
	return PerfEvent
}

func (p *PerfEventController) Path(path string) string {
	return filepath.Join(p.root, path)
}
