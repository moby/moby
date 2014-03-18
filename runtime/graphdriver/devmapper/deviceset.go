// +build linux,amd64

package devmapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
	// A floating mount means one reference is not owned and
	// will be stolen by the next mount. This allows us to
	// avoid unmounting directly after creation before the
	// first get (since we need to mount to set up the device
	// a bit first).
	floating bool `json:"-"`

	// The global DeviceSet lock guarantees that we serialize all
	// the calls to libdevmapper (which is not threadsafe), but we
	// sometimes release that lock while sleeping. In that case
	// this per-device lock is still held, protecting against
	// other accesses to the device that we're doing the wait on.
	lock sync.Mutex `json:"-"`
}

type MetaData struct {
	Devices map[string]*DevInfo `json:devices`
}

type DeviceSet struct {
	MetaData
	sync.Mutex       // Protects Devices map and serializes calls into libdevmapper
	root             string
	devicePrefix     string
	TransactionId    uint64
	NewTransactionId uint64
	nextFreeDevice   int
	sawBusy          bool
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

type UnmountMode int

const (
	UnmountRegular UnmountMode = iota
	UnmountFloat
	UnmountSink
)

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

func (devices *DeviceSet) jsonFile() string {
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

	_, err := osStat(filename)
	return err == nil
}

// ensureImage creates a sparse file of <size> bytes at the path
// <root>/devicemapper/<name>.
// If the file already exists, it does nothing.
// Either way it returns the full path.
func (devices *DeviceSet) ensureImage(name string, size int64) (string, error) {
	dirname := devices.loopbackDir()
	filename := path.Join(dirname, name)

	if err := osMkdirAll(dirname, 0700); err != nil && !osIsExist(err) {
		return "", err
	}

	if _, err := osStat(filename); err != nil {
		if !osIsNotExist(err) {
			return "", err
		}
		utils.Debugf("Creating loopback file %s for device-manage use", filename)
		file, err := osOpenFile(filename, osORdWr|osOCreate, 0600)
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

func (devices *DeviceSet) allocateDeviceId() int {
	// TODO: Add smarter reuse of deleted devices
	id := devices.nextFreeDevice
	devices.nextFreeDevice = devices.nextFreeDevice + 1
	return id
}

func (devices *DeviceSet) allocateTransactionId() uint64 {
	devices.NewTransactionId = devices.NewTransactionId + 1
	return devices.NewTransactionId
}

func (devices *DeviceSet) saveMetadata() error {
	jsonData, err := json.Marshal(devices.MetaData)
	if err != nil {
		return fmt.Errorf("Error encoding metadata to json: %s", err)
	}
	tmpFile, err := ioutil.TempFile(filepath.Dir(devices.jsonFile()), ".json")
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
	if err := osRename(tmpFile.Name(), devices.jsonFile()); err != nil {
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

	devices.Devices[hash] = info
	if err := devices.saveMetadata(); err != nil {
		// Try to remove unused device
		delete(devices.Devices, hash)
		return nil, err
	}

	return info, nil
}

func (devices *DeviceSet) activateDeviceIfNeeded(hash string) error {
	utils.Debugf("activateDeviceIfNeeded(%v)", hash)
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	if devinfo, _ := getInfo(info.Name()); devinfo != nil && devinfo.Exists != 0 {
		return nil
	}

	return activateDevice(devices.getPoolDevName(), info.Name(), info.DeviceId, info.Size)
}

func (devices *DeviceSet) createFilesystem(info *DevInfo) error {
	devname := info.DevName()

	err := execRun("mkfs.ext4", "-E", "discard,lazy_itable_init=0,lazy_journal_init=0", devname)
	if err != nil {
		err = execRun("mkfs.ext4", "-E", "discard,lazy_itable_init=0", devname)
	}
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	return nil
}

func (devices *DeviceSet) loadMetaData() error {
	utils.Debugf("loadMetadata()")
	defer utils.Debugf("loadMetadata END")
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

	jsonData, err := ioutil.ReadFile(devices.jsonFile())
	if err != nil && !osIsNotExist(err) {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	devices.MetaData.Devices = make(map[string]*DevInfo)
	if jsonData != nil {
		if err := json.Unmarshal(jsonData, &devices.MetaData); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	for hash, d := range devices.Devices {
		d.Hash = hash
		d.devices = devices

		if d.DeviceId >= devices.nextFreeDevice {
			devices.nextFreeDevice = d.DeviceId + 1
		}

		// If the transaction id is larger than the actual one we lost the device due to some crash
		if d.TransactionId > devices.TransactionId {
			utils.Debugf("Removing lost device %s with id %d", hash, d.TransactionId)
			delete(devices.Devices, hash)
		}
	}
	return nil
}

func (devices *DeviceSet) setupBaseImage() error {
	oldInfo := devices.Devices[""]
	if oldInfo != nil && oldInfo.Initialized {
		return nil
	}

	if oldInfo != nil && !oldInfo.Initialized {
		utils.Debugf("Removing uninitialized base image")
		if err := devices.deleteDevice(""); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	utils.Debugf("Initializing base device-manager snapshot")

	id := devices.allocateDeviceId()

	// Create initial device
	if err := createDevice(devices.getPoolDevName(), id); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	utils.Debugf("Registering base device (id %v) with FS size %v", id, DefaultBaseFsSize)
	info, err := devices.registerDevice(id, "", DefaultBaseFsSize)
	if err != nil {
		_ = deleteDevice(devices.getPoolDevName(), id)
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	utils.Debugf("Creating filesystem on base device-manager snapshot")

	if err = devices.activateDeviceIfNeeded(""); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if err := devices.createFilesystem(info); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	info.Initialized = true
	if err = devices.saveMetadata(); err != nil {
		info.Initialized = false
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	return nil
}

func setCloseOnExec(name string) {
	if fileInfos, _ := ioutil.ReadDir("/proc/self/fd"); fileInfos != nil {
		for _, i := range fileInfos {
			link, _ := osReadlink(filepath.Join("/proc/self/fd", i.Name()))
			if link == name {
				fd, err := strconv.Atoi(i.Name())
				if err == nil {
					sysCloseOnExec(fd)
				}
			}
		}
	}
}

func (devices *DeviceSet) log(level int, file string, line int, dmError int, message string) {
	if level >= 7 {
		return // Ignore _LOG_DEBUG
	}

	if strings.Contains(message, "busy") {
		devices.sawBusy = true
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

	datafile, err := osOpenFile(datafilename, osORdWr, 0)
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

	metadatafile, err := osOpenFile(metadatafilename, osORdWr, 0)
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

	// Make sure the sparse images exist in <root>/devicemapper/data and
	// <root>/devicemapper/metadata

	hasData := devices.hasImage("data")
	hasMetadata := devices.hasImage("metadata")

	if !doInit && !hasData {
		return errors.New("Loopback data file not found")
	}

	if !doInit && !hasMetadata {
		return errors.New("Loopback metadata file not found")
	}

	createdLoopback := !hasData || !hasMetadata
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

	// Set the device prefix from the device id and inode of the docker root dir

	st, err := osStat(devices.root)
	if err != nil {
		return fmt.Errorf("Error looking up dir %s: %s", devices.root, err)
	}
	sysSt := toSysStatT(st.Sys())
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

	// If the pool doesn't exist, create it
	if info.Exists == 0 {
		utils.Debugf("Pool doesn't exist. Creating it.")

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
	// load the metadata from the existing file.
	if !createdLoopback {
		if err = devices.loadMetaData(); err != nil {
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
	devices.Lock()
	defer devices.Unlock()

	if devices.Devices[hash] != nil {
		return fmt.Errorf("hash %s already exists", hash)
	}

	baseInfo := devices.Devices[baseHash]
	if baseInfo == nil {
		return fmt.Errorf("Error adding device for '%s': can't find device for parent '%s'", hash, baseHash)
	}

	baseInfo.lock.Lock()
	defer baseInfo.lock.Unlock()

	deviceId := devices.allocateDeviceId()

	if err := devices.createSnapDevice(devices.getPoolDevName(), deviceId, baseInfo.Name(), baseInfo.DeviceId); err != nil {
		utils.Debugf("Error creating snap device: %s\n", err)
		return err
	}

	if _, err := devices.registerDevice(deviceId, hash, baseInfo.Size); err != nil {
		deleteDevice(devices.getPoolDevName(), deviceId)
		utils.Debugf("Error registering device: %s\n", err)
		return err
	}
	return nil
}

func (devices *DeviceSet) deleteDevice(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("hash %s doesn't exists", hash)
	}

	// This is a workaround for the kernel not discarding block so
	// on the thin pool when we remove a thinp device, so we do it
	// manually
	if err := devices.activateDeviceIfNeeded(hash); err == nil {
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

	if info.Initialized {
		info.Initialized = false
		if err := devices.saveMetadata(); err != nil {
			utils.Debugf("Error saving meta data: %s\n", err)
			return err
		}
	}

	if err := deleteDevice(devices.getPoolDevName(), info.DeviceId); err != nil {
		utils.Debugf("Error deleting device: %s\n", err)
		return err
	}

	devices.allocateTransactionId()
	delete(devices.Devices, info.Hash)

	if err := devices.saveMetadata(); err != nil {
		devices.Devices[info.Hash] = info
		utils.Debugf("Error saving meta data: %s\n", err)
		return err
	}

	return nil
}

func (devices *DeviceSet) DeleteDevice(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	return devices.deleteDevice(hash)
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

func (devices *DeviceSet) deactivateDevice(hash string) error {
	utils.Debugf("[devmapper] deactivateDevice(%s)", hash)
	defer utils.Debugf("[devmapper] deactivateDevice END")

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
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
		devices.sawBusy = false
		err = removeDevice(devname)
		if err == nil {
			break
		}
		if !devices.sawBusy {
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
func (devices *DeviceSet) waitClose(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}
	i := 0
	for ; i < 1000; i += 1 {
		devinfo, err := getInfo(info.Name())
		if err != nil {
			return err
		}
		if i%100 == 0 {
			utils.Debugf("Waiting for unmount of %s: opencount=%d", hash, devinfo.OpenCount)
		}
		if devinfo.OpenCount == 0 {
			break
		}
		devices.Unlock()
		time.Sleep(10 * time.Millisecond)
		devices.Lock()
	}
	if i == 1000 {
		return fmt.Errorf("Timeout while waiting for device %s to close", hash)
	}
	return nil
}

func (devices *DeviceSet) Shutdown() error {
	devices.Lock()
	defer devices.Unlock()

	utils.Debugf("[deviceset %s] shutdown()", devices.devicePrefix)
	utils.Debugf("[devmapper] Shutting down DeviceSet: %s", devices.root)
	defer utils.Debugf("[deviceset %s] shutdown END", devices.devicePrefix)

	for _, info := range devices.Devices {
		info.lock.Lock()
		if info.mountCount > 0 {
			if err := sysUnmount(info.mountPath, 0); err != nil {
				utils.Debugf("Shutdown unmounting %s, error: %s\n", info.mountPath, err)
			}
		}
		info.lock.Unlock()
	}

	for _, d := range devices.Devices {
		d.lock.Lock()

		if err := devices.waitClose(d.Hash); err != nil {
			utils.Errorf("Warning: error waiting for device %s to unmount: %s\n", d.Hash, err)
		}
		if err := devices.deactivateDevice(d.Hash); err != nil {
			utils.Debugf("Shutdown deactivate %s , error: %s\n", d.Hash, err)
		}

		d.lock.Unlock()
	}

	if err := devices.deactivatePool(); err != nil {
		utils.Debugf("Shutdown deactivate pool , error: %s\n", err)
	}

	return nil
}

func (devices *DeviceSet) MountDevice(hash, path string) error {
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	if info.mountCount > 0 {
		if path != info.mountPath {
			return fmt.Errorf("Trying to mount devmapper device in multple places (%s, %s)", info.mountPath, path)
		}

		if info.floating {
			// Steal floating ref
			info.floating = false
		} else {
			info.mountCount++
		}
		return nil
	}

	if err := devices.activateDeviceIfNeeded(hash); err != nil {
		return fmt.Errorf("Error activating devmapper device for '%s': %s", hash, err)
	}

	var flags uintptr = sysMsMgcVal

	err := sysMount(info.DevName(), path, "ext4", flags, "discard")
	if err != nil && err == sysEInval {
		err = sysMount(info.DevName(), path, "ext4", flags, "")
	}
	if err != nil {
		return fmt.Errorf("Error mounting '%s' on '%s': %s", info.DevName(), path, err)
	}

	info.mountCount = 1
	info.mountPath = path
	info.floating = false

	return devices.setInitialized(hash)
}

func (devices *DeviceSet) UnmountDevice(hash string, mode UnmountMode) error {
	utils.Debugf("[devmapper] UnmountDevice(hash=%s, mode=%d)", hash, mode)
	defer utils.Debugf("[devmapper] UnmountDevice END")
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("UnmountDevice: no such device %s\n", hash)
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	if mode == UnmountFloat {
		if info.floating {
			return fmt.Errorf("UnmountDevice: can't float floating reference %s\n", hash)
		}

		// Leave this reference floating
		info.floating = true
		return nil
	}

	if mode == UnmountSink {
		if !info.floating {
			// Someone already sunk this
			return nil
		}
		// Otherwise, treat this as a regular unmount
	}

	if info.mountCount == 0 {
		return fmt.Errorf("UnmountDevice: device not-mounted id %s\n", hash)
	}

	info.mountCount--
	if info.mountCount > 0 {
		return nil
	}

	utils.Debugf("[devmapper] Unmount(%s)", info.mountPath)
	if err := sysUnmount(info.mountPath, 0); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	utils.Debugf("[devmapper] Unmount done")
	// Wait for the unmount to be effective,
	// by watching the value of Info.OpenCount for the device
	if err := devices.waitClose(hash); err != nil {
		return err
	}

	devices.deactivateDevice(hash)

	info.mountPath = ""

	return nil
}

func (devices *DeviceSet) HasDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	return devices.Devices[hash] != nil
}

func (devices *DeviceSet) HasInitializedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	return info != nil && info.Initialized
}

func (devices *DeviceSet) HasActivatedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	if info == nil {
		return false
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	devinfo, _ := getInfo(info.Name())
	return devinfo != nil && devinfo.Exists != 0
}

func (devices *DeviceSet) setInitialized(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	info.Initialized = true
	if err := devices.saveMetadata(); err != nil {
		info.Initialized = false
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	return nil
}

func (devices *DeviceSet) List() []string {
	devices.Lock()
	defer devices.Unlock()

	ids := make([]string, len(devices.Devices))
	i := 0
	for k := range devices.Devices {
		ids[i] = k
		i++
	}
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
	devices.Lock()
	defer devices.Unlock()

	info := devices.Devices[hash]
	if info == nil {
		return nil, fmt.Errorf("No device %s", hash)
	}

	info.lock.Lock()
	defer info.lock.Unlock()

	status := &DevStatus{
		DeviceId:      info.DeviceId,
		Size:          info.Size,
		TransactionId: info.TransactionId,
	}

	if err := devices.activateDeviceIfNeeded(hash); err != nil {
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
