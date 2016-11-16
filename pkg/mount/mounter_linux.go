package mount

import (
	"syscall"
)

func mount(device, target, mType string, flag uintptr, data string) error {
	if err := syscall.Mount(device, target, mType, flag, data); err != nil {
		// If traditional mount failed, give one more chance for fuse-based/glusterfs/ceph/.. requests to mount via standard mount command
		// * SSHFS Example: docker service create .. --mount type=volume,volume-opt=type=fuse.sshfs,volume-opt=device=ssh-node-1:/home/user1,volume-opt=o=workaround=all:o=reconnect:o=IdentityFile=/ssh_keys/user1.key,source=user1.home,target=/mount ..
		// * GlusterFS Example: docker service create .. --mount type=volume,volume-opt=type=glusterfs,volume-opt=device=gfs-node-1:/shared,source=gfs-shared-1,target=/mount ..
		// * CephFS Example: docker service create .. --mount type=volume,volume-opt=type=ceph,volume-opt=device=ceph-node-1:/shared,source=ceph-shared-1,target=/mount ..
		cmd := []string{"-t", mType, device, target}
		if data != "" {
			opts := strings.Split(data, ":o=")
			for i := 0; i < len(opts); i++ {
				cmd = append(cmd, []string{"-o", opts[i]}...)
			}
		}
		err := exec.Command("mount", cmd...).Run()
		return err
	}

	// If we have a bind mount or remount, remount...
	if flag&syscall.MS_BIND == syscall.MS_BIND && flag&syscall.MS_RDONLY == syscall.MS_RDONLY {
		return syscall.Mount(device, target, mType, flag|syscall.MS_REMOUNT, data)
	}
	return nil
}

func unmount(target string, flag int) error {
	return syscall.Unmount(target, flag)
}
