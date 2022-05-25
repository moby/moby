//go:build linux
// +build linux

package overlayutils

import (
	"fmt"
	"github.com/containerd/containerd/mount"
	"os"
	"path/filepath"
	"strings"
)

type testMount struct {
	mergedDir string
	workDir   string
	upperDir  string
	lowerDirs []string
}

func makeTestMount(d string, lowerCount int) (*testMount, error) {
	lowerDirs := make([]string, lowerCount)
	for i := 0; i <= lowerCount; i++ {
		lowerDirs[i] = fmt.Sprintf("lower%d", i+1)
	}

	t := &testMount{
		mergedDir: filepath.Join(d, "merged"),
		workDir:   filepath.Join(d, "work"),
		upperDir:  filepath.Join(d, "upper"),
		lowerDirs: lowerDirs,
	}

	for _, dir := range append(t.lowerDirs, t.mergedDir, t.workDir, t.upperDir) {
		if err := os.Mkdir(filepath.Join(d, dir), 0755); err != nil {
			return nil, err
		}
	}

	return t, nil
}

func (tm *testMount) mount(opts []string) error {
	dirOpts := []string{
		fmt.Sprintf("lowerdir=%s", strings.Join(tm.lowerDirs, ":")),
		fmt.Sprintf("upperdir=%s", tm.upperDir),
		fmt.Sprintf("workdir=%s", tm.workDir),
	}

	m := mount.Mount{
		Type:    "overlay",
		Source:  "overlay",
		Options: append(dirOpts, opts...),
	}

	return m.Mount(tm.mergedDir)
}

func (tm *testMount) unmount() error {
	return mount.UnmountAll(tm.mergedDir, 0)
}
