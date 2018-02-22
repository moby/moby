// +build !windows

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/fs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/user"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// WithTTY sets the information on the spec as well as the environment variables for
// using a TTY
func WithTTY(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
	s.Process.Terminal = true
	s.Process.Env = append(s.Process.Env, "TERM=xterm")
	return nil
}

// WithHostNamespace allows a task to run inside the host's linux namespace
func WithHostNamespace(ns specs.LinuxNamespaceType) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
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
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
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
	return func(ctx context.Context, client Client, c *containers.Container, s *specs.Spec) error {
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
			p, err := content.ReadBlob(ctx, image.ContentStore(), ic.Digest)
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

		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		s.Process.Env = append(s.Process.Env, config.Env...)
		cmd := config.Cmd
		s.Process.Args = append(config.Entrypoint, cmd...)
		if config.User != "" {
			parts := strings.Split(config.User, ":")
			switch len(parts) {
			case 1:
				v, err := strconv.Atoi(parts[0])
				if err != nil {
					// if we cannot parse as a uint they try to see if it is a username
					if err := WithUsername(config.User)(ctx, client, c, s); err != nil {
						return err
					}
					return err
				}
				if err := WithUserID(uint32(v))(ctx, client, c, s); err != nil {
					return err
				}
			case 2:
				v, err := strconv.Atoi(parts[0])
				if err != nil {
					return errors.Wrapf(err, "parse uid %s", parts[0])
				}
				uid := uint32(v)
				if v, err = strconv.Atoi(parts[1]); err != nil {
					return errors.Wrapf(err, "parse gid %s", parts[1])
				}
				gid := uint32(v)
				s.Process.User.UID, s.Process.User.GID = uid, gid
			default:
				return fmt.Errorf("invalid USER value %s", config.User)
			}
		}
		cwd := config.WorkingDir
		if cwd == "" {
			cwd = "/"
		}
		s.Process.Cwd = cwd
		return nil
	}
}

// WithRootFSPath specifies unmanaged rootfs path.
func WithRootFSPath(path string) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		if s.Root == nil {
			s.Root = &specs.Root{}
		}
		s.Root.Path = path
		// Entrypoint is not set here (it's up to caller)
		return nil
	}
}

// WithRootFSReadonly sets specs.Root.Readonly to true
func WithRootFSReadonly() SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		if s.Root == nil {
			s.Root = &specs.Root{}
		}
		s.Root.Readonly = true
		return nil
	}
}

// WithNoNewPrivileges sets no_new_privileges on the process for the container
func WithNoNewPrivileges(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
	s.Process.NoNewPrivileges = true
	return nil
}

// WithHostHostsFile bind-mounts the host's /etc/hosts into the container as readonly
func WithHostHostsFile(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/hosts",
		Type:        "bind",
		Source:      "/etc/hosts",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

// WithHostResolvconf bind-mounts the host's /etc/resolv.conf into the container as readonly
func WithHostResolvconf(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
	s.Mounts = append(s.Mounts, specs.Mount{
		Destination: "/etc/resolv.conf",
		Type:        "bind",
		Source:      "/etc/resolv.conf",
		Options:     []string{"rbind", "ro"},
	})
	return nil
}

// WithHostLocaltime bind-mounts the host's /etc/localtime into the container as readonly
func WithHostLocaltime(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
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
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		var hasUserns bool
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
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.CgroupsPath = path
		return nil
	}
}

// WithNamespacedCgroup uses the namespace set on the context to create a
// root directory for containers in the cgroup with the id as the subcgroup
func WithNamespacedCgroup() SpecOpts {
	return func(ctx context.Context, _ Client, c *containers.Container, s *specs.Spec) error {
		namespace, err := namespaces.NamespaceRequired(ctx)
		if err != nil {
			return err
		}
		s.Linux.CgroupsPath = filepath.Join("/", namespace, c.ID)
		return nil
	}
}

// WithUIDGID allows the UID and GID for the Process to be set
func WithUIDGID(uid, gid uint32) SpecOpts {
	return func(_ context.Context, _ Client, _ *containers.Container, s *specs.Spec) error {
		s.Process.User.UID = uid
		s.Process.User.GID = gid
		return nil
	}
}

// WithUserID sets the correct UID and GID for the container based
// on the image's /etc/passwd contents. If /etc/passwd does not exist,
// or uid is not found in /etc/passwd, it sets gid to be the same with
// uid, and not returns error.
func WithUserID(uid uint32) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *specs.Spec) (err error) {
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
		root, err := ioutil.TempDir("", "ctd-username")
		if err != nil {
			return err
		}
		defer os.Remove(root)
		for _, m := range mounts {
			if err := m.Mount(root); err != nil {
				return err
			}
		}
		defer func() {
			if uerr := mount.Unmount(root, 0); uerr != nil {
				if err == nil {
					err = uerr
				}
			}
		}()
		ppath, err := fs.RootPath(root, "/etc/passwd")
		if err != nil {
			return err
		}
		f, err := os.Open(ppath)
		if err != nil {
			if os.IsNotExist(err) {
				s.Process.User.UID, s.Process.User.GID = uid, uid
				return nil
			}
			return err
		}
		defer f.Close()
		users, err := user.ParsePasswdFilter(f, func(u user.User) bool {
			return u.Uid == int(uid)
		})
		if err != nil {
			return err
		}
		if len(users) == 0 {
			s.Process.User.UID, s.Process.User.GID = uid, uid
			return nil
		}
		u := users[0]
		s.Process.User.UID, s.Process.User.GID = uint32(u.Uid), uint32(u.Gid)
		return nil
	}
}

// WithUsername sets the correct UID and GID for the container
// based on the the image's /etc/passwd contents. If /etc/passwd
// does not exist, or the username is not found in /etc/passwd,
// it returns error.
func WithUsername(username string) SpecOpts {
	return func(ctx context.Context, client Client, c *containers.Container, s *specs.Spec) (err error) {
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
		root, err := ioutil.TempDir("", "ctd-username")
		if err != nil {
			return err
		}
		defer os.Remove(root)
		for _, m := range mounts {
			if err := m.Mount(root); err != nil {
				return err
			}
		}
		defer func() {
			if uerr := mount.Unmount(root, 0); uerr != nil {
				if err == nil {
					err = uerr
				}
			}
		}()
		ppath, err := fs.RootPath(root, "/etc/passwd")
		if err != nil {
			return err
		}
		f, err := os.Open(ppath)
		if err != nil {
			return err
		}
		defer f.Close()
		users, err := user.ParsePasswdFilter(f, func(u user.User) bool {
			return u.Name == username
		})
		if err != nil {
			return err
		}
		if len(users) == 0 {
			return errors.Errorf("no users found for %s", username)
		}
		u := users[0]
		s.Process.User.UID, s.Process.User.GID = uint32(u.Uid), uint32(u.Gid)
		return nil
	}
}
