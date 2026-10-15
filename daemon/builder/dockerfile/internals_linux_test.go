package dockerfile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/moby/moby/v2/daemon/server/buildbackend"
	"github.com/moby/sys/user"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestChownFlagParsing(t *testing.T) {
	testFiles := map[string]string{
		"passwd": `root:x:0:0::/bin:/bin/false
bin:x:1:1::/bin:/bin/false
wwwwww:x:21:33::/bin:/bin/false
unicorn:x:1001:1002::/bin:/bin/false
		`,
		"group": `root:x:0:
bin:x:1:
wwwwww:x:33:
unicorn:x:1002:
somegrp:x:5555:
othergrp:x:6666:
		`,
	}
	// test mappings for validating use of maps
	idMaps := []user.IDMap{
		{
			ID:       0,
			ParentID: 100000,
			Count:    65536,
		},
	}
	remapped := user.IdentityMapping{UIDMaps: idMaps, GIDMaps: idMaps}
	unmapped := user.IdentityMapping{}

	contextDir := t.TempDir()

	if err := os.Mkdir(filepath.Join(contextDir, "etc"), 0o755); err != nil {
		t.Fatalf("error creating test directory: %v", err)
	}

	for filename, content := range testFiles {
		createTestTempFile(t, filepath.Join(contextDir, "etc"), filename, content, 0o644)
	}

	// positive tests
	for _, testcase := range []struct {
		builder   *Builder
		name      string
		chownStr  string
		idMapping user.IdentityMapping
		state     *dispatchState
		expected  identity
	}{
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UIDNoMap",
			chownStr:  "1",
			idMapping: unmapped,
			state:     &dispatchState{},
			expected:  identity{UID: 1, GID: 1},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UIDGIDNoMap",
			chownStr:  "0:1",
			idMapping: unmapped,
			state:     &dispatchState{},
			expected:  identity{UID: 0, GID: 1},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UIDWithMap",
			chownStr:  "0",
			idMapping: remapped,
			state:     &dispatchState{},
			expected:  identity{UID: 100000, GID: 100000},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UIDGIDWithMap",
			chownStr:  "1:33",
			idMapping: remapped,
			state:     &dispatchState{},
			expected:  identity{UID: 100001, GID: 100033},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UserNoMap",
			chownStr:  "bin:5555",
			idMapping: unmapped,
			state:     &dispatchState{},
			expected:  identity{UID: 1, GID: 5555},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "GroupWithMap",
			chownStr:  "0:unicorn",
			idMapping: remapped,
			state:     &dispatchState{},
			expected:  identity{UID: 100000, GID: 101002},
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UserOnlyWithMap",
			chownStr:  "unicorn",
			idMapping: remapped,
			state:     &dispatchState{},
			expected:  identity{UID: 101001, GID: 101002},
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			idPair, err := parseChownFlag(t.Context(), testcase.builder, testcase.state, testcase.chownStr, contextDir, testcase.idMapping)
			assert.NilError(t, err, "Failed to parse chown flag: %q", testcase.chownStr)
			assert.Check(t, is.DeepEqual(testcase.expected, idPair), "chown flag mapping failure")
		})
	}

	// error tests
	for _, testcase := range []struct {
		builder   *Builder
		name      string
		chownStr  string
		idMapping user.IdentityMapping
		state     *dispatchState
		descr     string
	}{
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "BadChownFlagFormat",
			chownStr:  "bob:1:555",
			idMapping: unmapped,
			state:     &dispatchState{},
			descr:     "invalid chown string format: bob:1:555",
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "UserNoExist",
			chownStr:  "bob",
			idMapping: unmapped,
			state:     &dispatchState{},
			descr:     "can't find uid for user bob: no such user: bob",
		},
		{
			builder:   &Builder{options: &buildbackend.BuildOptions{Platform: "linux"}},
			name:      "GroupNoExist",
			chownStr:  "root:bob",
			idMapping: unmapped,
			state:     &dispatchState{},
			descr:     "can't find gid for group bob: no such group: bob",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			_, err := parseChownFlag(t.Context(), testcase.builder, testcase.state, testcase.chownStr, contextDir, testcase.idMapping)
			assert.Check(t, is.Error(err, testcase.descr), "Expected error string doesn't match")
		})
	}
}

// TestCopyFromImageWithIDMapping checks that files copied from another build
// stage or image, which are already owned by host (remapped) IDs, are not
// remapped again.
func TestCopyFromImageWithIDMapping(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")

	idMap := []user.IDMap{{ID: 0, ParentID: 10000, Count: 10000}}
	b := &Builder{idMapping: user.IdentityMapping{UIDMaps: idMap, GIDMaps: idMap}}

	type owner struct{ uid, gid int }
	getOwner := func(t *testing.T, path string) owner {
		t.Helper()
		fi, err := os.Lstat(path)
		assert.NilError(t, err)
		st := fi.Sys().(*syscall.Stat_t)
		return owner{int(st.Uid), int(st.Gid)}
	}

	srcRoot := t.TempDir()
	for _, f := range []struct {
		path  string
		dir   bool
		owner owner
	}{
		{path: "rootfile", owner: owner{10000, 10000}},
		{path: "file", owner: owner{10100, 10200}},
		{path: "dir", dir: true, owner: owner{10100, 10200}},
		{path: "dir/subdir", dir: true, owner: owner{10101, 10201}},
		{path: "dir/subdir/nestedfile", owner: owner{10102, 10202}},
	} {
		p := filepath.Join(srcRoot, f.path)
		if f.dir {
			assert.NilError(t, os.Mkdir(p, 0o755))
		} else {
			assert.NilError(t, os.WriteFile(p, nil, 0o644))
		}
		assert.NilError(t, os.Lchown(p, f.owner.uid, f.owner.gid))
	}

	var (
		root  = owner{10000, 10000}
		chown = owner{10300, 10400}
	)
	tests := []struct {
		name string
		inst copyInstruction
		id   identity
		want map[string]owner
	}{
		{
			name: "preserve ownership",
			inst: copyInstruction{fromImage: true, preserveOwnership: true},
			id:   identity{UID: root.uid, GID: root.gid},
			want: map[string]owner{
				"rootfile":                           root,
				"file":                               {10100, 10200},
				"newdir":                             root,
				"newdir/subdir":                      {10101, 10201},
				"newdir/subdir/nestedfile":           {10102, 10202},
				"newparent":                          root,
				"newparent/newdir":                   root,
				"newparent/newdir/subdir":            {10101, 10201},
				"newparent/newdir/subdir/nestedfile": {10102, 10202},
				"newfileparent":                      root,
				"newfileparent/file":                 {10100, 10200},
			},
		},
		{
			name: "chown",
			inst: copyInstruction{fromImage: true},
			id:   identity{UID: chown.uid, GID: chown.gid},
			want: map[string]owner{
				"rootfile":                           chown,
				"file":                               chown,
				"newdir":                             chown,
				"newdir/subdir":                      chown,
				"newdir/subdir/nestedfile":           chown,
				"newparent":                          root,
				"newparent/newdir":                   chown,
				"newparent/newdir/subdir":            chown,
				"newparent/newdir/subdir/nestedfile": chown,
				"newfileparent":                      chown,
				"newfileparent/file":                 chown,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			destRoot := t.TempDir()
			opts := b.getCopyFileOptions(tc.inst, tc.id)
			copyTo := func(src, dest string) {
				t.Helper()
				err := performCopyForInfo(copyInfo{root: destRoot, path: dest}, copyInfo{root: srcRoot, path: src}, opts)
				assert.NilError(t, err)
			}

			copyTo("rootfile", "/")
			copyTo("file", "/")
			copyTo("dir", "newdir")
			copyTo("dir", "newparent/newdir")
			copyTo("file", "newfileparent/")

			for path, want := range tc.want {
				assert.Check(t, is.Equal(getOwner(t, filepath.Join(destRoot, path)), want), path)
			}
		})
	}
}
