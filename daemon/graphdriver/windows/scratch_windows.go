package windows

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/containerd/log"
	"golang.org/x/sys/windows"
)

// Flushing the scratch volume before it is detached.
//
// DeactivateLayer detaches a container's scratch VHD while NTFS may still
// cache dirty pages of the volume. Writing them afterwards fails ("Delayed
// Write Failed"), and such a page stays on Windows' modified-no-write list for
// good: it keeps its control area, the control area the stream's file object,
// and that file object the whole volume, until the node reboots. Flushing and
// dismounting the volume before the detach leaves nothing behind.
//
// The volume stays locked until the detach is done. A dismounted volume that
// is not locked is mounted again by the next open of anything on it (a
// scanner, a performance counter); when that open races the detach, NTFS
// cannot mount the vanishing disk, RAW does, and RAW keeps the volume until
// the node reboots.

const (
	fsctlLockVolume     = 0x00090018
	fsctlDismountVolume = 0x00090020

	// scratchLockTimeout bounds how long Put waits for other handles on the
	// volume (a scanner reading what the container wrote, say) to close
	// before it dismounts the volume anyway. Under load such a scan can take
	// a few seconds.
	scratchLockTimeout = 10 * time.Second

	// scratchRelockTimeout bounds the second attempt, on a handle opened
	// after that forced dismount invalidated the other handles.
	scratchRelockTimeout = time.Second
)

// scratchVolumeDevice returns the device path (\\?\Volume{guid}) of a layer
// mounted as a volume (mount path \\?\Volume{guid}, with or without a
// trailing backslash), and false for any other layer.
func scratchVolumeDevice(mountPath string) (string, bool) {
	device := strings.TrimSuffix(mountPath, `\`)
	if !strings.HasPrefix(device, `\\?\Volume{`) || !strings.HasSuffix(device, `}`) {
		return "", false
	}
	return device, true
}

func openVolume(device string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(device)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

func volumeControl(h windows.Handle, code uint32) error {
	var n uint32
	return windows.DeviceIoControl(h, code, nil, 0, nil, 0, &n, nil)
}

// lockVolume locks the volume, retrying every 50 ms while another handle is
// open, for up to timeout.
func lockVolume(h windows.Handle, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if volumeControl(h, fsctlLockVolume) == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// dismountScratchVolume writes out everything NTFS caches for the volume,
// locks it, waiting up to scratchLockTimeout for other handles to close, and
// dismounts it. It returns the volume handle, which keeps the volume locked
// and dismounted (any other open fails with ERROR_NOT_READY) until it is
// closed. If the lock cannot be taken, the dismount is forced, which
// invalidates the other handles after the flush, and a new handle locks the
// volume again. It reports whether the volume ended up locked and whether
// the dismount had to be forced.
func dismountScratchVolume(device string) (_ windows.Handle, locked, forced bool, _ error) {
	h, err := openVolume(device)
	if err != nil {
		return 0, false, false, fmt.Errorf("open: %w", err)
	}
	for attempt, timeout := range []time.Duration{scratchLockTimeout, scratchRelockTimeout} {
		if forced = attempt > 0; forced {
			// Force the dismount: it invalidates the other handles, and the
			// next open mounts the volume again with no other handle on it.
			err := volumeControl(h, fsctlDismountVolume)
			windows.CloseHandle(h)
			if err != nil {
				return 0, false, forced, fmt.Errorf("dismount: %w", err)
			}
			if h, err = openVolume(device); err != nil {
				return 0, false, forced, fmt.Errorf("reopen: %w", err)
			}
		}
		if err := windows.FlushFileBuffers(h); err != nil {
			windows.CloseHandle(h)
			return 0, false, forced, fmt.Errorf("flush: %w", err)
		}
		if locked = lockVolume(h, timeout); locked {
			break
		}
	}
	if err := volumeControl(h, fsctlDismountVolume); err != nil {
		windows.CloseHandle(h)
		return 0, locked, forced, fmt.Errorf("dismount: %w", err)
	}
	return h, locked, forced, nil
}

// flushScratch flushes and dismounts the scratch volume of layer id before
// Put detaches it, and returns the function that unlocks it, which Put calls
// once the detach is done. Failures are logged and never fail the Put.
func flushScratch(id, mountPath string) (unlock func()) {
	device, ok := scratchVolumeDevice(mountPath)
	if !ok {
		return func() {}
	}
	start := time.Now()
	h, locked, forced, err := dismountScratchVolume(device)
	logger := log.G(context.TODO()).WithFields(log.Fields{"id": id, "volume": device, "locked": locked, "forced": forced, "elapsed": time.Since(start).String()})
	switch {
	case err != nil:
		logger.WithError(err).Warn("Could not flush the scratch volume before detaching it")
		return func() {}
	case !locked:
		logger.Warn("Dismounted the scratch volume without a lock: another handle stayed open")
	case forced:
		logger.Warn("Forced the scratch volume's dismount, then locked it: another handle stayed open")
	default:
		logger.Debug("Flushed and dismounted the scratch volume")
	}
	return func() { windows.CloseHandle(h) }
}
