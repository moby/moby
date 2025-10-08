package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/identity"
	"github.com/moby/sys/mountinfo"
	"github.com/pkg/errors"
)

const volumePath = "/abc/a"

var rootfs string

func main() {
	if err := run(); err != nil {
		log.Printf("error: %+v", err)
	}
}

func run() error {
	infos, err := mountinfo.GetMounts(nil)
	if err != nil {
		return err
	}
	hasVolume := false
	for _, info := range infos {
		if info.Mountpoint == "/" {
			v, err := getOverlayRootfs(info)
			if err != nil {
				return err
			}
			rootfs = v
		}

		if info.Mountpoint == volumePath {
			hasVolume = true
		}
		log.Printf("mount: %+v", info)
	}
	if !hasVolume {
		return errors.Errorf("volume not found: %s", volumePath)
	}

	log.Printf("rootfs: %s", rootfs)

	base := filepath.Base(rootfs)
	if err := os.Symlink("/", filepath.Join(volumePath, base)); err != nil {
		return err
	}

	// create duplicate volume path with symlink target
	p := "/"
	var volumeRoot string
	for _, c := range strings.Split(filepath.Dir(volumePath), string(filepath.Separator)) {
		if c == "" {
			continue
		}
		if volumeRoot == "" {
			volumeRoot = "/" + c
			c += "_target"
		}
		p = filepath.Join(p, c)
		if err := os.Mkdir(p, 0755); err != nil {
			if os.IsExist(err) {
				continue
			}
			return err
		}
		log.Printf("created: %s", p)
	}
	if err := os.Symlink(filepath.Dir(rootfs), filepath.Join(p, filepath.Base(volumePath))); err != nil {
		return err
	}

	if err := os.Rename(volumeRoot, volumeRoot+"_old"); err != nil {
		return err
	}

	for {
		if _, err := os.Stat(volumeRoot); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		// log.Printf("detected: %s", volumeRoot)
		break
	}

	for {
		if err := os.Rename(volumeRoot+"_target", volumeRoot); err != nil {
			if os.IsExist(err) {
				if err := os.Rename(volumeRoot, volumeRoot+"_"+identity.NewID()); err != nil {
					return err
				}
				continue
			}
			log.Printf("failed to rename: %s", err)
		}
		break
	}

	return nil
}

func getOverlayRootfs(info *mountinfo.Info) (string, error) {
	if info.FSType != "overlay" {
		return "", errors.Errorf("not overlay: %s", info.FSType)
	}
	for _, opt := range strings.Split(info.VFSOptions, ",") {
		parts := strings.SplitN(opt, "=", 2)
		if parts[0] == "workdir" {
			return filepath.Join(filepath.Dir(parts[1]), "merged"), nil
		}
	}
	return "", errors.Errorf("workdir not found: %s", info.VFSOptions)
}

// /var/lib/docker/overlay2/86fcb18db0e3774cfe3b9ed6fa526d4dffcb45ac3b26cbe99db6c2d08e6dfc0e/merged

// merged/<sym>/dir1/dir2/

// volume
