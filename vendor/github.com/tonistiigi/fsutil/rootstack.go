package fsutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type rootStack struct {
	items []rootStackItem
}

type rootStackItem struct {
	path string
	root Root
}

func newRootStack(root Root) *rootStack {
	return &rootStack{
		items: []rootStackItem{{path: ".", root: root}},
	}
}

func (s *rootStack) get(path string) (Root, string, error) {
	path = cleanRootPath(path)
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	for {
		top := s.items[len(s.items)-1]
		if top.path == dir {
			return top.root, base, nil
		}
		if rel, ok := rootStackRel(top.path, dir); ok {
			osroot, err := top.root.OpenRoot(rel)
			if err != nil {
				return nil, "", errors.WithStack(err)
			}
			root := NewRoot(osroot)
			s.items = append(s.items, rootStackItem{path: dir, root: root})
			return root, base, nil
		}
		if len(s.items) == 1 {
			return nil, "", errors.WithStack(&os.PathError{Op: "openroot", Path: dir, Err: os.ErrNotExist})
		}
		if err := s.pop(); err != nil {
			return nil, "", err
		}
	}
}

func (s *rootStack) Close() error {
	var err error
	for len(s.items) > 1 {
		if err2 := s.pop(); err == nil {
			err = err2
		}
	}
	return err
}

func (s *rootStack) pop() error {
	i := len(s.items) - 1
	item := s.items[i]
	s.items = s.items[:i]
	return item.root.Close()
}

// rootStackRel returns target relative to base when target is below base.
func rootStackRel(base, target string) (string, bool) {
	if base == "." {
		if target == "." {
			return "", false
		}
		return target, true
	}
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}
