package devmapper

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var (
	DefaultDataLoopbackSize     int64  = 100 * 1024 * 1024 * 1024
	DefaultMetaDataLoopbackSize int64  = 2 * 1024 * 1024 * 1024
	DefaultBaseFsSize           uint64 = 10 * 1024 * 1024 * 1024
)

type DevInfo struct {
	Hash          string       `json:"-"`
	DeviceId      int          `json:"device_id"`
	Size          uint64       `json:"size"`
	TransactionId uint64       `json:"transaction_id"`
	Initialized   bool         `json:"initialized"`
	devices       *DeviceSet `json:"-"`
}

type MetaData struct {
	Devices map[string]*DevInfo `json:devices`
}

type DeviceSet struct {
	MetaData
	sync.Mutex
	initialized      bool
	root             string
	devicePrefix     string
	TransactionId    uint64
	NewTransactionId uint64
	nextFreeDevice   int
	activeMounts     map[string]int
}

type DiskUsage struct {
	Used uint64
	Total uint64
}

type Status struct {
	PoolName string
	DataLoopback string
	MetadataLoopback string
	Data DiskUsage
	Metadata DiskUsage
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
		return fmt.Errorf("Error encoding metaadata to json: %s", err)
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
	if err := os.Rename(tmpFile.Name(), devices.jsonFile()); err != nil {
		return fmt.Errorf("Error committing metadata file", err)
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
	if err != nil && !os.IsNotExist(err) {
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
		if err := devices.removeDevice(""); err != nil {
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
	fileInfos, _ := ioutil.ReadDir("/proc/self/fd")
	if fileInfos != nil {
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

func major (device uint64) uint64 {
	return  (device >> 8) & 0xfff
}

func minor (device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

func (devices *DeviceSet) initDevmapper() error {
	logInit(devices)

	// Make sure the sparse images exist in <root>/devicemapper/data and
	// <root>/devicemapper/metadata

	createdLoopback := !devices.hasImage("data") || !devices.hasImage("metadata")
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

	// If the pool doesn't exist, create it
	if info.Exists == 0 {
		utils.Debugf("Pool doesn't exist. Creating it.")

		dataFile, err := AttachLoopDevice(data)
		if err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
		defer dataFile.Close()

		metadataFile, err := AttachLoopDevice(metadata)
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
	if err := devices.setupBaseImage(); err != nil {
		utils.Debugf("Error device setupBaseImage: %s\n", err)
		return err
	}

	return nil
}

func (devices *DeviceSet) AddDevice(hash, baseHash string) error {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	if devices.Devices[hash] != nil {
		return fmt.Errorf("hash %s already exists", hash)
	}

	baseInfo := devices.Devices[baseHash]
	if baseInfo == nil {
		return fmt.Errorf("Error adding device for '%s': can't find device for parent '%s'", hash, baseHash)
	}

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

func (devices *DeviceSet) removeDevice(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("hash %s doesn't exists", hash)
	}

	devinfo, _ := getInfo(info.Name())
	if devinfo != nil && devinfo.Exists != 0 {
		if err := removeDevice(info.Name()); err != nil {
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

func (devices *DeviceSet) RemoveDevice(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	return devices.removeDevice(hash)
}

func (devices *DeviceSet) deactivateDevice(hash string) error {
	utils.Debugf("[devmapper] deactivateDevice(%s)", hash)
	defer utils.Debugf("[devmapper] deactivateDevice END")
	var devname string
	// FIXME: shouldn't we just register the pool into devices?
	devname, err := devices.byHash(hash)
	if err != nil {
		return err
	}
	devinfo, err := getInfo(devname)
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if devinfo.Exists != 0 {
		if err := removeDevice(devname); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
		if err := devices.waitRemove(hash); err != nil {
			return err
		}
	}

	return nil
}

// waitRemove blocks until either:
// a) the device registered at <device_set_prefix>-<hash> is removed,
// or b) the 1 second timeout expires.
func (devices *DeviceSet) waitRemove(hash string) error {
	utils.Debugf("[deviceset %s] waitRemove(%s)", devices.devicePrefix, hash)
	defer utils.Debugf("[deviceset %s] waitRemove END", devices.devicePrefix, hash)
	devname, err := devices.byHash(hash)
	if err != nil {
		return err
	}
	i := 0
	for ; i < 1000; i += 1 {
		devinfo, err := getInfo(devname)
		if err != nil {
			// If there is an error we assume the device doesn't exist.
			// The error might actually be something else, but we can't differentiate.
			return nil
		}
		utils.Debugf("Waiting for removal of %s: exists=%d", devname, devinfo.Exists)
		if devinfo.Exists == 0 {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	if i == 1000 {
		return fmt.Errorf("Timeout while waiting for device %s to be removed", devname)
	}
	return nil
}

// waitClose blocks until either:
// a) the device registered at <device_set_prefix>-<hash> is closed,
// or b) the 1 second timeout expires.
func (devices *DeviceSet) waitClose(hash string) error {
	devname, err := devices.byHash(hash)
	if err != nil {
		return err
	}
	i := 0
	for ; i < 1000; i += 1 {
		devinfo, err := getInfo(devname)
		if err != nil {
			return err
		}
		utils.Debugf("Waiting for unmount of %s: opencount=%d", devname, devinfo.OpenCount)
		if devinfo.OpenCount == 0 {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	if i == 1000 {
		return fmt.Errorf("Timeout while waiting for device %s to close", devname)
	}
	return nil
}

// byHash is a hack to allow looking up the deviceset's pool by the hash "pool".
// FIXME: it seems probably cleaner to register the pool in devices.Devices,
// but I am afraid of arcane implications deep in the devicemapper code,
// so this will do.
func (devices *DeviceSet) byHash(hash string) (devname string, err error) {
	if hash == "pool" {
		return devices.getPoolDevName(), nil
	}
	info := devices.Devices[hash]
	if info == nil {
		return "", fmt.Errorf("hash %s doesn't exists", hash)
	}
	return info.Name(), nil
}

func (devices *DeviceSet) Shutdown() error {
	utils.Debugf("[deviceset %s] shutdown()", devices.devicePrefix)
	defer utils.Debugf("[deviceset %s] shutdown END", devices.devicePrefix)
	devices.Lock()
	utils.Debugf("[devmapper] Shutting down DeviceSet: %s", devices.root)
	defer devices.Unlock()

	if !devices.initialized {
		return nil
	}

	for path, count := range devices.activeMounts {
		for i := count; i > 0; i-- {
			if err := syscall.Unmount(path, 0); err != nil {
				utils.Debugf("Shutdown unmounting %s, error: %s\n", path, err)
			}
		}
		delete(devices.activeMounts, path)
	}

	for _, d := range devices.Devices {
		if err := devices.waitClose(d.Hash); err != nil {
			utils.Errorf("Warning: error waiting for device %s to unmount: %s\n", d.Hash, err)
		}
		if err := devices.deactivateDevice(d.Hash); err != nil {
			utils.Debugf("Shutdown deactivate %s , error: %s\n", d.Hash, err)
		}
	}

	pool := devices.getPoolDevName()
	if devinfo, err := getInfo(pool); err == nil && devinfo.Exists != 0 {
		if err := devices.deactivateDevice("pool"); err != nil {
			utils.Debugf("Shutdown deactivate %s , error: %s\n", pool, err)
		}
	}

	return nil
}

func (devices *DeviceSet) MountDevice(hash, path string, readOnly bool) error {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	if err := devices.activateDeviceIfNeeded(hash); err != nil {
		return fmt.Errorf("Error activating devmapper device for '%s': %s", hash, err)
	}

	info := devices.Devices[hash]

	var flags uintptr = syscall.MS_MGC_VAL

	if readOnly {
		flags = flags | syscall.MS_RDONLY
	}

	err := syscall.Mount(info.DevName(), path, "ext4", flags, "discard")
	if err != nil && err == syscall.EINVAL {
		err = syscall.Mount(info.DevName(), path, "ext4", flags, "")
	}
	if err != nil {
		return fmt.Errorf("Error mounting '%s' on '%s': %s", info.DevName(), path, err)
	}

	count := devices.activeMounts[path]
	devices.activeMounts[path] = count + 1

	return nil
}

func (devices *DeviceSet) UnmountDevice(hash, path string, deactivate bool) error {
	utils.Debugf("[devmapper] UnmountDevice(hash=%s path=%s)", hash, path)
	defer utils.Debugf("[devmapper] UnmountDevice END")
	devices.Lock()
	defer devices.Unlock()

	utils.Debugf("[devmapper] Unmount(%s)", path)
	if err := syscall.Unmount(path, 0); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	utils.Debugf("[devmapper] Unmount done")
	// Wait for the unmount to be effective,
	// by watching the value of Info.OpenCount for the device
	if err := devices.waitClose(hash); err != nil {
		return err
	}

	if count := devices.activeMounts[path]; count > 1 {
		devices.activeMounts[path] = count - 1
	} else {
		delete(devices.activeMounts, path)
	}

	if deactivate {
		devices.deactivateDevice(hash)
	}

	return nil
}

func (devices *DeviceSet) HasDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	return devices.Devices[hash] != nil
}

func (devices *DeviceSet) HasInitializedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	info := devices.Devices[hash]
	return info != nil && info.Initialized
}

func (devices *DeviceSet) HasActivatedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

	info := devices.Devices[hash]
	if info == nil {
		return false
	}
	devinfo, _ := getInfo(info.Name())
	return devinfo != nil && devinfo.Exists != 0
}

func (devices *DeviceSet) SetInitialized(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	devices.ensureInit()

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

func (devices *DeviceSet) Status() *Status {
	devices.Lock()
	defer devices.Unlock()

	status := &Status {}

	devices.ensureInit()

	status.PoolName = devices.getPoolName()
	status.DataLoopback = path.Join( devices.loopbackDir(), "data")
	status.MetadataLoopback = path.Join( devices.loopbackDir(), "metadata")

	_, totalSizeInSectors, _, params, err := getStatus(devices.getPoolName())
	if err == nil {
		var transactionId, dataUsed, dataTotal, metadataUsed, metadataTotal uint64
		if _, err := fmt.Sscanf(params, "%d %d/%d %d/%d", &transactionId, &metadataUsed, &metadataTotal, &dataUsed, &dataTotal); err == nil {
			// Convert from blocks to bytes
			blockSizeInSectors := totalSizeInSectors / dataTotal;

			status.Data.Used = dataUsed * blockSizeInSectors * 512
			status.Data.Total = dataTotal * blockSizeInSectors * 512

			// metadata blocks are always 4k
			status.Metadata.Used = metadataUsed * 4096
			status.Metadata.Total = metadataTotal * 4096
		}
	}

	return status
}

func (devices *DeviceSet) ensureInit() {
	if !devices.initialized {
		devices.initialized = true
		if err := devices.initDevmapper(); err != nil {
			log.Fatalf("Unable to initialize device mapper support: %v", err)
		}
	}
}

func NewDeviceSet(root string) *DeviceSet {
	SetDevDir("/dev")

	return &DeviceSet{
		initialized:  false,
		root:         root,
		MetaData:     MetaData{Devices: make(map[string]*DevInfo)},
		activeMounts: make(map[string]int),
	}
}
