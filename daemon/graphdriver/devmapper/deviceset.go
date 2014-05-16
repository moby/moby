// +build linux,amd64

package devmapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/dotcloud/docker/pkg/label"
	"github.com/dotcloud/docker/utils"
)

var (
	DefaultDataLoopbackSize     int64  = 100 * 1024 * 1024 * 1024
	DefaultMetaDataLoopbackSize int64  = 2 * 1024 * 1024 * 1024
	DefaultBaseFsSize           uint64 = 10 * 1024 * 1024 * 1024
)

type DevInfo struct {
	Hash          string     `json:"-"`
	DeviceId      int        `json:"device_id"`
	Size          uint64     `json:"size"`
	TransactionId uint64     `json:"transaction_id"`
	Initialized   bool       `json:"initialized"`
	devices       *DeviceSet `json:"-"`

	mountCount int    `json:"-"`
	mountPath  string `json:"-"`

	// The global DeviceSet lock guarantees that we serialize all
	// the calls to libdevmapper (which is not threadsafe), but we
	// sometimes release that lock while sleeping. In that case
	// this per-device lock is still held, protecting against
	// other accesses to the device that we're doing the wait on.
	//
	// WARNING: In order to avoid AB-BA deadlocks when releasing
	// the global lock while holding the per-device locks all
	// device locks must be aquired *before* the device lock, and
	// multiple device locks should be aquired parent before child.
	lock sync.Mutex `json:"-"`
}

type MetaData struct {
	Devices     map[string]*DevInfo `json:devices`
	devicesLock sync.Mutex          `json:"-"` // Protects all read/writes to Devices map
}

type DeviceSet struct {
	MetaData
	sync.Mutex       // Protects Devices map and serializes calls into libdevmapper
	root             string
	devicePrefix     string
	TransactionId    uint64
	NewTransactionId uint64
	nextDeviceId     int
}

type DiskUsage struct {
	Used  uint64
	Total uint64
}

type Status struct {
	PoolName         string
	DataLoopback     string
	MetadataLoopback string
	Data             DiskUsage
	Metadata         DiskUsage
	SectorSize       uint64
}

type DevStatus struct {
	DeviceId            int
	Size                uint64
	TransactionId       uint64
	SizeInSectors       uint64
	MappedSectors       uint64
	HighestMappedSector uint64
}

func getDevName(name string) string {
	return "/dev/mapper/" + name
}

func (info *DevInfo) Name() string {
	hash := info.Hash
	if hash == "" {
		hash = "base"
	}
	return fmt.Sprintf("%s-%s", info.devices.devicePrefix, hash)
}

func (info *DevInfo) DevName() string {
	return getDevName(info.Name())
}

func (devices *DeviceSet) loopbackDir() string {
	return path.Join(devices.root, "devicemapper")
}

func (devices *DeviceSet) metadataDir() string {
	return path.Join(devices.root, "metadata")
}

func (devices *DeviceSet) metadataFile(info *DevInfo) string {
	file := info.Hash
	if file == "" {
		file = "base"
	}
	return path.Join(devices.metadataDir(), file)
}

func (devices *DeviceSet) oldMetadataFile() string {
	return path.Join(devices.loopbackDir(), "json")
}

func (devices *DeviceSet) getPoolName() string {
	return devices.devicePrefix + "-pool"
}

func (devices *DeviceSet) getPoolDevName() string {
	return getDevName(devices.getPoolName())
}

func (devices *DeviceSet) hasImage(name string) bool {
	dirname := devices.loopbackDir()
	filename := path.Join(dirname, name)

	_, err := os.Stat(filename)
	return err == nil
}

// ensureImage creates a sparse file of <size> bytes at the path
// <root>/devicemapper/<name>.
// If the file already exists, it does nothing.
// Either way it returns the full path.
func (devices *DeviceSet) ensureImage(name string, size int64) (string, error) {
	dirname := devices.loopbackDir()
	filename := path.Join(dirname, name)

	if err := os.MkdirAll(dirname, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}

	if _, err := os.Stat(filename); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		utils.Debugf("Creating loopback file %s for device-manage use", filename)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if err = file.Truncate(size); err != nil {
			return "", err
		}
	}
	return filename, nil
}

func (devices *DeviceSet) allocateTransactionId() uint64 {
	devices.NewTransactionId = devices.NewTransactionId + 1
	return devices.NewTransactionId
}

func (devices *DeviceSet) removeMetadata(info *DevInfo) error {
	if err := os.RemoveAll(devices.metadataFile(info)); err != nil {
		return fmt.Errorf("Error removing metadata file %s: %s", devices.metadataFile(info), err)
	}
	return nil
}

func (devices *DeviceSet) saveMetadata(info *DevInfo) error {
	jsonData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("Error encoding metadata to json: %s", err)
	}
	tmpFile, err := ioutil.TempFile(devices.metadataDir(), ".tmp")
	if err != nil {
		return fmt.Errorf("Error creating metadata file: %s", err)
	}

	n, err := tmpFile.Write(jsonData)
	if err != nil {
		return fmt.Errorf("Error writing metadata to %s: %s", tmpFile.Name(), err)
	}
	if n < len(jsonData) {
		return io.ErrShortWrite
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("Error syncing metadata file %s: %s", tmpFile.Name(), err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("Error closing metadata file %s: %s", tmpFile.Name(), err)
	}
	if err := os.Rename(tmpFile.Name(), devices.metadataFile(info)); err != nil {
		return fmt.Errorf("Error committing metadata file %s: %s", tmpFile.Name(), err)
	}

	if devices.NewTransactionId != devices.TransactionId {
		if err = setTransactionId(devices.getPoolDevName(), devices.TransactionId, devices.NewTransactionId); err != nil {
			return fmt.Errorf("Error setting devmapper transition ID: %s", err)
		}
		devices.TransactionId = devices.NewTransactionId
	}
	return nil
}

func (devices *DeviceSet) lookupDevice(hash string) (*DevInfo, error) {
	devices.devicesLock.Lock()
	defer devices.devicesLock.Unlock()
	info := devices.Devices[hash]
	if info == nil {
		info = devices.loadMetadata(hash)
		if info == nil {
			return nil, fmt.Errorf("Unknown device %s", hash)
		}

		devices.Devices[hash] = info
	}
	return info, nil
}

func (devices *DeviceSet) registerDevice(id int, hash string, size uint64) (*DevInfo, error) {
	utils.Debugf("registerDevice(%v, %v)", id, hash)
	info := &DevInfo{
		Hash:          hash,
		DeviceId:      id,
		Size:          size,
		TransactionId: devices.allocateTransactionId(),
		Initialized:   false,
		devices:       devices,
	}

	devices.devicesLock.Lock()
	devices.Devices[hash] = info
	devices.devicesLock.Unlock()

	if err := devices.saveMetadata(info); err != nil {
		// Try to remove unused device
		devices.devicesLock.Lock()
		delete(devices.Devices, hash)
		devices.devicesLock.Unlock()
		return nil, err
	}

	return info, nil
}

func (devices *DeviceSet) activateDeviceIfNeeded(info *DevInfo) error {
	utils.Debugf("activateDeviceIfNeeded(%v)", info.Hash)

	if devinfo, _ := getInfo(info.Name()); devinfo != nil && devinfo.Exists != 0 {
		return nil
	}

	return activateDevice(devices.getPoolDevName(), info.Name(), info.DeviceId, info.Size)
}

func (devices *DeviceSet) createFilesystem(info *DevInfo) error {
	devname := info.DevName()

	err := exec.Command("mkfs.ext4", "-E", "discard,lazy_itable_init=0,lazy_journal_init=0", devname).Run()
	if err != nil {
		err = exec.Command("mkfs.ext4", "-E", "discard,lazy_itable_init=0", devname).Run()
	}
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	return nil
}

func (devices *DeviceSet) initMetaData() error {
	_, _, _, params, err := getStatus(devices.getPoolName())
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if _, err := fmt.Sscanf(params, "%d", &devices.TransactionId); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	devices.NewTransactionId = devices.TransactionId

	// Migrate old metadatafile

	jsonData, err := ioutil.ReadFile(devices.oldMetadataFile())
	if err != nil && !os.IsNotExist(err) {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if jsonData != nil {
		m := MetaData{Devices: make(map[string]*DevInfo)}

		if err := json.Unmarshal(jsonData, &m); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}

		for hash, info := range m.Devices {
			info.Hash = hash

			// If the transaction id is larger than the actual one we lost the device due to some crash
			if info.TransactionId <= devices.TransactionId {
				devices.saveMetadata(info)
			}
		}
		if err := os.Rename(devices.oldMetadataFile(), devices.oldMetadataFile()+".migrated"); err != nil {
			return err
		}

	}

	return nil
}

func (devices *DeviceSet) loadMetadata(hash string) *DevInfo {
	info := &DevInfo{Hash: hash, devices: devices}

	jsonData, err := ioutil.ReadFile(devices.metadataFile(info))
	if err != nil {
		return nil
	}

	if err := json.Unmarshal(jsonData, &info); err != nil {
		return nil
	}

	// If the transaction id is larger than the actual one we lost the device due to some crash
	if info.TransactionId > devices.TransactionId {
		return nil
	}

	return info
}

func (devices *DeviceSet) setupBaseImage() error {
	oldInfo, _ := devices.lookupDevice("")
	if oldInfo != nil && oldInfo.Initialized {
		return nil
	}

	if oldInfo != nil && !oldInfo.Initialized {
		utils.Debugf("Removing uninitialized base image")
		if err := devices.deleteDevice(oldInfo); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	utils.Debugf("Initializing base device-manager snapshot")

	id := devices.nextDeviceId

	// Create initial device
	if err := createDevice(devices.getPoolDevName(), &id); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	// Ids are 24bit, so wrap around
	devices.nextDeviceId = (id + 1) & 0xffffff

	utils.Debugf("Registering base device (id %v) with FS size %v", id, DefaultBaseFsSize)
	info, err := devices.registerDevice(id, "", DefaultBaseFsSize)
	if err != nil {
		_ = deleteDevice(devices.getPoolDevName(), id)
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	utils.Debugf("Creating filesystem on base device-manager snapshot")

	if err = devices.activateDeviceIfNeeded(info); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if err := devices.createFilesystem(info); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	info.Initialized = true
	if err = devices.saveMetadata(info); err != nil {
		info.Initialized = false
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	return nil
}

func setCloseOnExec(name string) {
	if fileInfos, _ := ioutil.ReadDir("/proc/self/fd"); fileInfos != nil {
		for _, i := range fileInfos {
			link, _ := os.Readlink(filepath.Join("/proc/self/fd", i.Name()))
			if link == name {
				fd, err := strconv.Atoi(i.Name())
				if err == nil {
					syscall.CloseOnExec(fd)
				}
			}
		}
	}
}

func (devices *DeviceSet) log(level int, file string, line int, dmError int, message string) {
	if level >= 7 {
		return // Ignore _LOG_DEBUG
	}

	utils.Debugf("libdevmapper(%d): %s:%d (%d) %s", level, file, line, dmError, message)
}

func major(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

func (devices *DeviceSet) ResizePool(size int64) error {
	dirname := devices.loopbackDir()
	datafilename := path.Join(dirname, "data")
	metadatafilename := path.Join(dirname, "metadata")

	datafile, err := os.OpenFile(datafilename, os.O_RDWR, 0)
	if datafile == nil {
		return err
	}
	defer datafile.Close()

	fi, err := datafile.Stat()
	if fi == nil {
		return err
	}

	if fi.Size() > size {
		return fmt.Errorf("Can't shrink file")
	}

	dataloopback := FindLoopDeviceFor(datafile)
	if dataloopback == nil {
		return fmt.Errorf("Unable to find loopback mount for: %s", datafilename)
	}
	defer dataloopback.Close()

	metadatafile, err := os.OpenFile(metadatafilename, os.O_RDWR, 0)
	if metadatafile == nil {
		return err
	}
	defer metadatafile.Close()

	metadataloopback := FindLoopDeviceFor(metadatafile)
	if metadataloopback == nil {
		return fmt.Errorf("Unable to find loopback mount for: %s", metadatafilename)
	}
	defer metadataloopback.Close()

	// Grow loopback file
	if err := datafile.Truncate(size); err != nil {
		return fmt.Errorf("Unable to grow loopback file: %s", err)
	}

	// Reload size for loopback device
	if err := LoopbackSetCapacity(dataloopback); err != nil {
		return fmt.Errorf("Unable to update loopback capacity: %s", err)
	}

	// Suspend the pool
	if err := suspendDevice(devices.getPoolName()); err != nil {
		return fmt.Errorf("Unable to suspend pool: %s", err)
	}

	// Reload with the new block sizes
	if err := reloadPool(devices.getPoolName(), dataloopback, metadataloopback); err != nil {
		return fmt.Errorf("Unable to reload pool: %s", err)
	}

	// Resume the pool
	if err := resumeDevice(devices.getPoolName()); err != nil {
		return fmt.Errorf("Unable to resume pool: %s", err)
	}

	return nil
}

func (devices *DeviceSet) initDevmapper(doInit bool) error {
	logInit(devices)

	if err := os.MkdirAll(devices.metadataDir(), 0700); err != nil && !os.IsExist(err) {
		return err
	}

	// Set the device prefix from the device id and inode of the docker root dir

	st, err := os.Stat(devices.root)
	if err != nil {
		return fmt.Errorf("Error looking up dir %s: %s", devices.root, err)
	}
	sysSt := st.Sys().(*syscall.Stat_t)
	// "reg-" stands for "regular file".
	// In the future we might use "dev-" for "device file", etc.
	// docker-maj,min[-inode] stands for:
	//	- Managed by docker
	//	- The target of this device is at major <maj> and minor <min>
	//	- If <inode> is defined, use that file inside the device as a loopback image. Otherwise use the device itself.
	devices.devicePrefix = fmt.Sprintf("docker-%d:%d-%d", major(sysSt.Dev), minor(sysSt.Dev), sysSt.Ino)
	utils.Debugf("Generated prefix: %s", devices.devicePrefix)

	// Check for the existence of the device <prefix>-pool
	utils.Debugf("Checking for existence of the pool '%s'", devices.getPoolName())
	info, err := getInfo(devices.getPoolName())
	if info == nil {
		utils.Debugf("Error device getInfo: %s", err)
		return err
	}

	// It seems libdevmapper opens this without O_CLOEXEC, and go exec will not close files
	// that are not Close-on-exec, and lxc-start will die if it inherits any unexpected files,
	// so we add this badhack to make sure it closes itself
	setCloseOnExec("/dev/mapper/control")

	// Make sure the sparse images exist in <root>/devicemapper/data and
	// <root>/devicemapper/metadata

	createdLoopback := false

	// If the pool doesn't exist, create it
	if info.Exists == 0 {
		utils.Debugf("Pool doesn't exist. Creating it.")

		hasData := devices.hasImage("data")
		hasMetadata := devices.hasImage("metadata")

		if !doInit && !hasData {
			return errors.New("Loopback data file not found")
		}

		if !doInit && !hasMetadata {
			return errors.New("Loopback metadata file not found")
		}

		createdLoopback = !hasData || !hasMetadata
		data, err := devices.ensureImage("data", DefaultDataLoopbackSize)
		if err != nil {
			utils.Debugf("Error device ensureImage (data): %s\n", err)
			return err
		}
		metadata, err := devices.ensureImage("metadata", DefaultMetaDataLoopbackSize)
		if err != nil {
			utils.Debugf("Error device ensureImage (metadata): %s\n", err)
			return err
		}

		dataFile, err := attachLoopDevice(data)
		if err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
		defer dataFile.Close()

		metadataFile, err := attachLoopDevice(metadata)
		if err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
		defer metadataFile.Close()

		if err := createPool(devices.getPoolName(), dataFile, metadataFile); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	// If we didn't just create the data or metadata image, we need to
	// load the transaction id and migrate old metadata
	if !createdLoopback {
		if err = devices.initMetaData(); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	// Setup the base image
	if doInit {
		if err := devices.setupBaseImage(); err != nil {
			utils.Debugf("Error device setupBaseImage: %s\n", err)
			return err
		}
	}

	return nil
}

func (devices *DeviceSet) AddDevice(hash, baseHash string) error {
	baseInfo, err := devices.lookupDevice(baseHash)
	if err != nil {
		return err
	}

	baseInfo.lock.Lock()
	defer baseInfo.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	if info, _ := devices.lookupDevice(hash); info != nil {
		return fmt.Errorf("device %s already exists", hash)
	}

	deviceId := devices.nextDeviceId

	if err := createSnapDevice(devices.getPoolDevName(), &deviceId, baseInfo.Name(), baseInfo.DeviceId); err != nil {
		utils.Debugf("Error creating snap device: %s\n", err)
		return err
	}

	// Ids are 24bit, so wrap around
	devices.nextDeviceId = (deviceId + 1) & 0xffffff

	if _, err := devices.registerDevice(deviceId, hash, baseInfo.Size); err != nil {
		deleteDevice(devices.getPoolDevName(), deviceId)
		utils.Debugf("Error registering device: %s\n", err)
		return err
	}
	return nil
}

func (devices *DeviceSet) deleteDevice(info *DevInfo) error {
	// This is a workaround for the kernel not discarding block so
	// on the thin pool when we remove a thinp device, so we do it
	// manually
	if err := devices.activateDeviceIfNeeded(info); err == nil {
		if err := BlockDeviceDiscard(info.DevName()); err != nil {
			utils.Debugf("Error discarding block on device: %s (ignoring)\n", err)
		}
	}

	devinfo, _ := getInfo(info.Name())
	if devinfo != nil && devinfo.Exists != 0 {
		if err := devices.removeDeviceAndWait(info.Name()); err != nil {
			utils.Debugf("Error removing device: %s\n", err)
			return err
		}
	}

	if err := deleteDevice(devices.getPoolDevName(), info.DeviceId); err != nil {
		utils.Debugf("Error deleting device: %s\n", err)
		return err
	}

	devices.allocateTransactionId()
	devices.devicesLock.Lock()
	delete(devices.Devices, info.Hash)
	devices.devicesLock.Unlock()

	if err := devices.removeMetadata(info); err != nil {
		devices.devicesLock.Lock()
		devices.Devices[info.Hash] = info
		devices.devicesLock.Unlock()
		utils.Debugf("Error removing meta data: %s\n", err)
		return err
	}

	return nil
}

func (devices *DeviceSet) DeleteDevice(hash string) error {
	info, err := devices.lookupDevice(hash)
	if err != nil {
		return err
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	return devices.deleteDevice(info)
}

func (devices *DeviceSet) deactivatePool() error {
	utils.Debugf("[devmapper] deactivatePool()")
	defer utils.Debugf("[devmapper] deactivatePool END")
	devname := devices.getPoolDevName()
	devinfo, err := getInfo(devname)
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if devinfo.Exists != 0 {
		return removeDevice(devname)
	}

	return nil
}

func (devices *DeviceSet) deactivateDevice(info *DevInfo) error {
	utils.Debugf("[devmapper] deactivateDevice(%s)", info.Hash)
	defer utils.Debugf("[devmapper] deactivateDevice END")

	// Wait for the unmount to be effective,
	// by watching the value of Info.OpenCount for the device
	if err := devices.waitClose(info); err != nil {
		utils.Errorf("Warning: error waiting for device %s to close: %s\n", info.Hash, err)
	}

	devinfo, err := getInfo(info.Name())
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if devinfo.Exists != 0 {
		if err := devices.removeDeviceAndWait(info.Name()); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	return nil
}

// Issues the underlying dm remove operation and then waits
// for it to finish.
func (devices *DeviceSet) removeDeviceAndWait(devname string) error {
	var err error

	for i := 0; i < 1000; i++ {
		err = removeDevice(devname)
		if err == nil {
			break
		}
		if err != ErrBusy {
			return err
		}

		// If we see EBUSY it may be a transient error,
		// sleep a bit a retry a few times.
		devices.Unlock()
		time.Sleep(10 * time.Millisecond)
		devices.Lock()
	}
	if err != nil {
		return err
	}

	if err := devices.waitRemove(devname); err != nil {
		return err
	}
	return nil
}

// waitRemove blocks until either:
// a) the device registered at <device_set_prefix>-<hash> is removed,
// or b) the 10 second timeout expires.
func (devices *DeviceSet) waitRemove(devname string) error {
	utils.Debugf("[deviceset %s] waitRemove(%s)", devices.devicePrefix, devname)
	defer utils.Debugf("[deviceset %s] waitRemove(%s) END", devices.devicePrefix, devname)
	i := 0
	for ; i < 1000; i += 1 {
		devinfo, err := getInfo(devname)
		if err != nil {
			// If there is an error we assume the device doesn't exist.
			// The error might actually be something else, but we can't differentiate.
			return nil
		}
		if i%100 == 0 {
			utils.Debugf("Waiting for removal of %s: exists=%d", devname, devinfo.Exists)
		}
		if devinfo.Exists == 0 {
			break
		}

		devices.Unlock()
		time.Sleep(10 * time.Millisecond)
		devices.Lock()
	}
	if i == 1000 {
		return fmt.Errorf("Timeout while waiting for device %s to be removed", devname)
	}
	return nil
}

// waitClose blocks until either:
// a) the device registered at <device_set_prefix>-<hash> is closed,
// or b) the 10 second timeout expires.
func (devices *DeviceSet) waitClose(info *DevInfo) error {
	i := 0
	for ; i < 1000; i += 1 {
		devinfo, err := getInfo(info.Name())
		if err != nil {
			return err
		}
		if i%100 == 0 {
			utils.Debugf("Waiting for unmount of %s: opencount=%d", info.Hash, devinfo.OpenCount)
		}
		if devinfo.OpenCount == 0 {
			break
		}
		devices.Unlock()
		time.Sleep(10 * time.Millisecond)
		devices.Lock()
	}
	if i == 1000 {
		return fmt.Errorf("Timeout while waiting for device %s to close", info.Hash)
	}
	return nil
}

func (devices *DeviceSet) Shutdown() error {

	utils.Debugf("[deviceset %s] shutdown()", devices.devicePrefix)
	utils.Debugf("[devmapper] Shutting down DeviceSet: %s", devices.root)
	defer utils.Debugf("[deviceset %s] shutdown END", devices.devicePrefix)

	var devs []*DevInfo

	devices.devicesLock.Lock()
	for _, info := range devices.Devices {
		devs = append(devs, info)
	}
	devices.devicesLock.Unlock()

	for _, info := range devs {
		info.lock.Lock()
		if info.mountCount > 0 {
			// We use MNT_DETACH here in case it is still busy in some running
			// container. This means it'll go away from the global scope directly,
			// and the device will be released when that container dies.
			if err := syscall.Unmount(info.mountPath, syscall.MNT_DETACH); err != nil {
				utils.Debugf("Shutdown unmounting %s, error: %s\n", info.mountPath, err)
			}

			devices.Lock()
			if err := devices.deactivateDevice(info); err != nil {
				utils.Debugf("Shutdown deactivate %s , error: %s\n", info.Hash, err)
			}
			devices.Unlock()
		}
		info.lock.Unlock()
	}

	info, _ := devices.lookupDevice("")
	if info != nil {
		info.lock.Lock()
		devices.Lock()
		if err := devices.deactivateDevice(info); err != nil {
			utils.Debugf("Shutdown deactivate base , error: %s\n", err)
		}
		devices.Unlock()
		info.lock.Unlock()
	}

	devices.Lock()
	if err := devices.deactivatePool(); err != nil {
		utils.Debugf("Shutdown deactivate pool , error: %s\n", err)
	}
	devices.Unlock()

	return nil
}

func (devices *DeviceSet) MountDevice(hash, path, mountLabel string) error {
	info, err := devices.lookupDevice(hash)
	if err != nil {
		return err
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	if info.mountCount > 0 {
		if path != info.mountPath {
			return fmt.Errorf("Trying to mount devmapper device in multple places (%s, %s)", info.mountPath, path)
		}

		info.mountCount++
		return nil
	}

	if err := devices.activateDeviceIfNeeded(info); err != nil {
		return fmt.Errorf("Error activating devmapper device for '%s': %s", hash, err)
	}

	var flags uintptr = syscall.MS_MGC_VAL

	mountOptions := label.FormatMountLabel("discard", mountLabel)
	err = syscall.Mount(info.DevName(), path, "ext4", flags, mountOptions)
	if err != nil && err == syscall.EINVAL {
		mountOptions = label.FormatMountLabel("", mountLabel)
		err = syscall.Mount(info.DevName(), path, "ext4", flags, mountOptions)
	}
	if err != nil {
		return fmt.Errorf("Error mounting '%s' on '%s': %s", info.DevName(), path, err)
	}

	info.mountCount = 1
	info.mountPath = path

	return nil
}

func (devices *DeviceSet) UnmountDevice(hash string) error {
	utils.Debugf("[devmapper] UnmountDevice(hash=%s)", hash)
	defer utils.Debugf("[devmapper] UnmountDevice END")

	info, err := devices.lookupDevice(hash)
	if err != nil {
		return err
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	if info.mountCount == 0 {
		return fmt.Errorf("UnmountDevice: device not-mounted id %s\n", hash)
	}

	info.mountCount--
	if info.mountCount > 0 {
		return nil
	}

	utils.Debugf("[devmapper] Unmount(%s)", info.mountPath)
	if err := syscall.Unmount(info.mountPath, 0); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	utils.Debugf("[devmapper] Unmount done")

	if err := devices.deactivateDevice(info); err != nil {
		return err
	}

	info.mountPath = ""

	return nil
}

func (devices *DeviceSet) HasDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	info, _ := devices.lookupDevice(hash)
	return info != nil
}

func (devices *DeviceSet) HasActivatedDevice(hash string) bool {
	info, _ := devices.lookupDevice(hash)
	if info == nil {
		return false
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	devinfo, _ := getInfo(info.Name())
	return devinfo != nil && devinfo.Exists != 0
}

func (devices *DeviceSet) List() []string {
	devices.Lock()
	defer devices.Unlock()

	devices.devicesLock.Lock()
	ids := make([]string, len(devices.Devices))
	i := 0
	for k := range devices.Devices {
		ids[i] = k
		i++
	}
	devices.devicesLock.Unlock()

	return ids
}

func (devices *DeviceSet) deviceStatus(devName string) (sizeInSectors, mappedSectors, highestMappedSector uint64, err error) {
	var params string
	_, sizeInSectors, _, params, err = getStatus(devName)
	if err != nil {
		return
	}
	if _, err = fmt.Sscanf(params, "%d %d", &mappedSectors, &highestMappedSector); err == nil {
		return
	}
	return
}

func (devices *DeviceSet) GetDeviceStatus(hash string) (*DevStatus, error) {
	info, err := devices.lookupDevice(hash)
	if err != nil {
		return nil, err
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devices.Lock()
	defer devices.Unlock()

	status := &DevStatus{
		DeviceId:      info.DeviceId,
		Size:          info.Size,
		TransactionId: info.TransactionId,
	}

	if err := devices.activateDeviceIfNeeded(info); err != nil {
		return nil, fmt.Errorf("Error activating devmapper device for '%s': %s", hash, err)
	}

	if sizeInSectors, mappedSectors, highestMappedSector, err := devices.deviceStatus(info.DevName()); err != nil {
		return nil, err
	} else {
		status.SizeInSectors = sizeInSectors
		status.MappedSectors = mappedSectors
		status.HighestMappedSector = highestMappedSector
	}

	return status, nil
}

func (devices *DeviceSet) poolStatus() (totalSizeInSectors, transactionId, dataUsed, dataTotal, metadataUsed, metadataTotal uint64, err error) {
	var params string
	if _, totalSizeInSectors, _, params, err = getStatus(devices.getPoolName()); err == nil {
		_, err = fmt.Sscanf(params, "%d %d/%d %d/%d", &transactionId, &metadataUsed, &metadataTotal, &dataUsed, &dataTotal)
	}
	return
}

func (devices *DeviceSet) Status() *Status {
	devices.Lock()
	defer devices.Unlock()

	status := &Status{}

	status.PoolName = devices.getPoolName()
	status.DataLoopback = path.Join(devices.loopbackDir(), "data")
	status.MetadataLoopback = path.Join(devices.loopbackDir(), "metadata")

	totalSizeInSectors, _, dataUsed, dataTotal, metadataUsed, metadataTotal, err := devices.poolStatus()
	if err == nil {
		// Convert from blocks to bytes
		blockSizeInSectors := totalSizeInSectors / dataTotal

		status.Data.Used = dataUsed * blockSizeInSectors * 512
		status.Data.Total = dataTotal * blockSizeInSectors * 512

		// metadata blocks are always 4k
		status.Metadata.Used = metadataUsed * 4096
		status.Metadata.Total = metadataTotal * 4096

		status.SectorSize = blockSizeInSectors * 512
	}

	return status
}

func NewDeviceSet(root string, doInit bool) (*DeviceSet, error) {
	SetDevDir("/dev")

	devices := &DeviceSet{
		root:     root,
		MetaData: MetaData{Devices: make(map[string]*DevInfo)},
	}

	if err := devices.initDevmapper(doInit); err != nil {
		return nil, err
	}

	return devices, nil
}
