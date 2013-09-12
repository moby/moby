package devmapper

import (
	"github.com/dotcloud/docker/utils"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const defaultDataLoopbackSize int64 = 100 * 1024 * 1024 * 1024
const defaultMetaDataLoopbackSize int64 = 2 * 1024 * 1024 * 1024
const defaultBaseFsSize uint64 = 10 * 1024 * 1024 * 1024

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
	initialized bool
	root        string
	devicePrefix string
	MetaData
	TransactionId    uint64
	NewTransactionId uint64
	nextFreeDevice   int
	activeMounts map[string]int
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
	return fmt.Sprintf("%s-pool", devices.devicePrefix)
}

func (devices *DeviceSetDM) getPoolDevName() string {
	return getDevName(devices.getPoolName())
}

func (devices *DeviceSetDM) createTask(t TaskType, name string) (*Task, error) {
	task := TaskCreate(t)
	if task == nil {
		return nil, fmt.Errorf("Can't create task of type %d", int(t))
	}
	err := task.SetName(name)
	if err != nil {
		return nil, fmt.Errorf("Can't set task name %s", name)
	}
	return task, nil
}

func (devices *DeviceSetDM) getInfo(name string) (*Info, error) {
	task, err := devices.createTask(DeviceInfo, name)
	if task == nil {
		return nil, err
	}
	err = task.Run()
	if err != nil {
		return nil, err
	}
	info, err := task.GetInfo()
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (devices *DeviceSetDM) getStatus(name string) (uint64, uint64, string, string, error) {
	task, err := devices.createTask(DeviceStatus, name)
	if task == nil {
		return 0, 0, "", "", err
	}
	err = task.Run()
	if err != nil {
		return 0, 0, "", "", err
	}

	devinfo, err := task.GetInfo()
	if err != nil {
		return 0, 0, "", "", err
	}
	if devinfo.Exists == 0 {
		return 0, 0, "", "", fmt.Errorf("Non existing device %s", name)
	}

	var next uintptr = 0
	next, start, length, target_type, params := task.GetNextTarget(next)

	return start, length, target_type, params, nil
}

func (devices *DeviceSetDM) setTransactionId(oldId uint64, newId uint64) error {
	task, err := devices.createTask(DeviceTargetMsg, devices.getPoolDevName())
	if task == nil {
		return err
	}

	err = task.SetSector(0)
	if err != nil {
		return fmt.Errorf("Can't set sector")
	}

	message := fmt.Sprintf("set_transaction_id %d %d", oldId, newId)
	err = task.SetMessage(message)
	if err != nil {
		return fmt.Errorf("Can't set message")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running setTransactionId")
	}
	return nil
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

	_, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		log.Printf("Creating loopback file %s for device-manage use", filename)
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return "", err
		}
		err = file.Truncate(size)
		if err != nil {
			return "", err
		}
	}
	return filename, nil
}

func (devices *DeviceSetDM) createPool(dataFile *os.File, metadataFile *os.File) error {
	utils.Debugf("Activating device-mapper pool %s", devices.getPoolName())
	task, err := devices.createTask(DeviceCreate, devices.getPoolName())
	if task == nil {
		return err
	}

	size, err := GetBlockDeviceSize(dataFile)
	if err != nil {
		return fmt.Errorf("Can't get data size")
	}

	params := metadataFile.Name() + " " + dataFile.Name() + " 512 8192"
	err = task.AddTarget(0, size/512, "thin-pool", params)
	if err != nil {
		return fmt.Errorf("Can't add target")
	}

	var cookie uint32 = 0
	err = task.SetCookie(&cookie, 32)
	if err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running DeviceCreate")
	}

	UdevWait(cookie)

	return nil
}

func (devices *DeviceSetDM) suspendDevice(info *DevInfo) error {
	task, err := devices.createTask(DeviceSuspend, info.Name())
	if task == nil {
		return err
	}
	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running DeviceSuspend")
	}
	return nil
}

func (devices *DeviceSetDM) resumeDevice(info *DevInfo) error {
	task, err := devices.createTask(DeviceResume, info.Name())
	if task == nil {
		return err
	}

	var cookie uint32 = 0
	err = task.SetCookie(&cookie, 32)
	if err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running DeviceSuspend")
	}

	UdevWait(cookie)

	return nil
}

func (devices *DeviceSetDM) createDevice(deviceId int) error {
	task, err := devices.createTask(DeviceTargetMsg, devices.getPoolDevName())
	if task == nil {
		return err
	}

	err = task.SetSector(0)
	if err != nil {
		return fmt.Errorf("Can't set sector")
	}

	message := fmt.Sprintf("create_thin %d", deviceId)
	err = task.SetMessage(message)
	if err != nil {
		return fmt.Errorf("Can't set message")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running createDevice")
	}
	return nil
}

func (devices *DeviceSetDM) createSnapDevice(deviceId int, baseInfo *DevInfo) error {
	doSuspend := false
	devinfo, _ := devices.getInfo(baseInfo.Name())
	if devinfo != nil && devinfo.Exists != 0 {
		doSuspend = true
	}

	if doSuspend {
		err := devices.suspendDevice(baseInfo)
		if err != nil {
			return err
		}
	}

	task, err := devices.createTask(DeviceTargetMsg, devices.getPoolDevName())
	if task == nil {
		_ = devices.resumeDevice(baseInfo)
		return err
	}
	err = task.SetSector(0)
	if err != nil {
		_ = devices.resumeDevice(baseInfo)
		return fmt.Errorf("Can't set sector")
	}

	message := fmt.Sprintf("create_snap %d %d", deviceId, baseInfo.DeviceId)
	err = task.SetMessage(message)
	if err != nil {
		_ = devices.resumeDevice(baseInfo)
		return fmt.Errorf("Can't set message")
	}

	err = task.Run()
	if err != nil {
		_ = devices.resumeDevice(baseInfo)
		return fmt.Errorf("Error running DeviceCreate")
	}

	if doSuspend {
		err = devices.resumeDevice(baseInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func (devices *DeviceSetDM) deleteDevice(deviceId int) error {
	task, err := devices.createTask(DeviceTargetMsg, devices.getPoolDevName())
	if task == nil {
		return err
	}

	err = task.SetSector(0)
	if err != nil {
		return fmt.Errorf("Can't set sector")
	}

	message := fmt.Sprintf("delete %d", deviceId)
	err = task.SetMessage(message)
	if err != nil {
		return fmt.Errorf("Can't set message")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running deleteDevice")
	}
	return nil
}

func (devices *DeviceSetDM) removeDevice(name string) error {
	task, err := devices.createTask(DeviceRemove, name)
	if task == nil {
		return err
	}
	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running removeDevice")
	}
	return nil
}

func (devices *DeviceSetDM) activateDevice(info *DevInfo) error {
	task, err := devices.createTask(DeviceCreate, info.Name())
	if task == nil {
		return err
	}

	params := fmt.Sprintf("%s %d", devices.getPoolDevName(), info.DeviceId)
	err = task.AddTarget(0, info.Size/512, "thin", params)
	if err != nil {
		return fmt.Errorf("Can't add target")
	}

	var cookie uint32 = 0
	err = task.SetCookie(&cookie, 32)
	if err != nil {
		return fmt.Errorf("Can't set cookie")
	}

	err = task.Run()
	if err != nil {
		return fmt.Errorf("Error running DeviceCreate")
	}

	UdevWait(cookie)

	return nil
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
		return err
	}
	tmpFile, err := ioutil.TempFile(filepath.Dir(devices.jsonFile()), ".json")
	if err != nil {
		return err
	}

	n, err := tmpFile.Write(jsonData)
	if err != nil {
		return err
	}
	if n < len(jsonData) {
		err = io.ErrShortWrite
	}
	err = tmpFile.Sync()
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}
	err = os.Rename(tmpFile.Name(), devices.jsonFile())
	if err != nil {
		return err
	}

	if devices.NewTransactionId != devices.TransactionId {
		err = devices.setTransactionId(devices.TransactionId, devices.NewTransactionId)
		if err != nil {
			return err
		}
		devices.TransactionId = devices.NewTransactionId
	}

	return nil
}

func (devices *DeviceSetDM) registerDevice(id int, hash string, size uint64) (*DevInfo, error) {
	transaction := devices.allocateTransactionId()

	info := &DevInfo{
		Hash:          hash,
		DeviceId:      id,
		Size:          size,
		TransactionId: transaction,
		Initialized:   false,
		devices:       devices,
	}

	devices.Devices[hash] = info
	err := devices.saveMetadata()
	if err != nil {
		// Try to remove unused device
		devices.Devices[hash] = nil
		return nil, err
	}

	return info, nil
}

func (devices *DeviceSetDM) activateDeviceIfNeeded(hash string) error {
	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	name := info.Name()
	devinfo, _ := devices.getInfo(name)
	if devinfo != nil && devinfo.Exists != 0 {
		return nil
	}

	return devices.activateDevice(info)
}

func (devices *DeviceSetDM) createFilesystem(info *DevInfo) error {
	devname := info.DevName()

	err := exec.Command("mkfs.ext4", "-E",
		"discard,lazy_itable_init=0,lazy_journal_init=0", devname).Run()
	if err != nil {
		err = exec.Command("mkfs.ext4", "-E",
		"discard,lazy_itable_init=0", devname).Run()
	}
	if err != nil {
		return err
	}
	return nil
}

func (devices *DeviceSetDM) loadMetaData() error {
	_, _, _, params, err := devices.getStatus(devices.getPoolName())
	if err != nil {
		return err
	}
	var currentTransaction uint64
	_, err = fmt.Sscanf(params, "%d", &currentTransaction)
	if err != nil {
		return err
	}

	devices.TransactionId = currentTransaction
	devices.NewTransactionId = devices.TransactionId

	jsonData, err := ioutil.ReadFile(devices.jsonFile())
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	metadata := &MetaData{
		Devices: make(map[string]*DevInfo),
	}
	if jsonData != nil {
		if err := json.Unmarshal(jsonData, metadata); err != nil {
			return err
		}
	}
	devices.MetaData = *metadata

	for hash, d := range devices.Devices {
		d.Hash = hash
		d.devices = devices

		if d.DeviceId >= devices.nextFreeDevice {
			devices.nextFreeDevice = d.DeviceId + 1
		}

		// If the transaction id is larger than the actual one we lost the device due to some crash
		if d.TransactionId > currentTransaction {
			log.Printf("Removing lost device %s with id %d", hash, d.TransactionId)
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
		log.Printf("Removing uninitialized base image")
		if err := devices.RemoveDevice(""); err != nil {
			return err
		}
	}

	log.Printf("Initializing base device-manager snapshot")

	id := devices.allocateDeviceId()

	// Create initial device
	err := devices.createDevice(id)
	if err != nil {
		return err
	}

	info, err := devices.registerDevice(id, "", defaultBaseFsSize)
	if err != nil {
		_ = devices.deleteDevice(id)
		return err
	}

	log.Printf("Creating filesystem on base device-manager snapshot")

	err = devices.activateDeviceIfNeeded("")
	if err != nil {
		return err
	}

	err = devices.createFilesystem(info)
	if err != nil {
		return err
	}

	info.Initialized = true

	err = devices.saveMetadata()
	if err != nil {
		info.Initialized = false
		return err
	}

	return nil
}

func (devices *DeviceSetDM) initDevmapper() error {
	info, err := devices.getInfo(devices.getPoolName())
	if info == nil {
		return err
	}

	if info.Exists != 0 {
		/* Pool exists, assume everything is up */
		err = devices.loadMetaData()
		if err != nil {
			return err
		}
		err = devices.setupBaseImage()
		if err != nil {
			return err
		}
		return nil
	}

	createdLoopback := false
	if !devices.hasImage("data") || !devices.hasImage("metadata") {
		/* If we create the loopback mounts we also need to initialize the base fs */
		createdLoopback = true
	}

	data, err := devices.ensureImage("data", defaultDataLoopbackSize)
	if err != nil {
		return err
	}

	metadata, err := devices.ensureImage("metadata", defaultMetaDataLoopbackSize)
	if err != nil {
		return err
	}

	dataFile, err := AttachLoopDevice(data)
	if err != nil {
		return err
	}
	defer dataFile.Close()

	metadataFile, err := AttachLoopDevice(metadata)
	if err != nil {
		return err
	}
	defer metadataFile.Close()

	err = devices.createPool(dataFile, metadataFile)
	if err != nil {
		return err
	}

	if !createdLoopback {
		err = devices.loadMetaData()
		if err != nil {
			return err
		}
	}

	err = devices.setupBaseImage()
	if err != nil {
		return err
	}

	return nil
}

func (devices *DeviceSetDM) AddDevice(hash, baseHash string) error {
	if err := devices.ensureInit(); err != nil {
		return err
	}

	if devices.Devices[hash] != nil {
		return fmt.Errorf("hash %s already exists", hash)
	}

	baseInfo := devices.Devices[baseHash]
	if baseInfo == nil {
		return fmt.Errorf("Unknown base hash %s", baseHash)
	}

	deviceId := devices.allocateDeviceId()

	err := devices.createSnapDevice(deviceId, baseInfo)
	if err != nil {
		return err
	}

	_, err = devices.registerDevice(deviceId, hash, baseInfo.Size)
	if err != nil {
		_ = devices.deleteDevice(deviceId)
		return err
	}
	return nil
}

func (devices *DeviceSetDM) RemoveDevice(hash string) error {
	if err := devices.ensureInit(); err != nil {
		return err
	}

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("hash %s doesn't exists", hash)
	}

	devinfo, _ := devices.getInfo(info.Name())
	if devinfo != nil && devinfo.Exists != 0 {
		err := devices.removeDevice(info.Name())
		if err != nil {
			return err
		}
	}

	if info.Initialized {
		info.Initialized = false
		err := devices.saveMetadata()
		if err != nil {
			return err
		}
	}

	err := devices.deleteDevice(info.DeviceId)
	if err != nil {
		return err
	}

	_ = devices.allocateTransactionId()
	delete(devices.Devices, info.Hash)

	err = devices.saveMetadata()
	if err != nil {
		devices.Devices[info.Hash] = info
		return err
	}

	return nil
}

func (devices *DeviceSetDM) DeactivateDevice(hash string) error {
	if err := devices.ensureInit(); err != nil {
		return err
	}

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("hash %s doesn't exists", hash)
	}

	devinfo, err := devices.getInfo(info.Name())
	if err != nil {
		return err
	}
	if devinfo.Exists != 0 {
		err := devices.removeDevice(info.Name())
		if err != nil {
			return err
		}
	}

	return nil
}

func (devices *DeviceSetDM) Shutdown() error {
	if !devices.initialized {
		return nil
	}

	for path, count := range devices.activeMounts {
		for i := count; i > 0; i-- {
			err := syscall.Unmount(path, 0)
			if err != nil {
				fmt.Printf("Shutdown unmounting %s, error: %s\n", path, err)
			}
		}
		delete(devices.activeMounts, path)
	}

	for _, d := range devices.Devices {
		if err := devices.DeactivateDevice(d.Hash); err != nil {
			fmt.Printf("Shutdown deactivate %s , error: %s\n", d.Hash, err)
		}
	}


	pool := devices.getPoolDevName()
	devinfo, err := devices.getInfo(pool)
	if err == nil && devinfo.Exists != 0 {
		if err := devices.removeDevice(pool); err != nil {
			fmt.Printf("Shutdown deactivate %s , error: %s\n", pool, err)
		}
	}

	return nil
}

func (devices *DeviceSetDM) MountDevice(hash, path string) error {
	if err := devices.ensureInit(); err != nil {
		return err
	}

	err := devices.activateDeviceIfNeeded(hash)
	if err != nil {
		return err
	}

	info := devices.Devices[hash]

	err = syscall.Mount(info.DevName(), path, "ext4", syscall.MS_MGC_VAL, "discard")
	if err != nil && err == syscall.EINVAL {
		err = syscall.Mount(info.DevName(), path, "ext4", syscall.MS_MGC_VAL, "")
	}
	if err != nil {
		return err
	}

	count := devices.activeMounts[path]
	devices.activeMounts[path] = count + 1

	return nil
}

func (devices *DeviceSetDM) UnmountDevice(hash, path string) error {
	err := syscall.Unmount(path, 0)
	if err != nil {
		return err
	}

	count := devices.activeMounts[path]
	if count > 1 {
		devices.activeMounts[path] = count - 1
	} else {
		delete(devices.activeMounts, path)
	}

	return nil
}


func (devices *DeviceSetDM) HasDevice(hash string) bool {
	if err := devices.ensureInit(); err != nil {
		return false
	}

	info := devices.Devices[hash]
	return info != nil
}

func (devices *DeviceSetDM) HasInitializedDevice(hash string) bool {
	if err := devices.ensureInit(); err != nil {
		return false
	}

	info := devices.Devices[hash]
	return info != nil && info.Initialized
}

func (devices *DeviceSetDM) SetInitialized(hash string) error {
	if err := devices.ensureInit(); err != nil {
		return err
	}

	info := devices.Devices[hash]
	if info == nil {
		return fmt.Errorf("Unknown device %s", hash)
	}

	info.Initialized = true
	err := devices.saveMetadata()
	if err != nil {
		info.Initialized = false
		return err
	}

	return nil
}

func (devices *DeviceSetDM) ensureInit() error {
	if !devices.initialized {
		devices.initialized = true
		err := devices.initDevmapper()
		if err != nil {
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

	devices := &DeviceSetDM{
		initialized: false,
		root:        root,
		devicePrefix: base,
	}
	devices.Devices = make(map[string]*DevInfo)
	devices.activeMounts = make(map[string]int)

	return devices
}
