// +build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/continuity/fs"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/user"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/syndtr/gocapability/capability"
)

// WithTTY sets the information on the spec as well as the environment variables for
// using a TTY
func WithTTY(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setProcess(s)
	s.Process.Terminal = true
	s.Process.Env = append(s.Process.Env, "TERM=xterm")
	return nil
}

// setRoot sets Root to empty if unset
func setRoot(s *Spec) {
	if s.Root == nil {
		s.Root = &specs.Root{}
	}
}

// setLinux sets Linux to empty if unset
func setLinux(s *Spec) {
	if s.Linux == nil {
		s.Linux = &specs.Linux{}
	}
}

// setCapabilities sets Linux Capabilities to empty if unset
func setCapabilities(s *Spec) {
	setProcess(s)
	if s.Process.Capabilities == nil {
		s.Process.Capabilities = &specs.LinuxCapabilities{}
	}
}

// WithHostNamespace allows a task to run inside the host's linux namespace
func WithHostNamespace(ns specs.LinuxNamespaceType) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setLinux(s)
		for i, n := range s.Linux.Namespaces {
			if n.Type == ns {
				s.Linux.Namespaces = append(s.Linux.Namespaces[:i], s.Linux.Namespaces[i+1:]...)
				return nil
			}
		}
		return nil
	}
}

// WithLinuxNamespace uses the passed in namespace for the spec. If a namespace of the same type already exists in the
// spec, the existing namespace is replaced by the one provided.
func WithLinuxNamespace(ns specs.LinuxNamespace) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setLinux(s)
		for i, n := range s.Linux.Namespaces {
			if n.Type == ns.Type {
				before := s.Linux.Namespaces[:i]
				after := s.Linux.Namespaces[i+1:]
				s.Linux.Namespaces = append(before, ns)
				s.Linux.Namespaces = append(s.Linux.Namespaces, after...)
				return nil
			}
		}
		s.Linux.Namespaces = append(s.Linux.Namespaces, ns)
		return nil
	}
}

// WithImageConfig configures the spec to from the configuration of an Image
func WithImageConfig(image Image) SpecOpts {
	return WithImageConfigArgs(image, nil)
}

// WithImageConfigArgs configures the spec to from the configuration of an Image with additional args that
// replaces the CMD of the image
func WithImageConfigArgs(image Image, args []string) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
		ic, err := image.Config(ctx)
		if err != nil {
			return err
		}
		var (
			ociimage v1.Image
			config   v1.ImageConfig
		)
		switch ic.MediaType {
		case v1.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
			p, err := content.ReadBlob(ctx, image.ContentStore(), ic)
			if err != nil {
				return err
			}

			if err := json.Unmarshal(p, &ociimage); err != nil {
				return err
			}
			config = ociimage.Config
		default:
			return fmt.Errorf("unknown image config media type %s", ic.MediaType)
		}

		setProcess(s)
		s.Process.Env = append(s.Process.Env, config.Env...)
		cmd := config.Cmd
		if len(args) > 0 {
			cmd = args
		}
		s.Process.Args = append(config.Entrypoint, cmd...)
		cwd := config.WorkingDir
		if cwd == "" {
			cwd = "/"
		}
		s.Process.Cwd = cwd
		if config.User != "" {
			return WithUser(config.User)(ctx, client, c, s)
		}
		return nil
	}
}

// WithRootFSPath specifies unmanaged rootfs path.
func WithRootFSPath(path string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setRoot(s)
		s.Root.Path = path
		// Entrypoint is not set here (it's up to caller)
		return nil
	}
}

// WithRootFSReadonly sets specs.Root.Readonly to true
func WithRootFSReadonly() SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setRoot(s)
		s.Root.Readonly = true
		return nil
	}
}

// WithNoNewPrivileges sets no_new_privileges on the process for the container
func WithNoNewPrivileges(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setProcess(s)
	s.Process.NoNewPrivileges = true
	return nil
}

// WithHostHostsFile bind-mounts the host's /etc/hosts into the container as readonly
func WithHostHostsFile(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/hosts",
		Type:        "bind",
		Source:      "/etc/hosts",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

// WithHostResolvconf bind-mounts the host's /etc/resolv.conf into the container as readonly
func WithHostResolvconf(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/resolv.conf",
		Type:        "bind",
		Source:      "/etc/resolv.conf",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

// WithHostLocaltime bind-mounts the host's /etc/localtime into the container as readonly
func WithHostLocaltime(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/localtime",
		Type:        "bind",
		Source:      "/etc/localtime",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

// WithUserNamespace sets the uid and gid mappings for the task
// this can be called multiple times to add more mappings to the generated spec
func WithUserNamespace(container, host, size uint32) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		var hasUserns bool
		setLinux(s)
		for _, ns := range s.Linux.Namespaces {
			if ns.Type == specs.UserNamespace {
				hasUserns = true
				break
			}
		}
		if !hasUserns {
			s.Linux.Namespaces = append(s.Linux.Namespaces, specs.LinuxNamespace{
				Type: specs.UserNamespace,
			})
		}
		mapping := specs.LinuxIDMapping{
			ContainerID: container,
			HostID:      host,
			Size:        size,
		}
		s.Linux.UIDMappings = append(s.Linux.UIDMappings, mapping)
		s.Linux.GIDMappings = append(s.Linux.GIDMappings, mapping)
		return nil
	}
}

// WithCgroup sets the container's cgroup path
func WithCgroup(path string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setLinux(s)
		s.Linux.CgroupsPath = path
		return nil
	}
}

// WithNamespacedCgroup uses the namespace set on the context to create a
// root directory for containers in the cgroup with the id as the subcgroup
func WithNamespacedCgroup() SpecOpts {
	return func(ctx context.Context, _ Client, c *containers.Container, s *Spec) error {
		namespace, err := namespaces.NamespaceRequired(ctx)
		if err != nil {
			return err
		}
		setLinux(s)
		s.Linux.CgroupsPath = filepath.Join("/", namespace, c.ID)
		return nil
	}
}

// WithUser sets the user to be used within the container.
// It accepts a valid user string in OCI Image Spec v1.0.0:
//   user, uid, user:group, uid:gid, uid:group, user:gid
func WithUser(userstr string) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *Spec) error {
		setProcess(s)
		parts := strings.Split(userstr, ":")
		switch len(parts) {
		case 1:
			v, err := strconv.Atoi(parts[0])
			if err != nil {
				// if we cannot parse as a uint they try to see if it is a username
				return WithUsername(userstr)(ctx, client, c, s)
			}
			return WithUserID(uint32(v))(ctx, client, c, s)
		case 2:
			var (
				username  string
				groupname string
			)
			var uid, gid uint32
			v, err := strconv.Atoi(parts[0])
			if err != nil {
				username = parts[0]
			} else {
				uid = uint32(v)
			}
			if v, err = strconv.Atoi(parts[1]); err != nil {
				groupname = parts[1]
			} else {
				gid = uint32(v)
			}
			if username == "" && groupname == "" {
				s.Process.User.UID, s.Process.User.GID = uid, gid
				return nil
			}
			f := func(root string) error {
				if username != "" {
					uid, _, err = getUIDGIDFromPath(root, func(u user.User) bool {
						return u.Name == username
					})
					if err != nil {
						return err
					}
				}
				if groupname != "" {
					gid, err = getGIDFromPath(root, func(g user.Group) bool {
						return g.Name == groupname
					})
					if err != nil {
						return err
					}
				}
				s.Process.User.UID, s.Process.User.GID = uid, gid
				return nil
			}
			if c.Snapshotter == "" && c.SnapshotKey == "" {
				if !isRootfsAbs(s.Root.Path) {
					return errors.New("rootfs absolute path is required")
				}
				return f(s.Root.Path)
			}
			if c.Snapshotter == "" {
				return errors.New("no snapshotter set for container")
			}
			if c.SnapshotKey == "" {
				return errors.New("rootfs snapshot not created for container")
			}
			snapshotter := client.SnapshotService(c.Snapshotter)
			mounts, err := snapshotter.Mounts(ctx, c.SnapshotKey)
			if err != nil {
				return err
			}
			return mount.WithTempMount(ctx, mounts, f)
		default:
			return fmt.Errorf("invalid USER value %s", userstr)
		}
	}
}

// WithUIDGID allows the UID and GID for the Process to be set
func WithUIDGID(uid, gid uint32) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.User.UID = uid
		s.Process.User.GID = gid
		return nil
	}
}

// WithUserID sets the correct UID and GID for the container based
// on the image's /etc/passwd contents. If /etc/passwd does not exist,
// or uid is not found in /etc/passwd, it sets the requested uid,
// additionally sets the gid to 0, and does not return an error.
func WithUserID(uid uint32) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *Spec) (err error) {
		setProcess(s)
		if c.Snapshotter == "" && c.SnapshotKey == "" {
			if !isRootfsAbs(s.Root.Path) {
				return errors.Errorf("rootfs absolute path is required")
			}
			uuid, ugid, err := getUIDGIDFromPath(s.Root.Path, func(u user.User) bool {
				return u.Uid == int(uid)
			})
			if err != nil {
				if os.IsNotExist(err) || err == errNoUsersFound {
					s.Process.User.UID, s.Process.User.GID = uid, 0
					return nil
				}
				return err
			}
			s.Process.User.UID, s.Process.User.GID = uuid, ugid
			return nil

		}
		if c.Snapshotter == "" {
			return errors.Errorf("no snapshotter set for container")
		}
		if c.SnapshotKey == "" {
			return errors.Errorf("rootfs snapshot not created for container")
		}
		snapshotter := client.SnapshotService(c.Snapshotter)
		mounts, err := snapshotter.Mounts(ctx, c.SnapshotKey)
		if err != nil {
			return err
		}
		return mount.WithTempMount(ctx, mounts, func(root string) error {
			uuid, ugid, err := getUIDGIDFromPath(root, func(u user.User) bool {
				return u.Uid == int(uid)
			})
			if err != nil {
				if os.IsNotExist(err) || err == errNoUsersFound {
					s.Process.User.UID, s.Process.User.GID = uid, 0
					return nil
				}
				return err
			}
			s.Process.User.UID, s.Process.User.GID = uuid, ugid
			return nil
		})
	}
}

// WithUsername sets the correct UID and GID for the container
// based on the the image's /etc/passwd contents. If /etc/passwd
// does not exist, or the username is not found in /etc/passwd,
// it returns error.
func WithUsername(username string) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *Spec) (err error) {
		setProcess(s)
		if c.Snapshotter == "" && c.SnapshotKey == "" {
			if !isRootfsAbs(s.Root.Path) {
				return errors.Errorf("rootfs absolute path is required")
			}
			uid, gid, err := getUIDGIDFromPath(s.Root.Path, func(u user.User) bool {
				return u.Name == username
			})
			if err != nil {
				return err
			}
			s.Process.User.UID, s.Process.User.GID = uid, gid
			return nil
		}
		if c.Snapshotter == "" {
			return errors.Errorf("no snapshotter set for container")
		}
		if c.SnapshotKey == "" {
			return errors.Errorf("rootfs snapshot not created for container")
		}
		snapshotter := client.SnapshotService(c.Snapshotter)
		mounts, err := snapshotter.Mounts(ctx, c.SnapshotKey)
		if err != nil {
			return err
		}
		return mount.WithTempMount(ctx, mounts, func(root string) error {
			uid, gid, err := getUIDGIDFromPath(root, func(u user.User) bool {
				return u.Name == username
			})
			if err != nil {
				return err
			}
			s.Process.User.UID, s.Process.User.GID = uid, gid
			return nil
		})
	}
}

// WithCapabilities sets Linux capabilities on the process
func WithCapabilities(caps []string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setCapabilities(s)

		s.Process.Capabilities.Bounding = caps
		s.Process.Capabilities.Effective = caps
		s.Process.Capabilities.Permitted = caps
		s.Process.Capabilities.Inheritable = caps

		return nil
	}
}

// WithAllCapabilities sets all linux capabilities for the process
var WithAllCapabilities = WithCapabilities(getAllCapabilities())

func getAllCapabilities() []string {
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}
	var caps []string
	for _, cap := range capability.List() {
		if cap > last {
			continue
		}
		caps = append(caps, "CAP_"+strings.ToUpper(cap.String()))
	}
	return caps
}

var errNoUsersFound = errors.New("no users found")

func getUIDGIDFromPath(root string, filter func(user.User) bool) (uid, gid uint32, err error) {
	ppath, err := fs.RootPath(root, "/etc/passwd")
	if err != nil {
		return 0, 0, err
	}
	users, err := user.ParsePasswdFileFilter(ppath, filter)
	if err != nil {
		return 0, 0, err
	}
	if len(users) == 0 {
		return 0, 0, errNoUsersFound
	}
	u := users[0]
	return uint32(u.Uid), uint32(u.Gid), nil
}

var errNoGroupsFound = errors.New("no groups found")

func getGIDFromPath(root string, filter func(user.Group) bool) (gid uint32, err error) {
	gpath, err := fs.RootPath(root, "/etc/group")
	if err != nil {
		return 0, err
	}
	groups, err := user.ParseGroupFileFilter(gpath, filter)
	if err != nil {
		return 0, err
	}
	if len(groups) == 0 {
		return 0, errNoGroupsFound
	}
	g := groups[0]
	return uint32(g.Gid), nil
}

func isRootfsAbs(root string) bool {
	return filepath.IsAbs(root)
}

// WithMaskedPaths sets the masked paths option
func WithMaskedPaths(paths []string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setLinux(s)
		s.Linux.MaskedPaths = paths
		return nil
	}
}

// WithReadonlyPaths sets the read only paths option
func WithReadonlyPaths(paths []string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setLinux(s)
		s.Linux.ReadonlyPaths = paths
		return nil
	}
}

// WithWriteableSysfs makes any sysfs mounts writeable
func WithWriteableSysfs(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	for i, m := range s.Mounts {
		if m.Type == "sysfs" {
			var options []string
			for _, o := range m.Options {
				if o == "ro" {
					o = "rw"
				}
				options = append(options, o)
			}
			s.Mounts[i].Options = options
		}
	}
	return nil
}

// WithWriteableCgroupfs makes any cgroup mounts writeable
func WithWriteableCgroupfs(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	for i, m := range s.Mounts {
		if m.Type == "cgroup" {
			var options []string
			for _, o := range m.Options {
				if o == "ro" {
					o = "rw"
				}
				options = append(options, o)
			}
			s.Mounts[i].Options = options
		}
	}
	return nil
}

// WithSelinuxLabel sets the process SELinux label
func WithSelinuxLabel(label string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.SelinuxLabel = label
		return nil
	}
}

// WithApparmorProfile sets the Apparmor profile for the process
func WithApparmorProfile(profile string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
		setProcess(s)
		s.Process.ApparmorProfile = profile
		return nil
	}
}

// WithSeccompUnconfined clears the seccomp profile
func WithSeccompUnconfined(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setLinux(s)
	s.Linux.Seccomp = nil
	return nil
}

// WithParentCgroupDevices uses the default cgroup setup to inherit the container's parent cgroup's
// allowed and denied devices
func WithParentCgroupDevices(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setLinux(s)
	if s.Linux.Resources == nil {
		s.Linux.Resources = &specs.LinuxResources{}
	}
	s.Linux.Resources.Devices = nil
	return nil
}

// WithDefaultUnixDevices adds the default devices for unix such as /dev/null, /dev/random to
// the container's resource cgroup spec
func WithDefaultUnixDevices(_ context.Context, _ Client, _ *containers.Container, s *Spec) error {
	setLinux(s)
	if s.Linux.Resources == nil {
		s.Linux.Resources = &specs.LinuxResources{}
	}
	intptr := func(i int64) *int64 {
		return &i
	}
	s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, []specs.LinuxDeviceCgroup{
		{
			// "/dev/null",
			Type:   "c",
			Major:  intptr(1),
			Minor:  intptr(3),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/random",
			Type:   "c",
			Major:  intptr(1),
			Minor:  intptr(8),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/full",
			Type:   "c",
			Major:  intptr(1),
			Minor:  intptr(7),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/tty",
			Type:   "c",
			Major:  intptr(5),
			Minor:  intptr(0),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/zero",
			Type:   "c",
			Major:  intptr(1),
			Minor:  intptr(5),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/urandom",
			Type:   "c",
			Major:  intptr(1),
			Minor:  intptr(9),
			Access: rwm,
			Allow:  true,
		},
		{
			// "/dev/console",
			Type:   "c",
			Major:  intptr(5),
			Minor:  intptr(1),
			Access: rwm,
			Allow:  true,
		},
		// /dev/pts/ - pts namespaces are "coming soon"
		{
			Type:   "c",
			Major:  intptr(136),
			Access: rwm,
			Allow:  true,
		},
		{
			Type:   "c",
			Major:  intptr(5),
			Minor:  intptr(2),
			Access: rwm,
			Allow:  true,
		},
		{
			// tuntap
			Type:   "c",
			Major:  intptr(10),
			Minor:  intptr(200),
			Access: rwm,
			Allow:  true,
		},
	}...)
	return nil
}

// WithPrivileged sets up options for a privileged container
// TODO(justincormack) device handling
var WithPrivileged = Compose(
	WithAllCapabilities,
	WithMaskedPaths(nil),
	WithReadonlyPaths(nil),
	WithWriteableSysfs,
	WithWriteableCgroupfs,
	WithSelinuxLabel(""),
	WithApparmorProfile(""),
	WithSeccompUnconfined,
)
