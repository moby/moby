package cgroups

import "path/filepath"

func NewNamed(root string, name Name) *namedController {
	return &namedController{
		root: root,
		name: name,
	}
}

type namedController struct {
	root string
	name Name
}

func (n *namedController) Name() Name {
	return n.name
}

func (n *namedController) Path(path string) string {
	return filepath.Join(n.root, string(n.name), path)
}
