package snapshotter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/mount"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/sys/user"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type fakeMountManager struct {
	mount.Manager
	activateErr   error
	deactivateErr error
	activated     []string
	deactivated   []string
	deactivateCtx error
	labels        map[string]string
	info          mount.ActivationInfo
	infoErr       error
	infoNames     []string
}

func (m *fakeMountManager) Activate(_ context.Context, name string, _ []mount.Mount, opts ...mount.ActivateOpt) (mount.ActivationInfo, error) {
	if m.activateErr != nil {
		return mount.ActivationInfo{}, m.activateErr
	}
	var o mount.ActivateOptions
	for _, opt := range opts {
		opt(&o)
	}
	m.labels = o.Labels
	m.activated = append(m.activated, name)
	return m.info, nil
}

func (m *fakeMountManager) Deactivate(ctx context.Context, name string) error {
	m.deactivateCtx = ctx.Err()
	m.deactivated = append(m.deactivated, name)

	return m.deactivateErr
}

func (m *fakeMountManager) Info(_ context.Context, name string) (mount.ActivationInfo, error) {
	m.infoNames = append(m.infoNames, name)
	return m.info, m.infoErr
}

type stubLeases struct{ leases.Manager }

func (stubLeases) Create(context.Context, ...leases.Opt) (leases.Lease, error) {
	return leases.Lease{ID: "test"}, nil
}

func (stubLeases) Delete(context.Context, leases.Lease, ...leases.DeleteOpt) error { return nil }

// newMounter creates a mounter whose containerd client uses mm as Mount Manager.
func newMounter(t *testing.T, home string, idMap user.IdentityMapping, mm mount.Manager) *refCountMounter {
	t.Helper()

	client, err := containerd.New("", containerd.WithServices(
		containerd.WithMountManager(mm),
		containerd.WithLeasesService(stubLeases{}),
	))
	assert.NilError(t, err)

	return NewMounter(home, "erofs", idMap, client)
}

// currentUserIDMap maps root to the current user so that mounting nothing
// needs no privileges.
func currentUserIDMap() user.IdentityMapping {
	return user.IdentityMapping{
		UIDMaps: []user.IDMap{{ID: 0, ParentID: int64(os.Getuid()), Count: 1}},
		GIDMaps: []user.IDMap{{ID: 0, ParentID: int64(os.Getgid()), Count: 1}},
	}
}

func TestActivate(t *testing.T) {
	errActivate := errors.New("activate failed")
	errInfo := errors.New("info failed")
	rawMounts := []mount.Mount{{Type: "erofs", Source: "/layer.erofs"}}
	systemMounts := []mount.Mount{{Type: "overlay", Source: "overlay", Options: []string{"lowerdir=/a:/b"}}}

	tests := []struct {
		name       string
		mm         fakeMountManager
		wantErr    error
		wantMounts []mount.Mount
		wantInfo   bool
	}{
		{
			name:       "activated",
			mm:         fakeMountManager{info: mount.ActivationInfo{System: systemMounts}},
			wantMounts: systemMounts,
		},
		{
			name:       "already active",
			mm:         fakeMountManager{activateErr: cerrdefs.ErrAlreadyExists, info: mount.ActivationInfo{System: systemMounts}},
			wantMounts: systemMounts,
			wantInfo:   true,
		},
		{
			name:     "already active and info fails",
			mm:       fakeMountManager{activateErr: cerrdefs.ErrAlreadyExists, infoErr: errInfo},
			wantErr:  errInfo,
			wantInfo: true,
		},
		{
			name:    "activate fails",
			mm:      fakeMountManager{activateErr: errActivate},
			wantErr: errActivate,
		},
		{
			name:    "mount manager not implemented",
			mm:      fakeMountManager{activateErr: cerrdefs.ErrNotImplemented},
			wantErr: cerrdefs.ErrNotImplemented,
		},
		{
			name: "all mounts active",
			mm: fakeMountManager{info: mount.ActivationInfo{Active: []mount.ActiveMount{
				{MountPoint: "/active/0"},
				{MountPoint: "/active/1"},
			}}},
			wantMounts: []mount.Mount{{Type: "bind", Source: "/active/1", Options: []string{"rbind"}}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mm := &tc.mm
			m := newMounter(t, t.TempDir(), user.IdentityMapping{}, mm)
			mounts, err := m.activate(t.Context(), "key", rawMounts)
			assert.Check(t, is.Equal(len(mm.infoNames) > 0, tc.wantInfo))

			if tc.wantErr != nil {
				assert.Check(t, is.ErrorIs(err, tc.wantErr))
				return
			}
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual(mounts, tc.wantMounts))
		})
	}
}

func TestDeactivate(t *testing.T) {
	errDeactivate := errors.New("deactivate failed")
	for _, tc := range []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "deactivated"},
		{name: "already deactivated", err: cerrdefs.ErrNotFound},
		{name: "mount manager not implemented", err: cerrdefs.ErrNotImplemented},
		{name: "deactivate fails", err: errDeactivate, wantErr: errDeactivate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := &fakeMountManager{deactivateErr: tc.err}
			m := newMounter(t, t.TempDir(), user.IdentityMapping{}, mm)
			err := m.deactivate(t.Context(), "key")
			assert.Check(t, is.ErrorIs(err, tc.wantErr))
			assert.Check(t, is.DeepEqual(mm.deactivated, []string{"key"}))
		})
	}
}

func TestWithTempMount(t *testing.T) {
	errActivate := errors.New("activate failed")
	errF := errors.New("f failed")
	errDeactivate := errors.New("deactivate failed")

	tests := []struct {
		name           string
		activateErr    error
		fErr           error
		deactivateErr  error
		wantErr        error
		wantCalled     bool
		noDeactivation bool
	}{
		{name: "success", wantCalled: true},
		{name: "activate fails", activateErr: errActivate, wantErr: errActivate},
		{name: "mount manager not implemented", activateErr: cerrdefs.ErrNotImplemented, wantCalled: true, noDeactivation: true},
		{name: "f fails", fErr: errF, wantErr: errF, wantCalled: true},
		{name: "deactivate fails", deactivateErr: errDeactivate, wantCalled: true},
		{name: "f and deactivate fail", fErr: errF, deactivateErr: errDeactivate, wantErr: errF, wantCalled: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mm := &fakeMountManager{activateErr: tc.activateErr, deactivateErr: tc.deactivateErr}
			m := newMounter(t, t.TempDir(), user.IdentityMapping{}, mm)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			called := false
			err := m.WithTempMount(ctx, nil, func(root string) error {
				called = true
				_, err := os.Stat(root)
				assert.NilError(t, err)
				cancel()
				return tc.fErr
			})
			assert.Check(t, is.ErrorIs(err, tc.wantErr))
			assert.Check(t, is.Equal(called, tc.wantCalled))

			if tc.noDeactivation {
				assert.Check(t, is.Len(mm.deactivated, 0))
				return
			}
			// Deactivated even when Activate fails, and with a context that is not cancelled.
			assert.Assert(t, is.Len(mm.deactivated, 1))
			assert.Check(t, is.Contains(mm.deactivated[0], "moby-erofs-tmp-"))
			assert.Check(t, is.Nil(mm.deactivateCtx))

			if tc.activateErr == nil {
				assert.Check(t, is.DeepEqual(mm.activated, mm.deactivated))
			}
		})
	}
}

func TestWithTempMountUniqueKeys(t *testing.T) {
	mm := &fakeMountManager{}

	m := newMounter(t, t.TempDir(), user.IdentityMapping{}, mm)
	for range 2 {
		assert.NilError(t, m.WithReadonlyTempMount(t.Context(), nil, func(string) error { return nil }))
	}
	assert.Check(t, is.Len(mm.activated, 2))
	assert.Check(t, mm.activated[0] != mm.activated[1])
}

func TestRefCountMounterActivation(t *testing.T) {
	mm := &fakeMountManager{deactivateErr: cerrdefs.ErrNotFound}
	m := newMounter(t, t.TempDir(), currentUserIDMap(), mm)
	key := "moby-erofs-sn-ctr"

	target, err := m.Mount(nil, "ctr")
	assert.NilError(t, err)
	again, err := m.Mount(nil, "ctr")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(again, target))
	assert.Check(t, is.DeepEqual(mm.activated, []string{key}))
	assert.Check(t, is.DeepEqual(mm.labels, map[string]string{"containerd.io/gc.bref.snapshot.erofs": "ctr"}))

	assert.NilError(t, m.Unmount(target))
	assert.Check(t, is.Len(mm.deactivated, 0))

	assert.NilError(t, m.Unmount(target))
	assert.Check(t, is.DeepEqual(mm.deactivated, []string{key}))
	_, err = os.Stat(target)
	assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
}

func TestRefCountMounterUnmountDeactivateFails(t *testing.T) {
	errDeactivate := errors.New("deactivate failed")
	mm := &fakeMountManager{deactivateErr: errDeactivate}
	m := newMounter(t, t.TempDir(), currentUserIDMap(), mm)

	target, err := m.Mount(nil, "ctr")
	assert.NilError(t, err)

	err = m.Unmount(target)
	assert.Check(t, is.ErrorIs(err, errDeactivate))
	// The mount point is removed even though deactivation failed.
	_, err = os.Stat(target)
	assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
}

func TestRefCountMounterActivateFails(t *testing.T) {
	errActivate := errors.New("activate failed")
	mm := &fakeMountManager{activateErr: errActivate}
	m := newMounter(t, t.TempDir(), user.IdentityMapping{}, mm)

	_, err := m.Mount(nil, "ctr")
	assert.Check(t, is.ErrorIs(err, errActivate))
	assert.Check(t, is.DeepEqual(mm.deactivated, []string{"moby-erofs-sn-ctr"}))
}

func TestRefCountMounterMountFails(t *testing.T) {
	home := t.TempDir()
	mm := &fakeMountManager{}
	m := newMounter(t, home, currentUserIDMap(), mm)
	key := "moby-erofs-sn-ctr"

	// A file in place of the mount point makes creating it fail.
	file := m.base.target("ctr")
	assert.NilError(t, os.MkdirAll(filepath.Dir(file), 0o700))
	assert.NilError(t, os.WriteFile(file, nil, 0o600))

	_, err := m.Mount(nil, "ctr")
	assert.Check(t, err != nil)
	assert.Check(t, is.DeepEqual(mm.activated, []string{key}))
	assert.Check(t, is.DeepEqual(mm.deactivated, []string{key}))

	// The failed Mount holds no reference, so the next one mounts again.
	target, err := m.Mount(nil, "ctr")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(mm.activated, []string{key, key}))
	_, err = os.Stat(target)
	assert.NilError(t, err)
}

type failingLeases struct{ leases.Manager }

func (failingLeases) Create(context.Context, ...leases.Opt) (leases.Lease, error) {
	return leases.Lease{}, errors.New("unexpected lease")
}

func TestSkipMountManager(t *testing.T) {
	for _, snapshotter := range []string{"overlayfs", "fuse-overlayfs", "native"} {
		t.Run(snapshotter, func(t *testing.T) {
			errUnexpected := errors.New("unexpected call")
			mm := &fakeMountManager{activateErr: errUnexpected, deactivateErr: errUnexpected}
			client, err := containerd.New("", containerd.WithServices(
				containerd.WithMountManager(mm),
				containerd.WithLeasesService(failingLeases{}),
			))
			assert.NilError(t, err)
			m := NewMounter(t.TempDir(), snapshotter, currentUserIDMap(), client)

			target, err := m.Mount(nil, "ctr")
			assert.NilError(t, err)
			assert.NilError(t, m.Unmount(target))
			assert.NilError(t, m.WithTempMount(t.Context(), nil, func(string) error { return nil }))
			assert.Check(t, is.Len(mm.deactivated, 0))
		})
	}
}
