// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Loopback related errors
var (
	ErrAttachLoopbackDevice   = errors.New("loopback attach failed")
	ErrGetLoopbackBackingFile = errors.New("Unable to get loopback backing file")
	ErrSetCapacity            = errors.New("Unable set loopback capacity")
)

func stringToLoopName(src string) [LoNameSize]uint8 {
	var dst [LoNameSize]uint8
	copy(dst[:], src[:])
	return dst
}

// The following handles the free-open race by sleeping for some random
// interval between 0ms and 64ms, up to 32 times for an absolute maximum
// delay of 2048ms (kernel time notwithstanding).
//
// We assume that the tick frequency of the kernel is, at its lowest,
// 250Hz, which gives us a period of 4ms. Experiments indicate that disjoint
// sleep times within a tick period's distance end up colliding with each
// other and require more attempts to not race each other. Using an expected
// tick period of 4ms with a maximum sleep time of 64ms gives us 16 discrete
// tick windows to work with. Using a maximum sleep time of 32ms would only
// give us 8 discrete tick windows.
//
// If we assume a burst mode of usage where 32 threads simultaneously race
// each other for access to an open loop device, and no other threads attempt
// to access an open loop device while these 32 are attempting to access an
// open loop device, this attempt loop will succeed 100% of the time. However,
// the moment you add one more thread, you introduce the possibility of
// racing all the other threads past the maximum attempt count, and never
// managing to access an open loop device.
//
// Let's use a 33 thread race as an example. If we were using a 32ms maximum
// sleep time on a kernel with a 250Hz tick rate, we would have 8 tick windows
// to work with on each attempt, and as per the pigeonhole principle
// two threads are guaranteed to race each other for the first 24 attempts.
// Of the 9 remaining threads, 8 will always succeed. There is a non-zero
// chance that the 9th thread will not. The probability of the 9th thread
// succeeding is the probability that the 9th thread races another thread
// for the same tick window on every every attempt, i.e.
//
// 8! / 8^8 ~= 0.0024
//
// This only gives us two 9s of reliability for that 33rd thread succeeding.
//
// With a 64ms maximum sleep time and a 4ms tick period, we now have 16 tick
// windows to work with on every attempt. This means that the pigeonhole
// principle only applies to 32 threads for the first 16 attempts, and for
// the 33rd thread, now treated as the 17th thread after the first 16 attempts,
// the probability that it will race another thread for the same tick window
// on every remaining attempt is now
//
// 16! / 16^16 ~= 1.134e-6
//
// Which gives us five 9s of reliability for that 33rd thread succeeding.
const (
	openAttempts = 32
	maxSleepTime = 64
)

var (
	rngLauncher sync.Once
	rngChan     chan time.Duration
)

func getSleepTime() time.Duration {
	rngLauncher.Do(func() {
		rngChan = make(chan time.Duration)
		go (func() {
			gen := rand.New(rand.NewSource(time.Now().UnixNano()))
			for {
				rngChan <- time.Duration(gen.Int31n(maxSleepTime+1)) * time.Millisecond
			}
		})()
	})
	return <-rngChan
}

func openNextAvailableLoopback(sparseFile *os.File) (loopFile *os.File, err error) {
	var loopName string
	// This is nil'd out on success, otherwise this is what we want to return
	err = ErrAttachLoopbackDevice
	modCtx := &concreteLoopModuleContext{}

	for i := 0; i < openAttempts; i++ {
		loopFileAttempt, created, typedErr := attachToNextAvailableDevice(modCtx, sparseFile)
		if created >= 0 {
			loopName = fmt.Sprintf(loopFormat, created)
			logrus.Warnf("Tried to forcibly create loopback device %s", loopName)
		}
		if typedErr == nil {
			err = nil
			loopFile = loopFileAttempt
			return
		}
		switch typedErr.atState {
		case attachErrorStateAttachFd:
			if i < (openAttempts - 1) {
				sleepTime := getSleepTime()
				sleepTimeMs := sleepTime / time.Millisecond
				logrus.Warnf("Lost race to attach to open loopback device (attempt %d of %d, sleep %dms)", i+1, openAttempts, sleepTimeMs)
				time.Sleep(sleepTime)
			} else {
				logrus.Errorf("Lost race to attach open loopback device (%d attempts)", openAttempts)
			}
		case attachErrorStateNextFree:
			logrus.Errorf("Error retrieving the next available loopback: %s", typedErr.underlying)
			return
		case attachErrorStateMknod:
			if created < 0 {
				panic(errors.New("Expected created device index"))
			}
			logrus.Errorf("Error creating loopback device %s: %s", loopName, typedErr.underlying)
			return
		case attachErrorStateStat:
			logrus.Errorf("Could not stat loopback device: %s", typedErr.underlying)
			return
		case attachErrorStateModeCheck:
			logrus.Error("Reported loopback device was not a block device")
			return
		case attachErrorStateOpenBlock:
			logrus.Errorf("Could not open loopback device: %s", typedErr.underlying)
			return
		}
	}
	return
}

func setAutoClear(loopFile *os.File) error {
	loopInfo := &loopInfo64{
		loFileName: stringToLoopName(loopFile.Name()),
		loOffset:   0,
		loFlags:    LoFlagsAutoClear,
	}

	return ioctlLoopSetStatus64(loopFile.Fd(), loopInfo)
}

// AttachLoopDevice attaches the given sparse file to the next
// available loopback device. It returns an opened *os.File.
func AttachLoopDevice(sparseName string) (loop *os.File, err error) {
	// OpenFile adds O_CLOEXEC
	sparseFile, err := os.OpenFile(sparseName, os.O_RDWR, 0644)
	if err != nil {
		logrus.Errorf("Error opening sparse file %s: %s", sparseName, err)
		return nil, ErrAttachLoopbackDevice
	}
	defer sparseFile.Close()

	loopFile, err := openNextAvailableLoopback(sparseFile)
	if err != nil {
		return nil, err
	}

	if err := setAutoClear(loopFile); err != nil {
		logrus.Errorf("Cannot set up loopback device info: %s", err)

		// If the call failed, then free the loopback device
		if err := ioctlLoopClrFd(loopFile.Fd()); err != nil {
			logrus.Error("Error while cleaning up the loopback device")
		}
		loopFile.Close()
		return nil, ErrAttachLoopbackDevice
	}

	return loopFile, nil
}
