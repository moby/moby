package specconv

import (
	"os"
	"sort"
	"strings"

	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// ToRootless converts spec to be compatible with "rootless" runc.
// * Adds userns (Note: since we are already in userns, ideally we should not need to do this. runc-side issue is tracked at https://github.com/opencontainers/runc/issues/1837)
// * Fix up mount flags (same as above)
// * Replace /sys with bind-mount (FIXME: we don't need to do this if netns is unshared)
func ToRootless(spec *specs.Spec) error {
	if !system.RunningInUserNS() {
		return errors.New("needs to be in user namespace")
	}
	uidMap, err := user.CurrentProcessUIDMap()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	gidMap, err := user.CurrentProcessUIDMap()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return toRootless(spec, uidMap, gidMap)
}

// toRootless was forked from github.com/opencontainers/runc/libcontainer/specconv
func toRootless(spec *specs.Spec, uidMap, gidMap []user.IDMap) error {
	if err := configureUserNS(spec, uidMap, gidMap); err != nil {
		return err
	}
	if err := configureMounts(spec); err != nil {
		return err
	}

	// Remove cgroup settings.
	spec.Linux.Resources = nil
	spec.Linux.CgroupsPath = ""
	return nil
}

// configureUserNS add suserns and the current ID map to the spec.
// Since we are already in userns, ideally we should not need to add userns.
// However, currently rootless runc always requires userns to be added.
// https://github.com/opencontainers/runc/issues/1837
func configureUserNS(spec *specs.Spec, uidMap, gidMap []user.IDMap) error {
	spec.Linux.Namespaces = append(spec.Linux.Namespaces, specs.LinuxNamespace{
		Type: specs.UserNamespace,
	})

	sort.Slice(uidMap, func(i, j int) bool { return uidMap[i].ID < uidMap[j].ID })
	uNextContainerID := int64(0)
	for _, u := range uidMap {
		spec.Linux.UIDMappings = append(spec.Linux.UIDMappings,
			specs.LinuxIDMapping{
				HostID:      uint32(u.ID),
				ContainerID: uint32(uNextContainerID),
				Size:        uint32(u.Count),
			})
		uNextContainerID += int64(u.Count)
	}
	sort.Slice(gidMap, func(i, j int) bool { return gidMap[i].ID < gidMap[j].ID })
	gNextContainerID := int64(0)
	for _, g := range gidMap {
		spec.Linux.GIDMappings = append(spec.Linux.GIDMappings,
			specs.LinuxIDMapping{
				HostID:      uint32(g.ID),
				ContainerID: uint32(gNextContainerID),
				Size:        uint32(g.Count),
			})
		gNextContainerID += int64(g.Count)
	}
	return nil
}

func configureMounts(spec *specs.Spec) error {
	var mounts []specs.Mount
	for _, mount := range spec.Mounts {
		// Ignore all mounts that are under /sys, because we add /sys later.
		if strings.HasPrefix(mount.Destination, "/sys") {
			continue
		}

		// Remove all gid= and uid= mappings.
		// Since we are already in userns, ideally we should not need to do this.
		// https://github.com/opencontainers/runc/issues/1837
		var options []string
		for _, option := range mount.Options {
			if !strings.HasPrefix(option, "gid=") && !strings.HasPrefix(option, "uid=") {
				options = append(options, option)
			}
		}
		mount.Options = options
		mounts = append(mounts, mount)
	}

	// Add the sysfs mount as an rbind, because we can't mount /sys unless we have netns.
	// TODO: keep original /sys mount when we have netns.
	mounts = append(mounts, specs.Mount{
		Source:      "/sys",
		Destination: "/sys",
		Type:        "none",
		Options:     []string{"rbind", "nosuid", "noexec", "nodev", "ro"},
	})
	spec.Mounts = mounts
	return nil
}
