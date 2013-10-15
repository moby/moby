package devmapper

import (
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	defaultDataLoopbackSize     int64  = 100 * 1024 * 1024 * 1024
	defaultMetaDataLoopbackSize int64  = 2 * 1024 * 1024 * 1024
	defaultBaseFsSize           uint64 = 10 * 1024 * 1024 * 1024
)

type DevInfo struct {
	Hash          string       `json:"-"`
	DeviceId      int          `json:"device_id"`
	Size          uint64       `json:"size"`
	TransactionId uint64       `json:"transaction_id"`
	Initialized   bool         `json:"initialized"`
	devices       *DeviceSetDM `json:"-"`
}

type MetaData struct {
	Devices map[string]*DevInfo `json:devices`
}

type DeviceSetDM struct {
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

func init() {
	var err error

	rawMetaSize := os.Getenv("DOCKER_LOOPBACK_META_SIZE")
	rawDataSize := os.Getenv("DOCKER_LOOPBACK_DATA_SIZE")
	rawBaseFSSize := os.Getenv("DOCKER_BASE_FS_SIZE")

	if rawMetaSize != "" {
		if defaultMetaDataLoopbackSize, err = strconv.ParseInt(rawMetaSize, 0, 0); err != nil {
			panic(err)
		}
	}
	if rawDataSize != "" {
		if defaultDataLoopbackSize, err = strconv.ParseInt(rawDataSize, 0, 0); err != nil {
			panic(err)
		}
	}
	if rawBaseFSSize != "" {
		if defaultBaseFsSize, err = strconv.ParseUint(rawBaseFSSize, 0, 0); err != nil {
			panic(err)
		}
	}
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

func (devices *DeviceSetDM) loopbackDir() string {
	return path.Join(devices.root, "loopback")
}

func (devices *DeviceSetDM) jsonFile() string {
	return path.Join(devices.loopbackDir(), "json")
}

func (devices *DeviceSetDM) getPoolName() string {
	return devices.devicePrefix + "-pool"
}

func (devices *DeviceSetDM) getPoolDevName() string {
	return getDevName(devices.getPoolName())
}

func (devices *DeviceSetDM) hasImage(name string) bool {
	dirname := devices.loopbackDir()
	filename := path.Join(dirname, name)

	_, err := os.Stat(filename)
	return err == nil
}

func (devices *DeviceSetDM) ensureImage(name string, size int64) (string, error) {
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

func (devices *DeviceSetDM) allocateDeviceId() int {
	// TODO: Add smarter reuse of deleted devices
	id := devices.nextFreeDevice
	devices.nextFreeDevice = devices.nextFreeDevice + 1
	return id
}

func (devices *DeviceSetDM) allocateTransactionId() uint64 {
	devices.NewTransactionId = devices.NewTransactionId + 1
	return devices.NewTransactionId
}

func (devices *DeviceSetDM) saveMetadata() error {
	jsonData, err := json.Marshal(devices.MetaData)
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	tmpFile, err := ioutil.TempFile(filepath.Dir(devices.jsonFile()), ".json")
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	n, err := tmpFile.Write(jsonData)
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if n < len(jsonData) {
		return io.ErrShortWrite
	}
	if err := tmpFile.Sync(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if err := os.Rename(tmpFile.Name(), devices.jsonFile()); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if devices.NewTransactionId != devices.TransactionId {
		if err = setTransactionId(devices.getPoolDevName(), devices.TransactionId, devices.NewTransactionId); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
		devices.TransactionId = devices.NewTransactionId
	}
	return nil
}

func (devices *DeviceSetDM) registerDevice(id int, hash string, size uint64) (*DevInfo, error) {
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

func (devices *DeviceSetDM) activateDeviceIfNeeded(hash string) error {
	utils.Debugf("activateDeviceIfNeeded()")
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	if devinfo, _ := getInfo(info.Name()); devinfo != nil && devinfo.Exists != 0 {
		return nil
	}

	return activateDevice(devices.getPoolDevName(), info.Name(), info.DeviceId, info.Size)
}

func (devices *DeviceSetDM) createFilesystem(info *DevInfo) error {
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

func (devices *DeviceSetDM) loadMetaData() error {
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

func (devices *DeviceSetDM) setupBaseImage() error {
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

	info, err := devices.registerDevice(id, "", defaultBaseFsSize)
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

func (devices *DeviceSetDM) log(level int, file string, line int, dmError int, message string) {
	if level >= 7 {
		return // Ignore _LOG_DEBUG
	}

	utils.Debugf("libdevmapper(%d): %s:%d (%d) %s", level, file, line, dmError, message)
}

func (devices *DeviceSetDM) initDevmapper() error {
	logInit(devices)

begin:
	info, err := getInfo(devices.getPoolName())
	if info == nil {
		utils.Debugf("Error device getInfo: %s", err)
		return err
	}

	loopbackExists := false
	if _, err := os.Stat(devices.loopbackDir()); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// If it does not, then we use a different pool name
		parts := strings.Split(devices.devicePrefix, "-")
		i, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			i = 0
			parts = append(parts, "0")
		}
		i++
		parts[len(parts)-1] = strconv.Itoa(i)
		devices.devicePrefix = strings.Join(parts, "-")
	} else {
		loopbackExists = true
	}

	// If the pool exists but the loopback does not, then we start again
	if info.Exists == 1 && !loopbackExists {
		goto begin
	}

	utils.Debugf("initDevmapper(). Pool exists: %v, loopback Exists: %v", info.Exists, loopbackExists)

	// It seems libdevmapper opens this without O_CLOEXEC, and go exec will not close files
	// that are not Close-on-exec, and lxc-start will die if it inherits any unexpected files,
	// so we add this badhack to make sure it closes itself
	setCloseOnExec("/dev/mapper/control")

	if info.Exists != 0 && loopbackExists {
		/* Pool exists, assume everything is up */
		if err := devices.loadMetaData(); err != nil {
			utils.Debugf("Error device loadMetaData: %s\n", err)
			return err
		}
		if err := devices.setupBaseImage(); err != nil {
			utils.Debugf("Error device setupBaseImage: %s\n", err)
			return err
		}
		return nil
	}

	/* If we create the loopback mounts we also need to initialize the base fs */
	createdLoopback := !devices.hasImage("data") || !devices.hasImage("metadata")

	data, err := devices.ensureImage("data", defaultDataLoopbackSize)
	if err != nil {
		utils.Debugf("Error device ensureImage (data): %s\n", err)
		return err
	}

	metadata, err := devices.ensureImage("metadata", defaultMetaDataLoopbackSize)
	if err != nil {
		utils.Debugf("Error device ensureImage (metadata): %s\n", err)
		return err
	}

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

	if !createdLoopback {
		if err = devices.loadMetaData(); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	if err := devices.setupBaseImage(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	return nil
}

func (devices *DeviceSetDM) AddDevice(hash, baseHash string) error {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		utils.Debugf("Error init: %s\n", err)
		return err
	}

	if devices.Devices[hash] != nil {
		return fmt.Errorf("hash %s already exists", hash)
	}

	baseInfo := devices.Devices[baseHash]
	if baseInfo == nil {
		utils.Debugf("Base Hash not found")
		return fmt.Errorf("Unknown base hash %s", baseHash)
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

func (devices *DeviceSetDM) removeDevice(hash string) error {
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

func (devices *DeviceSetDM) RemoveDevice(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	return devices.removeDevice(hash)
}

func (devices *DeviceSetDM) deactivateDevice(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("hash %s doesn't exists", hash)
	}

	devinfo, err := getInfo(info.Name())
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}
	if devinfo.Exists != 0 {
		if err := removeDevice(info.Name()); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}

	return nil
}

func (devices *DeviceSetDM) DeactivateDevice(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	utils.Debugf("DeactivateDevice %s", hash)
	return devices.deactivateDevice(hash)
}

func (devices *DeviceSetDM) Shutdown() error {
	devices.Lock()
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
		if err := devices.deactivateDevice(d.Hash); err != nil {
			utils.Debugf("Shutdown deactivate %s , error: %s\n", d.Hash, err)
		}
	}

	pool := devices.getPoolDevName()
	if devinfo, err := getInfo(pool); err == nil && devinfo.Exists != 0 {
		if err := removeDevice(pool); err != nil {
			utils.Debugf("Shutdown deactivate %s , error: %s\n", pool, err)
		}
	}

	return nil
}

func (devices *DeviceSetDM) MountDevice(hash, path string) error {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	if err := devices.activateDeviceIfNeeded(hash); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	info := devices.Devices[hash]

	err := syscall.Mount(info.DevName(), path, "ext4", syscall.MS_MGC_VAL, "discard")
	if err != nil && err == syscall.EINVAL {
		err = syscall.Mount(info.DevName(), path, "ext4", syscall.MS_MGC_VAL, "")
	}
	if err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

	count := devices.activeMounts[path]
	devices.activeMounts[path] = count + 1

	return nil
}

func (devices *DeviceSetDM) UnmountDevice(hash, path string, deactivate bool) error {
	devices.Lock()
	defer devices.Unlock()

	if err := syscall.Unmount(path, 0); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
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

func (devices *DeviceSetDM) HasDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		return false
	}
	return devices.Devices[hash] != nil
}

func (devices *DeviceSetDM) HasInitializedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		return false
	}

	info := devices.Devices[hash]
	return info != nil && info.Initialized
}

func (devices *DeviceSetDM) HasActivatedDevice(hash string) bool {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		return false
	}

	info := devices.Devices[hash]
	if info == nil {
		return false
	}
	devinfo, _ := getInfo(info.Name())
	return devinfo != nil && devinfo.Exists != 0
}

func (devices *DeviceSetDM) SetInitialized(hash string) error {
	devices.Lock()
	defer devices.Unlock()

	if err := devices.ensureInit(); err != nil {
		utils.Debugf("\n--->Err: %s\n", err)
		return err
	}

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

func (devices *DeviceSetDM) ensureInit() error {
	if !devices.initialized {
		devices.initialized = true
		if err := devices.initDevmapper(); err != nil {
			utils.Debugf("\n--->Err: %s\n", err)
			return err
		}
	}
	return nil
}

func NewDeviceSetDM(root string) *DeviceSetDM {
	SetDevDir("/dev")

	base := filepath.Base(root)
	if !strings.HasPrefix(base, "docker") {
		base = "docker-" + base
	}

	return &DeviceSetDM{
		initialized:  false,
		root:         root,
		devicePrefix: base,
		MetaData:     MetaData{Devices: make(map[string]*DevInfo)},
		activeMounts: make(map[string]int),
	}
}
