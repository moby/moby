package zfs

import (
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"strings"
)

// TODO(burke): Return output instead of "exit status 1"

// TODO(burke): debug logging in a more sane way here...
func zfsCommand(args ...string) *exec.Cmd {
	log.Println("zfsCommand:", args)
	return exec.Command("zfs", args...)
}

func zfsGetPool(path string) (poolName, mountPoint string, err error) {
	args := []string{"list", "-Ho", "name,mountpoint", "-t", "filesystem", path}
	out, err := zfsCommand(args...).Output()
	if err != nil {
		return "", "", err
	}
	line := strings.TrimSpace(string(out))
	fields := strings.Split(line, "\t")
	return fields[0], fields[1], nil
}

func zfsCreateAutomountingDataset(path string) error {
	return zfsCommand("create", "-o", path).Run()
}

func zfsCreateDataset(path string) error {
	// Note that we instruct ZFS not to mount these datasets automatically. It
	// would not be sensible for the OS to attempt to mount all docker volumes on
	// startup.
	return zfsCommand("create", "-o", "canmount=noauto", path).Run()
}

func zfsMountDataset(id, mlsLabel string) error {
	// First, check if it's already mounted, and return if it is.
	out, err := zfsCommand("list", "-Ho", "mounted", id).Output()
	if err == nil && string(out) == "yes\n" {
		return nil
	}

	if mlsLabel == "" {
		return zfsCommand("mount", id).Run()
	}
	label := fmt.Sprintf("mlslabel=%s", mlsLabel)
	return zfsCommand("mount", "-o", label, id).Run()
}

func zfsUnmountDataset(id string) error {
	return zfsCommand("unmount", id).Run()
}

func zfsDatasetExists(id string) bool {
	// here we just interpret any error as non-existence. This isn't ideal, but
	// we're forced into a binary return by Driver.Exists() anyway, so it's ok.
	err := zfsCommand("list", "-Ho", "mounted", id)
	return err == nil
}

// zfsCloneDataset clones a dataset using the following procedure:
//  1. Create a snapshot of the source dataset;
//  2. Clone the snapshot as the destination dataset;
//  3. Destroy the snapshot.
func zfsCloneDataset(src, dest string) error {
	// default rand source is ok here.
	snapshot := fmt.Sprintf("%s@docker_%x", src, rand.Int31())

	// 1. Create the snapshot
	if err := zfsCommand("snapshot", snapshot).Run(); err != nil {
		return err
	}

	// 2. Clone the snapshot
	args := []string{"clone", "-o", "canmount=noauto", snapshot, dest}
	if err := zfsCommand(args...).Run(); err != nil {
		return err
	}

	// 3. Destroy the snapshot
	if err := zfsCommand("destroy", "-d", snapshot).Run(); err != nil {
		return err
	}

	return nil
}

// We *may* want to go through the trouble to promote a clone first here. See:
// https://github.com/gurjeet/docker/blob/00cb6818/graphdriver/zfs/zfs.go#L182-L221
func zfsDestroyDataset(id string) error {
	return zfsCommand("destroy", "-r", id).Run()
}
