package cgroups

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
)

type Path func(subsystem Name) (string, error)

func RootPath(subsysem Name) (string, error) {
	return "/", nil
}

// StaticPath returns a static path to use for all cgroups
func StaticPath(path string) Path {
	return func(_ Name) (string, error) {
		return path, nil
	}
}

// NestedPath will nest the cgroups based on the calling processes cgroup
// placing its child processes inside its own path
func NestedPath(suffix string) Path {
	paths, err := parseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return errorPath(err)
	}
	return existingPath(paths, suffix)
}

// PidPath will return the correct cgroup paths for an existing process running inside a cgroup
// This is commonly used for the Load function to restore an existing container
func PidPath(pid int) Path {
	p := fmt.Sprintf("/proc/%d/cgroup", pid)
	paths, err := parseCgroupFile(p)
	if err != nil {
		return errorPath(errors.Wrapf(err, "parse cgroup file %s", p))
	}
	return existingPath(paths, "")
}

func existingPath(paths map[string]string, suffix string) Path {
	// localize the paths based on the root mount dest for nested cgroups
	for n, p := range paths {
		dest, err := getCgroupDestination(string(n))
		if err != nil {
			return errorPath(err)
		}
		rel, err := filepath.Rel(dest, p)
		if err != nil {
			return errorPath(err)
		}
		if rel == "." {
			rel = dest
		}
		paths[n] = filepath.Join("/", rel)
	}
	return func(name Name) (string, error) {
		root, ok := paths[string(name)]
		if !ok {
			if root, ok = paths[fmt.Sprintf("name=%s", name)]; !ok {
				return "", fmt.Errorf("unable to find %q in controller set", name)
			}
		}
		if suffix != "" {
			return filepath.Join(root, suffix), nil
		}
		return root, nil
	}
}

func subPath(path Path, subName string) Path {
	return func(name Name) (string, error) {
		p, err := path(name)
		if err != nil {
			return "", err
		}
		return filepath.Join(p, subName), nil
	}
}

func errorPath(err error) Path {
	return func(_ Name) (string, error) {
		return "", err
	}
}
