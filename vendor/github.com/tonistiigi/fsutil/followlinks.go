package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	strings "strings"

	"github.com/pkg/errors"
)

func FollowLinks(root string, paths []string) ([]string, error) {
	r := &symlinkResolver{root: root, resolved: map[string]struct{}{}}
	for _, p := range paths {
		if err := r.append(p); err != nil {
			return nil, err
		}
	}
	res := make([]string, 0, len(r.resolved))
	for r := range r.resolved {
		res = append(res, filepath.ToSlash(r))
	}
	sort.Strings(res)
	return dedupePaths(res), nil
}

type symlinkResolver struct {
	root     string
	resolved map[string]struct{}
}

func (r *symlinkResolver) append(p string) error {
	if runtime.GOOS == "windows" && filepath.IsAbs(filepath.FromSlash(p)) {
		absParts := strings.SplitN(p, ":", 2)
		if len(absParts) == 2 {
			p = absParts[1]
		}
	}
	p = filepath.Join(".", p)
	current := "."
	for {
		parts := strings.SplitN(p, string(filepath.Separator), 2)
		current = filepath.Join(current, parts[0])

		targets, err := r.readSymlink(current, true)
		if err != nil {
			return err
		}
		p = ""
		if len(parts) == 2 {
			p = parts[1]
		}

		if p == "" || targets != nil {
			if _, ok := r.resolved[current]; ok {
				return nil
			}
		}

		if targets != nil {
			r.resolved[current] = struct{}{}
			for _, target := range targets {
				if err := r.append(filepath.Join(target, p)); err != nil {
					return err
				}
			}
			return nil
		}

		if p == "" {
			r.resolved[current] = struct{}{}
			return nil
		}
	}
}

func (r *symlinkResolver) readSymlink(p string, allowWildcard bool) ([]string, error) {
	realPath := filepath.Join(r.root, p)
	base := filepath.Base(p)
	if allowWildcard && containsWildcards(base) {
		fis, err := os.ReadDir(filepath.Dir(realPath))
		if err != nil {
			if isNotFound(err) {
				return nil, nil
			}
			return nil, errors.Wrap(err, "readdir")
		}
		var out []string
		for _, f := range fis {
			if ok, _ := filepath.Match(base, f.Name()); ok {
				res, err := r.readSymlink(filepath.Join(filepath.Dir(p), f.Name()), false)
				if err != nil {
					return nil, err
				}
				out = append(out, res...)
			}
		}
		return out, nil
	}

	fi, err := os.Lstat(realPath)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, errors.WithStack(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return nil, nil
	}
	link, err := os.Readlink(realPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	link = filepath.Clean(link)
	if filepath.IsAbs(link) {
		return []string{link}, nil
	}
	return []string{
		filepath.Join(string(filepath.Separator), filepath.Join(filepath.Dir(p), link)),
	}, nil
}

func containsWildcards(name string) bool {
	isWindows := runtime.GOOS == "windows"
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' && !isWindows {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

// dedupePaths expects input as a sorted list
func dedupePaths(in []string) []string {
	out := make([]string, 0, len(in))
	var last string
	for _, s := range in {
		// if one of the paths is root there is no filter
		if s == "." {
			return nil
		}
		if strings.HasPrefix(s, last+"/") {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}
