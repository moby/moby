// +build linux,cgo

package loopback // import "github.com/docker/docker/pkg/loopback"

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// This is only used for debugging purposes within testing
func attachErrorStateString(aes attachErrorState) string {
	switch aes {
	case attachErrorStateNextFree:
		return "nextFree"
	case attachErrorStateMknod:
		return "mknod"
	case attachErrorStateStat:
		return "stat"
	case attachErrorStateModeCheck:
		return "modeCheck"
	case attachErrorStateOpenBlock:
		return "openBlock"
	case attachErrorStateAttachFd:
		return "attachFd"
	default:
		return "?"
	}
}

type createOnNoStatFileInfo struct {
	name string
}

const (
	createOnNoStatFileSize = 42
	createOnNoStatFileMode = os.FileMode(0654)
)

func (fi *createOnNoStatFileInfo) Name() string {
	return fi.name
}

func (fi *createOnNoStatFileInfo) Size() int64 {
	return createOnNoStatFileSize
}

func (fi *createOnNoStatFileInfo) Mode() os.FileMode {
	return createOnNoStatFileMode | os.ModeDevice
}

func (fi *createOnNoStatFileInfo) ModTime() time.Time {
	return time.Now()
}

func (fi *createOnNoStatFileInfo) IsDir() bool {
	return false
}

func (fi *createOnNoStatFileInfo) Sys() interface{} {
	return nil
}

type createOnNoStatModuleContext struct {
	testContext                 *testing.T
	pathStatCallCount           int
	nextFreeDeviceIndexCount    int
	baseDeviceNodeStatCount     int
	openSysfsParameterFileCount int
	maxPartitionParameterCount  int
	partitionShiftCount         int
	mknodDeviceNumberCount      int
	performMknodCount           int
	makeIndexNodeCount          int
	openDeviceFileCount         int
	setLoopFileFdCount          int
	sentinelLoopFile            os.File
	sentinelSparseFile          os.File
}

func (ctx *createOnNoStatModuleContext) PerformPathStat(path string) (os.FileInfo, error) {
	ctx.pathStatCallCount++
	if ctx.performMknodCount <= 0 {
		return nil, os.ErrNotExist
	}
	return &createOnNoStatFileInfo{name: path}, nil
}

func (ctx *createOnNoStatModuleContext) GetNextFreeDeviceIndex() (int, error) {
	ctx.nextFreeDeviceIndexCount++
	return 0, nil
}

func (ctx *createOnNoStatModuleContext) GetBaseDeviceNodeStat() (*syscall.Stat_t, error) {
	ctx.baseDeviceNodeStatCount++
	return &syscall.Stat_t{
		Uid: 0,
		Gid: 0,
		Mode: 0640,
	}, nil
}

func (ctx *createOnNoStatModuleContext) OpenSysfsParameterFile(param string) (io.ReadCloser, error) {
	// keep
	succ, err := openLoopModuleSysfsParameter(param)
	if err != nil {
		ctx.testContext.Logf("Error in openLoopModuleSysfsParameter while testing: %s", err)
	}
	ctx.openSysfsParameterFileCount++
	return succ, err
}

func (ctx *createOnNoStatModuleContext) GetMaxPartitionParameter() (uint, error) {
	// keep
	succ, err := getMaxPartitionParameter(ctx)
	ctx.maxPartitionParameterCount++
	return succ, err
}

func (ctx *createOnNoStatModuleContext) GetPartitionShift() (uint, error) {
	// keep
	succ, err := getPartitionShift(ctx)
	ctx.partitionShiftCount++
	return succ, err
}

func (ctx *createOnNoStatModuleContext) GetMknodDeviceNumber(index int) (int, error) {
	// keep
	succ, err := getMknodDeviceNumber(ctx, index)
	if err != nil {
		ctx.testContext.Logf("Error in getMknodDeviceNumber while testing: %s", err)
	}
	ctx.mknodDeviceNumberCount++
	return succ, err
}

func (ctx *createOnNoStatModuleContext) PerformMknod(path string, mode uint32, dev int) error {
	ctx.performMknodCount++
	return nil
}

func (ctx *createOnNoStatModuleContext) MakeIndexNode(index int) (os.FileInfo, error) {
	// keep
	succ, err := directIndexMknod(ctx, index)
	ctx.makeIndexNodeCount++
	return succ, err
}

func (ctx *createOnNoStatModuleContext) OpenDeviceFile(path string) (*os.File, error) {
	ctx.openDeviceFileCount++
	return &ctx.sentinelLoopFile, nil
}

func (ctx *createOnNoStatModuleContext) SetLoopFileFd(loopFile *os.File, sparseFile *os.File) error {
	ctx.setLoopFileFdCount++
	return nil
}

func (ctx *createOnNoStatModuleContext) AttachToNextAvailableDevice(sparseFile *os.File) (*os.File, int, *attachError) {
	return attachNextAvailableDevice(ctx, sparseFile)
}

func TestCreateOnNoStat(t *testing.T) {
	modCtx := &createOnNoStatModuleContext{testContext: t}
	loopFile, created, err := modCtx.AttachToNextAvailableDevice(&modCtx.sentinelSparseFile)
	if err != nil {
		t.Fatalf("Error in AttachToNextAvailableDevice at state %s: %s", attachErrorStateString(err.atState), err)
	}
	if modCtx.nextFreeDeviceIndexCount <= 0 {
		t.Fatal("modCtx.GetNextFreeDeviceIndex was never called")
	}
	// The mock context always asserts that the first available loop index
	// is 0, but starts off by stating that it doesn't exist on the filesystem.
	// So the actual code should "create" it.
	if created != 0 {
		t.Fatalf("Expected 'created' to be 0, got %d", created)
	}
	if modCtx.makeIndexNodeCount <= 0 {
		t.Fatal("modCtx.MakeIndexNode was never called")
	}
	if modCtx.setLoopFileFdCount <= 0 {
		t.Fatal("modCtx.SetLoopFileFd was never called")
	}
	if loopFile != &modCtx.sentinelLoopFile {
		t.Fatal("Received unexpected device file pointer")
	}
}

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
			t.Logf("Error opening next loop device at state %s: %s", attachErrorStateString(typedErr.atState), typedErr.underlying)
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
		goroutineReturn   = make(chan error)
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
		returnCount++

		if err != nil {
			t.Logf("Error attaching to loop device: %s", err)
			errorCount++
		}
	}

	if errorCount > 0 {
		t.Fail()
	}
}