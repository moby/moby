// +build linux

package sharedloop

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/docker/docker/pkg/reexec"
)

// FIXME: this is copy-pasted from the aufs driver.
// It should be moved into the core.

// Mounted returns true if a mount point exists.
func Mounted(mountpoint string) (bool, error) {
	mntpoint, err := os.Stat(mountpoint)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	parent, err := os.Stat(filepath.Join(mountpoint, ".."))
	if err != nil {
		return false, err
	}
	mntpointSt := mntpoint.Sys().(*syscall.Stat_t)
	parentSt := parent.Sys().(*syscall.Stat_t)
	return mntpointSt.Dev != parentSt.Dev, nil
}

type probeData struct {
	fsName string
	magic  string
	offset uint64
}

// ProbeFsType returns the filesystem name for the given device id.
func ProbeFsType(device string) (string, error) {
	probes := []probeData{
		{"btrfs", "_BHRfS_M", 0x10040},
		{"ext4", "\123\357", 0x438},
		{"xfs", "XFSB", 0},
	}

	maxLen := uint64(0)
	for _, p := range probes {
		l := p.offset + uint64(len(p.magic))
		if l > maxLen {
			maxLen = l
		}
	}

	file, err := os.Open(device)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, maxLen)
	l, err := file.Read(buffer)
	if err != nil {
		return "", err
	}

	if uint64(l) != maxLen {
		return "", fmt.Errorf("sharedloop: unable to detect filesystem type of %s, short read", device)
	}

	for _, p := range probes {
		if bytes.Equal([]byte(p.magic), buffer[p.offset:p.offset+uint64(len(p.magic))]) {
			return p.fsName, nil
		}
	}

	return "", fmt.Errorf("sharedloop: Unknown filesystem type on %s", device)
}

func joinMountOptions(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "," + b
}

func fatal(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

type mountOptions struct {
	Device string
	Target string
	Type   string
	Label  string
	Flag   uint32
}

func mountFrom(dir, device, target, mType string, flags uintptr, label string) error {
	options := &mountOptions{
		Device: device,
		Target: target,
		Type:   mType,
		Flag:   uint32(flags),
		Label:  label,
	}

	cmd := reexec.Command("docker-mountfrom", dir)
	w, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mountfrom error on pipe creation: %v", err)
	}

	output := bytes.NewBuffer(nil)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mountfrom error on re-exec cmd: %v", err)
	}
	//write the options to the pipe for the untar exec to read
	if err := json.NewEncoder(w).Encode(options); err != nil {
		return fmt.Errorf("mountfrom json encode to pipe failed: %v", err)
	}
	w.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("mountfrom re-exec error: %v: output: %s", err, output)
	}
	return nil
}

// mountfromMain is the entry-point for docker-mountfrom on re-exec.
func mountFromMain() {
	runtime.LockOSThread()
	flag.Parse()

	var options *mountOptions

	if err := json.NewDecoder(os.Stdin).Decode(&options); err != nil {
		fatal(err)
	}

	if err := os.Chdir(flag.Arg(0)); err != nil {
		fatal(err)
	}

	if err := syscall.Mount(options.Device, options.Target, options.Type, uintptr(options.Flag), options.Label); err != nil {
		fatal(err)
	}

	os.Exit(0)
}
