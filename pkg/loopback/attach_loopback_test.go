// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

const maxOpenDevices = 8 // As per daemon/graphdriver/devmapper_test.go: 8's a good number

func TestFindOpenRaceResolution(t *testing.T) {
	modCtx := &concreteLoopModuleContext{}
	backingFiles := []*os.File{}

	defer (func() {
		for _, fp := range backingFiles {
			fp.Close()
		}
	})()

	t.Log("Perform initial create")
	for i := 0; i < maxOpenDevices; i++ {
		// Step 1: open up to maxOpenDevices for our internal usage
		backingFile, err := ioutil.TempFile("", "docker-loopback-test.*.img") 
		if err != nil {
			t.Fatalf("Could not create temporary file: %s", err)
		}

		loopFile, created, typedErr := modCtx.AttachToNextAvailableDevice(backingFile)
		if created >= 0 {
			t.Logf("Attempted to create loop device file %s", fmt.Sprintf(loopFormat, created))
		}
		if typedErr != nil {
			// We _may_ have run out of devices here, but this is not fatal.
			// If we have any devices at all, we can likely run these tests.
			// If we have none, we'll just skip the test entirely.
			t.Logf("Error opening next loop device at state %s: %s", attachErrorString(typedErr.atState), typedErr.underlying)
			backingFile.Close()
			break
		} else {
			if err := ioctlLoopClrFd(loopFile.Fd()); err != nil {
				t.Fatalf("Could not clear loop device file descriptor: %s", err)
			}
			loopFile.Close()
		}

		backingFiles = append(backingFiles, backingFile)
	}

	if len(backingFiles) == 0 {
		t.Skip("Could not open any loop devices")
	}

	// Step 2: Open as many devices as we just did, but in parallel
	var (
		goroutineLaunchWg sync.WaitGroup
		goroutineReturn   chan error = make(chan error)
		returnCount       int
		errorCount        int
	)
	t.Log("Launching goroutines")
	goroutineLaunchWg.Add(1)
	for _, backingFile := range backingFiles {
		go (func(fp *os.File) {
			// Wait until all goroutines have been spawned before attempting
			// to attach to a loop device. This increases the likelihood of
			// triggering a race condition.
			goroutineLaunchWg.Wait()

			loopFile, err := openNextAvailableLoopback(fp)
			if err == nil {
				err = setAutoClear(loopFile)
				loopFile.Close()
			}
			goroutineReturn <- err
		})(backingFile)
	}
	goroutineLaunchWg.Done()

	t.Log("Getting errors")
	for returnCount < len(backingFiles) {
		err := <- goroutineReturn
		t.Log("Got error")
		returnCount += 1

		if err != nil {
			t.Logf("Error attaching to loop device: %s", err)
			errorCount += 1
		}
	}

	if errorCount > 0 {
		t.Fail()
	}
}