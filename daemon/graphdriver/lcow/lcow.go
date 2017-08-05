// +build windows

// Maintainer:  jhowardmsft
// Locale:      en-gb
// About:       Graph-driver for Linux Containers On Windows (LCOW)
//
// This graphdriver runs in two modes. Yet to be determined which one will
// be the shipping mode. The global mode is where a single utility VM
// is used for all service VM tool operations. This isn't safe security-wise
// as it's attaching a sandbox of multiple containers to it, containing
// untrusted data. This may be fine for client devops scenarios. In
// safe mode, a unique utility VM is instantiated for all service VM tool
// operations. The downside of safe-mode is that operations are slower as
// a new service utility VM has to be started and torn-down when needed.
//
// Options (needs official documentation, but lets get full functionality first...) @jhowardmsft
//
// The following options are read by the graphdriver itself:
//
//   * lcow.globalmode - Enables global service VM Mode
//        -- Possible values:     true/false
//        -- Default if omitted:  false
//
//   * lcow.sandboxsize - Specifies a custom sandbox size in GB for starting a container
//        -- Possible values:      >= default sandbox size (opengcs defined, currently 20)
//        -- Default if ommitted:  20
//
// The following options are read by opengcs:
//
//   * lcow.kirdpath - Specifies a custom path to a kernel/initrd pair
//        -- Possible values:      Any local path that is not a mapped drive
//        -- Default if ommitted:  %ProgramFiles%\Linux Containers
//
//   * lcow.kernel - Specifies a custom kernel file located in the `lcow.kirdpath` path
//        -- Possible values:      Any valid filename
//        -- Default if ommitted:  bootx64.efi
//
//   * lcow.initrd - Specifies a custom initrd file located in the `lcow.kirdpath` path
//        -- Possible values:      Any valid filename
//        -- Default if ommitted:  initrd.img
//
//   * lcow.bootparameters - Specifies additional boot parameters for booting in kernel+initrd mode
//        -- Possible values:      Any valid linux kernel boot options
//        -- Default if ommitted:  <nil>
//
//   * lcow.vhdx - Specifies a custom vhdx file to boot (instead of a kernel+initrd)
//        -- Possible values:      Any valid filename
//        -- Default if ommitted:  uvm.vhdx under `lcow.kirdpath`
//
//   * lcow.timeout - Specifies a timeout for utility VM operations in seconds
//        -- Possible values:      >=0
//        -- Default if ommitted:  300

// TODO: Grab logs from SVM at terminate or errors

package lcow

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/system"
	"github.com/jhowardmsft/opengcs/gogcs/client"
	"github.com/sirupsen/logrus"
)

// init registers this driver to the register. It gets initialised by the
// function passed in the second parameter, implemented in this file.
func init() {
	graphdriver.Register("lcow", InitDriver)
}

const (
	// sandboxFilename is the name of the file containing a layer's sandbox (read-write layer).
	sandboxFilename = "sandbox.vhdx"

	// scratchFilename is the name of the scratch-space used by an SVM to avoid running out of memory.
	scratchFilename = "scratch.vhdx"

	// layerFilename is the name of the file containing a layer's read-only contents.
	// Note this really is VHD format, not VHDX.
	layerFilename = "layer.vhd"

	// toolsScratchPath is a location in a service utility VM that the tools can use as a
	// scratch space to avoid running out of memory.
	toolsScratchPath = "/tmp/scratch"

	// svmGlobalID is the ID used in the serviceVMs map for the global service VM when running in "global" mode.
	svmGlobalID = "_lcow_global_svm_"

	// cacheDirectory is the sub-folder under the driver's data-root used to cache blank sandbox and scratch VHDs.
	cacheDirectory = "cache"

	// scratchDirectory is the sub-folder under the driver's data-root used for scratch VHDs in service VMs
	scratchDirectory = "scratch"
)

// cacheItem is our internal structure representing an item in our local cache
// of things that have been mounted.
type cacheItem struct {
	sync.Mutex        // Protects operations performed on this item
	uvmPath    string // Path in utility VM
	hostPath   string // Path on host
	refCount   int    // How many times its been mounted
	isSandbox  bool   // True if a sandbox
	isMounted  bool   // True when mounted in a service VM
}

// setIsMounted is a helper function for a cacheItem which does exactly what it says
func (ci *cacheItem) setIsMounted() {
	logrus.Debugf("locking cache item for set isMounted")
	ci.Lock()
	defer ci.Unlock()
	ci.isMounted = true
	logrus.Debugf("set isMounted on cache item")
}

// incrementRefCount is a helper function for a cacheItem which does exactly what it says
func (ci *cacheItem) incrementRefCount() {
	logrus.Debugf("locking cache item for increment")
	ci.Lock()
	defer ci.Unlock()
	ci.refCount++
	logrus.Debugf("incremented refcount on cache item %+v", ci)
}

// decrementRefCount is a helper function for a cacheItem which does exactly what it says
func (ci *cacheItem) decrementRefCount() int {
	logrus.Debugf("locking cache item for decrement")
	ci.Lock()
	defer ci.Unlock()
	ci.refCount--
	logrus.Debugf("decremented refcount on cache item %+v", ci)
	return ci.refCount
}

// serviceVMItem is our internal structure representing an item in our
// map of service VMs we are maintaining.
type serviceVMItem struct {
	sync.Mutex                     // Serialises operations being performed in this service VM.
	scratchAttached bool           // Has a scratch been attached?
	config          *client.Config // Represents the service VM item.
}

// Driver represents an LCOW graph driver.
type Driver struct {
	dataRoot           string                    // Root path on the host where we are storing everything.
	cachedSandboxFile  string                    // Location of the local default-sized cached sandbox.
	cachedSandboxMutex sync.Mutex                // Protects race conditions from multiple threads creating the cached sandbox.
	cachedScratchFile  string                    // Location of the local cached empty scratch space.
	cachedScratchMutex sync.Mutex                // Protects race conditions from multiple threads creating the cached scratch.
	options            []string                  // Graphdriver options we are initialised with.
	serviceVmsMutex    sync.Mutex                // Protects add/updates/delete to the serviceVMs map.
	serviceVms         map[string]*serviceVMItem // Map of the configs representing the service VM(s) we are running.
	globalMode         bool                      // Indicates if running in an unsafe/global service VM mode.

	// NOTE: It is OK to use a cache here because Windows does not support
	// restoring containers when the daemon dies.

	cacheMutex sync.Mutex            // Protects add/update/deletes to cache.
	cache      map[string]*cacheItem // Map holding a cache of all the IDs we've mounted/unmounted.
}

// layerDetails is the structure returned by a helper function `getLayerDetails`
// for getting information about a layer folder
type layerDetails struct {
	filename  string // \path\to\sandbox.vhdx or \path\to\layer.vhd
	size      int64  // size of the above file
	isSandbox bool   // true if sandbox.vhdx
}

// deletefiles is a helper function for initialisation where we delete any
// left-over scratch files in case we were previously forcibly terminated.
func deletefiles(path string, f os.FileInfo, err error) error {
	if strings.HasSuffix(f.Name(), ".vhdx") {
		logrus.Warnf("lcowdriver: init: deleting stale scratch file %s", path)
		return os.Remove(path)
	}
	return nil
}

// InitDriver returns a new LCOW storage driver.
func InitDriver(dataRoot string, options []string, _, _ []idtools.IDMap) (graphdriver.Driver, error) {
	title := "lcowdriver: init:"

	cd := filepath.Join(dataRoot, cacheDirectory)
	sd := filepath.Join(dataRoot, scratchDirectory)

	d := &Driver{
		dataRoot:          dataRoot,
		options:           options,
		cachedSandboxFile: filepath.Join(cd, sandboxFilename),
		cachedScratchFile: filepath.Join(cd, scratchFilename),
		cache:             make(map[string]*cacheItem),
		serviceVms:        make(map[string]*serviceVMItem),
		globalMode:        false,
	}

	// Looks for relevant options
	for _, v := range options {
		opt := strings.SplitN(v, "=", 2)
		if len(opt) == 2 {
			switch strings.ToLower(opt[0]) {
			case "lcow.globalmode":
				var err error
				d.globalMode, err = strconv.ParseBool(opt[1])
				if err != nil {
					return nil, fmt.Errorf("%s failed to parse value for 'lcow.globalmode' - must be 'true' or 'false'", title)
				}
				break
			}
		}
	}

	// Make sure the dataRoot directory is created
	if err := idtools.MkdirAllAs(dataRoot, 0700, 0, 0); err != nil {
		return nil, fmt.Errorf("%s failed to create '%s': %v", title, dataRoot, err)
	}

	// Make sure the cache directory is created under dataRoot
	if err := idtools.MkdirAllAs(cd, 0700, 0, 0); err != nil {
		return nil, fmt.Errorf("%s failed to create '%s': %v", title, cd, err)
	}

	// Make sure the scratch directory is created under dataRoot
	if err := idtools.MkdirAllAs(sd, 0700, 0, 0); err != nil {
		return nil, fmt.Errorf("%s failed to create '%s': %v", title, sd, err)
	}

	// Delete any items in the scratch directory
	filepath.Walk(sd, deletefiles)

	logrus.Infof("%s dataRoot: %s globalMode: %t", title, dataRoot, d.globalMode)

	return d, nil
}

// startServiceVMIfNotRunning starts a service utility VM if it is not currently running.
// It can optionally be started with a mapped virtual disk. Returns a opengcs config structure
// representing the VM.
func (d *Driver) startServiceVMIfNotRunning(id string, mvdToAdd *hcsshim.MappedVirtualDisk, context string) (*serviceVMItem, error) {
	// Use the global ID if in global mode
	if d.globalMode {
		id = svmGlobalID
	}

	title := fmt.Sprintf("lcowdriver: startservicevmifnotrunning %s:", id)

	// Make sure thread-safe when interrogating the map
	logrus.Debugf("%s taking serviceVmsMutex", title)
	d.serviceVmsMutex.Lock()

	// Nothing to do if it's already running except add the mapped drive if supplied.
	if svm, ok := d.serviceVms[id]; ok {
		logrus.Debugf("%s exists, releasing serviceVmsMutex", title)
		d.serviceVmsMutex.Unlock()

		if mvdToAdd != nil {
			logrus.Debugf("hot-adding %s to %s", mvdToAdd.HostPath, mvdToAdd.ContainerPath)

			// Ensure the item is locked while doing this
			logrus.Debugf("%s locking serviceVmItem %s", title, svm.config.Name)
			svm.Lock()

			if err := svm.config.HotAddVhd(mvdToAdd.HostPath, mvdToAdd.ContainerPath, false, true); err != nil {
				logrus.Debugf("%s releasing serviceVmItem %s on hot-add failure %s", title, svm.config.Name, err)
				svm.Unlock()
				return nil, fmt.Errorf("%s hot add %s to %s failed: %s", title, mvdToAdd.HostPath, mvdToAdd.ContainerPath, err)
			}

			logrus.Debugf("%s releasing serviceVmItem %s", title, svm.config.Name)
			svm.Unlock()
		}
		return svm, nil
	}

	// Release the lock early
	logrus.Debugf("%s releasing serviceVmsMutex", title)
	d.serviceVmsMutex.Unlock()

	// So we are starting one. First need an enpty structure.
	svm := &serviceVMItem{
		config: &client.Config{},
	}

	// Generate a default configuration
	if err := svm.config.GenerateDefault(d.options); err != nil {
		return nil, fmt.Errorf("%s failed to generate default gogcs configuration for global svm (%s): %s", title, context, err)
	}

	// For the name, we deliberately suffix if safe-mode to ensure that it doesn't
	// clash with another utility VM which may be running for the container itself.
	// This also makes it easier to correlate through Get-ComputeProcess.
	if id == svmGlobalID {
		svm.config.Name = svmGlobalID
	} else {
		svm.config.Name = fmt.Sprintf("%s_svm", id)
	}

	// Ensure we take the cached scratch mutex around the check to ensure the file is complete
	// and not in the process of being created by another thread.
	scratchTargetFile := filepath.Join(d.dataRoot, scratchDirectory, fmt.Sprintf("%s.vhdx", id))

	logrus.Debugf("%s locking cachedScratchMutex", title)
	d.cachedScratchMutex.Lock()
	if _, err := os.Stat(d.cachedScratchFile); err == nil {
		// Make a copy of cached scratch to the scratch directory
		logrus.Debugf("lcowdriver: startServiceVmIfNotRunning: (%s) cloning cached scratch for mvd", context)
		if err := client.CopyFile(d.cachedScratchFile, scratchTargetFile, true); err != nil {
			logrus.Debugf("%s releasing cachedScratchMutex on err: %s", title, err)
			d.cachedScratchMutex.Unlock()
			return nil, err
		}

		// Add the cached clone as a mapped virtual disk
		logrus.Debugf("lcowdriver: startServiceVmIfNotRunning: (%s) adding cloned scratch as mvd", context)
		mvd := hcsshim.MappedVirtualDisk{
			HostPath:          scratchTargetFile,
			ContainerPath:     toolsScratchPath,
			CreateInUtilityVM: true,
		}
		svm.config.MappedVirtualDisks = append(svm.config.MappedVirtualDisks, mvd)
		svm.scratchAttached = true
	}
	logrus.Debugf("%s releasing cachedScratchMutex", title)
	d.cachedScratchMutex.Unlock()

	// If requested to start it with a mapped virtual disk, add it now.
	if mvdToAdd != nil {
		svm.config.MappedVirtualDisks = append(svm.config.MappedVirtualDisks, *mvdToAdd)
	}

	// Start it.
	logrus.Debugf("lcowdriver: startServiceVmIfNotRunning: (%s) starting %s", context, svm.config.Name)
	if err := svm.config.StartUtilityVM(); err != nil {
		return nil, fmt.Errorf("failed to start service utility VM (%s): %s", context, err)
	}

	// As it's now running, add it to the map, checking for a race where another
	// thread has simultaneously tried to start it.
	logrus.Debugf("%s locking serviceVmsMutex for insertion", title)
	d.serviceVmsMutex.Lock()
	if svm, ok := d.serviceVms[id]; ok {
		logrus.Debugf("%s releasing serviceVmsMutex after insertion but exists", title)
		d.serviceVmsMutex.Unlock()
		return svm, nil
	}
	d.serviceVms[id] = svm
	logrus.Debugf("%s releasing serviceVmsMutex after insertion", title)
	d.serviceVmsMutex.Unlock()

	// Now we have a running service VM, we can create the cached scratch file if it doesn't exist.
	logrus.Debugf("%s locking cachedScratchMutex", title)
	d.cachedScratchMutex.Lock()
	if _, err := os.Stat(d.cachedScratchFile); err != nil {
		logrus.Debugf("%s (%s): creating an SVM scratch - locking serviceVM", title, context)
		svm.Lock()
		if err := svm.config.CreateExt4Vhdx(scratchTargetFile, client.DefaultVhdxSizeGB, d.cachedScratchFile); err != nil {
			logrus.Debugf("%s (%s): releasing serviceVM on error path from CreateExt4Vhdx: %s", title, context, err)
			svm.Unlock()
			logrus.Debugf("%s (%s): releasing cachedScratchMutex on error path", title, context)
			d.cachedScratchMutex.Unlock()

			// Do a force terminate and remove it from the map on failure, ignoring any errors
			if err2 := d.terminateServiceVM(id, "error path from CreateExt4Vhdx", true); err2 != nil {
				logrus.Warnf("failed to terminate service VM on error path from CreateExt4Vhdx: %s", err2)
			}

			return nil, fmt.Errorf("failed to create SVM scratch VHDX (%s): %s", context, err)
		}
		logrus.Debugf("%s (%s): releasing serviceVM after %s created and cached to %s", title, context, scratchTargetFile, d.cachedScratchFile)
		svm.Unlock()
	}
	logrus.Debugf("%s (%s): releasing cachedScratchMutex", title, context)
	d.cachedScratchMutex.Unlock()

	// Hot-add the scratch-space if not already attached
	if !svm.scratchAttached {
		logrus.Debugf("lcowdriver: startServiceVmIfNotRunning: (%s) hot-adding scratch %s - locking serviceVM", context, scratchTargetFile)
		svm.Lock()
		if err := svm.config.HotAddVhd(scratchTargetFile, toolsScratchPath, false, true); err != nil {
			logrus.Debugf("%s (%s): releasing serviceVM on error path of HotAddVhd: %s", title, context, err)
			svm.Unlock()

			// Do a force terminate and remove it from the map on failure, ignoring any errors
			if err2 := d.terminateServiceVM(id, "error path from HotAddVhd", true); err2 != nil {
				logrus.Warnf("failed to terminate service VM on error path from HotAddVhd: %s", err2)
			}

			return nil, fmt.Errorf("failed to hot-add %s failed: %s", scratchTargetFile, err)
		}
		logrus.Debugf("%s (%s): releasing serviceVM", title, context)
		svm.Unlock()
	}

	logrus.Debugf("lcowdriver: startServiceVmIfNotRunning: (%s) success", context)
	return svm, nil
}

// getServiceVM returns the appropriate service utility VM instance, optionally
// deleting it from the map (but not the global one)
func (d *Driver) getServiceVM(id string, deleteFromMap bool) (*serviceVMItem, error) {
	logrus.Debugf("lcowdriver: getservicevm:locking serviceVmsMutex")
	d.serviceVmsMutex.Lock()
	defer func() {
		logrus.Debugf("lcowdriver: getservicevm:releasing serviceVmsMutex")
		d.serviceVmsMutex.Unlock()
	}()
	if d.globalMode {
		id = svmGlobalID
	}
	if _, ok := d.serviceVms[id]; !ok {
		return nil, fmt.Errorf("getservicevm for %s failed as not found", id)
	}
	svm := d.serviceVms[id]
	if deleteFromMap && id != svmGlobalID {
		logrus.Debugf("lcowdriver: getservicevm: removing %s from map", id)
		delete(d.serviceVms, id)
	}
	return svm, nil
}

// terminateServiceVM terminates a service utility VM if its running, but does nothing
// when in global mode as it's lifetime is limited to that of the daemon.
func (d *Driver) terminateServiceVM(id, context string, force bool) error {

	// We don't do anything in safe mode unless the force flag has been passed, which
	// is only the case for cleanup at driver termination.
	if d.globalMode {
		if !force {
			logrus.Debugf("lcowdriver: terminateservicevm: %s (%s) - doing nothing as in global mode", id, context)
			return nil
		}
		id = svmGlobalID
	}

	// Get the service VM and delete it from the map
	svm, err := d.getServiceVM(id, true)
	if err != nil {
		return err
	}

	// We run the deletion of the scratch as a deferred function to at least attempt
	// clean-up in case of errors.
	defer func() {
		if svm.scratchAttached {
			scratchTargetFile := filepath.Join(d.dataRoot, scratchDirectory, fmt.Sprintf("%s.vhdx", id))
			logrus.Debugf("lcowdriver: terminateservicevm: %s (%s) - deleting scratch %s", id, context, scratchTargetFile)
			if err := os.Remove(scratchTargetFile); err != nil {
				logrus.Warnf("failed to remove scratch file %s (%s): %s", scratchTargetFile, context, err)
			}
		}
	}()

	// Nothing to do if it's not running
	if svm.config.Uvm != nil {
		logrus.Debugf("lcowdriver: terminateservicevm: %s (%s) - calling terminate", id, context)
		if err := svm.config.Uvm.Terminate(); err != nil {
			return fmt.Errorf("failed to terminate utility VM (%s): %s", context, err)
		}

		logrus.Debugf("lcowdriver: terminateservicevm: %s (%s) - waiting for utility VM to terminate", id, context)
		if err := svm.config.Uvm.WaitTimeout(time.Duration(svm.config.UvmTimeoutSeconds) * time.Second); err != nil {
			return fmt.Errorf("failed waiting for utility VM to terminate (%s): %s", context, err)
		}
	}

	logrus.Debugf("lcowdriver: terminateservicevm: %s (%s) - success", id, context)
	return nil
}

// String returns the string representation of a driver. This should match
// the name the graph driver has been registered with.
func (d *Driver) String() string {
	return "lcow"
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"LCOW", ""},
		// TODO: Add some more info here - mode, home, ....
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	_, err := os.Lstat(d.dir(id))
	logrus.Debugf("lcowdriver: exists: id %s %t", id, err == nil)
	return err == nil
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system. That equates to creating a sandbox.
func (d *Driver) CreateReadWrite(id, parent string, opts *graphdriver.CreateOpts) error {
	title := fmt.Sprintf("lcowdriver: createreadwrite: id %s", id)
	logrus.Debugf(title)

	// First we need to create the folder
	if err := d.Create(id, parent, opts); err != nil {
		return err
	}

	// Look for an explicit sandbox size option.
	sandboxSize := uint64(client.DefaultVhdxSizeGB)
	for k, v := range opts.StorageOpt {
		switch strings.ToLower(k) {
		case "lcow.sandboxsize":
			var err error
			sandboxSize, err = strconv.ParseUint(v, 10, 32)
			if err != nil {
				return fmt.Errorf("%s failed to parse value '%s' for 'lcow.sandboxsize'", title, v)
			}
			if sandboxSize < client.DefaultVhdxSizeGB {
				return fmt.Errorf("%s 'lcow.sandboxsize' option cannot be less than %d", title, client.DefaultVhdxSizeGB)
			}
			break
		}
	}

	// Massive perf optimisation here. If we know that the RW layer is the default size,
	// and that the cached sandbox already exists, and we are running in safe mode, we
	// can just do a simple copy into the layers sandbox file without needing to start a
	// unique service VM. For a global service VM, it doesn't really matter. Of course,
	// this is only the case where the sandbox is the default size.
	//
	// Make sure we have the sandbox mutex taken while we are examining it.
	if sandboxSize == client.DefaultVhdxSizeGB {
		logrus.Debugf("%s: locking cachedSandboxMutex", title)
		d.cachedSandboxMutex.Lock()
		_, err := os.Stat(d.cachedSandboxFile)
		logrus.Debugf("%s: releasing cachedSandboxMutex", title)
		d.cachedSandboxMutex.Unlock()
		if err == nil {
			logrus.Debugf("%s: using cached sandbox to populate", title)
			if err := client.CopyFile(d.cachedSandboxFile, filepath.Join(d.dir(id), sandboxFilename), true); err != nil {
				return err
			}
			return nil
		}
	}

	logrus.Debugf("%s: creating SVM to create sandbox", title)
	svm, err := d.startServiceVMIfNotRunning(id, nil, "createreadwrite")
	if err != nil {
		return err
	}
	defer d.terminateServiceVM(id, "createreadwrite", false)

	// So the sandbox needs creating. If default size ensure we are the only thread populating the cache.
	// Non-default size we don't store, just create them one-off so no need to lock the cachedSandboxMutex.
	if sandboxSize == client.DefaultVhdxSizeGB {
		logrus.Debugf("%s: locking cachedSandboxMutex for creation", title)
		d.cachedSandboxMutex.Lock()
		defer func() {
			logrus.Debugf("%s: releasing cachedSandboxMutex for creation", title)
			d.cachedSandboxMutex.Unlock()
		}()
	}

	// Synchronise the operation in the service VM.
	logrus.Debugf("%s: locking svm for sandbox creation", title)
	svm.Lock()
	defer func() {
		logrus.Debugf("%s: releasing svm for sandbox creation", title)
		svm.Unlock()
	}()

	// Make sure we don't write to our local cached copy if this is for a non-default size request.
	targetCacheFile := d.cachedSandboxFile
	if sandboxSize != client.DefaultVhdxSizeGB {
		targetCacheFile = ""
	}

	// Actually do the creation.
	if err := svm.config.CreateExt4Vhdx(filepath.Join(d.dir(id), sandboxFilename), uint32(sandboxSize), targetCacheFile); err != nil {
		return err
	}

	return nil
}

// Create creates the folder for the layer with the given id, and
// adds it to the layer chain.
func (d *Driver) Create(id, parent string, opts *graphdriver.CreateOpts) error {
	logrus.Debugf("lcowdriver: create: id %s parent: %s", id, parent)

	parentChain, err := d.getLayerChain(parent)
	if err != nil {
		return err
	}

	var layerChain []string
	if parent != "" {
		if !d.Exists(parent) {
			return fmt.Errorf("lcowdriver: cannot create layer folder with missing parent %s", parent)
		}
		layerChain = []string{d.dir(parent)}
	}
	layerChain = append(layerChain, parentChain...)

	// Make sure layers are created with the correct ACL so that VMs can access them.
	layerPath := d.dir(id)
	logrus.Debugf("lcowdriver: create: id %s: creating %s", id, layerPath)
	if err := system.MkdirAllWithACL(layerPath, 755, system.SddlNtvmAdministratorsLocalSystem); err != nil {
		return err
	}

	if err := d.setLayerChain(id, layerChain); err != nil {
		if err2 := os.RemoveAll(layerPath); err2 != nil {
			logrus.Warnf("failed to remove layer %s: %s", layerPath, err2)
		}
		return err
	}
	logrus.Debugf("lcowdriver: create: id %s: success", id)

	return nil
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	logrus.Debugf("lcowdriver: remove: id %s", id)
	tmpID := fmt.Sprintf("%s-removing", id)
	tmpLayerPath := d.dir(tmpID)
	layerPath := d.dir(id)

	logrus.Debugf("lcowdriver: remove: id %s: layerPath %s", id, layerPath)
	if err := os.Rename(layerPath, tmpLayerPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.RemoveAll(tmpLayerPath); err != nil {
		return err
	}

	logrus.Debugf("lcowdriver: remove: id %s: layerPath %s succeeded", id, layerPath)
	return nil
}

// Get returns the rootfs path for the id. It is reference counted and
// effectively can be thought of as a "mount the layer into the utility
// vm if it isn't already". The contract from the caller of this is that
// all Gets and Puts are matched. It -should- be the case that on cleanup,
// nothing is mounted.
//
// For optimisation, we don't actually mount the filesystem (which in our
// case means [hot-]adding it to a service VM. But we track that and defer
// the actual adding to the point we need to access it.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	title := fmt.Sprintf("lcowdriver: get: %s", id)
	logrus.Debugf(title)

	// Work out what we are working on
	ld, err := getLayerDetails(d.dir(id))
	if err != nil {
		logrus.Debugf("%s failed to get layer details from %s: %s", title, d.dir(id), err)
		return "", fmt.Errorf("%s failed to open layer or sandbox VHD to open in %s: %s", title, d.dir(id), err)
	}
	logrus.Debugf("%s %s, size %d, isSandbox %t", title, ld.filename, ld.size, ld.isSandbox)

	// Add item to cache, or update existing item, but ensure we have the
	// lock while updating items.
	logrus.Debugf("%s: locking cacheMutex", title)
	d.cacheMutex.Lock()
	var ci *cacheItem
	if item, ok := d.cache[id]; !ok {
		// The item is not currently in the cache.
		ci = &cacheItem{
			refCount:  1,
			isSandbox: ld.isSandbox,
			hostPath:  ld.filename,
			uvmPath:   fmt.Sprintf("/mnt/%s", id),
			isMounted: false, // we defer this as an optimisation
		}
		d.cache[id] = ci
		logrus.Debugf("%s: added cache item %+v", title, ci)
	} else {
		// Increment the reference counter in the cache.
		item.incrementRefCount()
	}
	logrus.Debugf("%s: releasing cacheMutex", title)
	d.cacheMutex.Unlock()

	logrus.Debugf("%s %s success. %s: %+v: size %d", title, id, d.dir(id), ci, ld.size)
	return d.dir(id), nil
}

// Put does the reverse of get. If there are no more references to
// the layer, it unmounts it from the utility VM.
func (d *Driver) Put(id string) error {
	title := fmt.Sprintf("lcowdriver: put: %s", id)

	logrus.Debugf("%s: locking cacheMutex", title)
	d.cacheMutex.Lock()
	item, ok := d.cache[id]
	if !ok {
		logrus.Debugf("%s: releasing cacheMutex on error path", title)
		d.cacheMutex.Unlock()
		return fmt.Errorf("%s possible ref-count error, or invalid id was passed to the graphdriver. Cannot handle id %s as it's not in the cache", title, id)
	}

	// Decrement the ref-count, and nothing more to do if still in use.
	if item.decrementRefCount() > 0 {
		logrus.Debugf("%s: releasing cacheMutex. Cache item is still in use", title)
		d.cacheMutex.Unlock()
		return nil
	}

	// Remove from the cache map.
	delete(d.cache, id)
	logrus.Debugf("%s: releasing cacheMutex. Ref count on cache item has dropped to zero, removed from cache", title)
	d.cacheMutex.Unlock()

	// If we have done a mount and we are in global mode, then remove it. We don't
	// need to remove in safe mode as the service VM is going to be torn down anyway.
	if d.globalMode {
		logrus.Debugf("%s: locking cache item at zero ref-count", title)
		item.Lock()
		defer func() {
			logrus.Debugf("%s: releasing cache item at zero ref-count", title)
			item.Unlock()
		}()
		if item.isMounted {
			svm, err := d.getServiceVM(id, false)
			if err != nil {
				return err
			}

			logrus.Debugf("%s: Hot-Removing %s. Locking svm", title, item.hostPath)
			svm.Lock()
			if err := svm.config.HotRemoveVhd(item.hostPath); err != nil {
				logrus.Debugf("%s: releasing svm on error path", title)
				svm.Unlock()
				return fmt.Errorf("%s failed to hot-remove %s from global service utility VM: %s", title, item.hostPath, err)
			}
			logrus.Debugf("%s: releasing svm", title)
			svm.Unlock()
		}
	}

	logrus.Debugf("%s %s: refCount 0. %s (%s) completed successfully", title, id, item.hostPath, item.uvmPath)
	return nil
}

// Cleanup ensures the information the driver stores is properly removed.
// We use this opportunity to cleanup any -removing folders which may be
// still left if the daemon was killed while it was removing a layer.
func (d *Driver) Cleanup() error {
	title := "lcowdriver: cleanup"

	d.cacheMutex.Lock()
	for k, v := range d.cache {
		logrus.Debugf("%s cache item: %s: %+v", title, k, v)
		if v.refCount > 0 {
			logrus.Warnf("%s leaked %s: %+v", title, k, v)
		}
	}
	d.cacheMutex.Unlock()

	items, err := ioutil.ReadDir(d.dataRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Note we don't return an error below - it's possible the files
	// are locked. However, next time around after the daemon exits,
	// we likely will be able to to cleanup successfully. Instead we log
	// warnings if there are errors.
	for _, item := range items {
		if item.IsDir() && strings.HasSuffix(item.Name(), "-removing") {
			if err := os.RemoveAll(filepath.Join(d.dataRoot, item.Name())); err != nil {
				logrus.Warnf("%s failed to cleanup %s: %s", title, item.Name(), err)
			} else {
				logrus.Infof("%s cleaned up %s", title, item.Name())
			}
		}
	}

	// Cleanup any service VMs we have running, along with their scratch spaces.
	// We don't take the lock for this as it's taken in terminateServiceVm.
	for k, v := range d.serviceVms {
		logrus.Debugf("%s svm: %s: %+v", title, k, v)
		d.terminateServiceVM(k, "cleanup", true)
	}

	return nil
}

// Diff takes a layer (and it's parent layer which may be null, but
// is ignored by this implementation below) and returns a reader for
// a tarstream representing the layers contents. The id could be
// a read-only "layer.vhd" or a read-write "sandbox.vhdx". The semantics
// of this function dictate that the layer is already mounted.
// However, as we do lazy mounting as a performance optimisation,
// this will likely not be the case.
func (d *Driver) Diff(id, parent string) (io.ReadCloser, error) {
	title := fmt.Sprintf("lcowdriver: diff: %s", id)

	logrus.Debugf("%s: locking cacheMutex", title)
	d.cacheMutex.Lock()
	if _, ok := d.cache[id]; !ok {
		logrus.Debugf("%s: releasing cacheMutex on error path", title)
		d.cacheMutex.Unlock()
		return nil, fmt.Errorf("%s fail as %s is not in the cache", title, id)
	}
	ci := d.cache[id]
	logrus.Debugf("%s: releasing cacheMutex", title)
	d.cacheMutex.Unlock()

	// Stat to get size
	logrus.Debugf("%s: locking cacheItem", title)
	ci.Lock()
	fileInfo, err := os.Stat(ci.hostPath)
	if err != nil {
		logrus.Debugf("%s: releasing cacheItem on error path", title)
		ci.Unlock()
		return nil, fmt.Errorf("%s failed to stat %s: %s", title, ci.hostPath, err)
	}
	logrus.Debugf("%s: releasing cacheItem", title)
	ci.Unlock()

	// Start the SVM with a mapped virtual disk. Note that if the SVM is
	// already runing and we are in global mode, this will be
	// hot-added.
	mvd := &hcsshim.MappedVirtualDisk{
		HostPath:          ci.hostPath,
		ContainerPath:     ci.uvmPath,
		CreateInUtilityVM: true,
		ReadOnly:          true,
	}

	logrus.Debugf("%s: starting service VM", title)
	svm, err := d.startServiceVMIfNotRunning(id, mvd, fmt.Sprintf("diff %s", id))
	if err != nil {
		return nil, err
	}

	// Set `isMounted` for the cache item. Note that we re-scan the cache
	// at this point as it's possible the cacheItem changed during the long-
	// running operation above when we weren't holding the cacheMutex lock.
	logrus.Debugf("%s: locking cacheMutex for updating isMounted", title)
	d.cacheMutex.Lock()
	if _, ok := d.cache[id]; !ok {
		logrus.Debugf("%s: releasing cacheMutex on error path of isMounted", title)
		d.cacheMutex.Unlock()
		d.terminateServiceVM(id, fmt.Sprintf("diff %s", id), false)
		return nil, fmt.Errorf("%s fail as %s is not in the cache when updating isMounted", title, id)
	}
	ci = d.cache[id]
	ci.setIsMounted()
	logrus.Debugf("%s: releasing cacheMutex for updating isMounted", title)
	d.cacheMutex.Unlock()

	// Obtain the tar stream for it
	logrus.Debugf("%s %s, size %d, isSandbox %t", title, ci.hostPath, fileInfo.Size(), ci.isSandbox)
	tarReadCloser, err := svm.config.VhdToTar(ci.hostPath, ci.uvmPath, ci.isSandbox, fileInfo.Size())
	if err != nil {
		d.terminateServiceVM(id, fmt.Sprintf("diff %s", id), false)
		return nil, fmt.Errorf("%s failed to export layer to tar stream for id: %s, parent: %s : %s", title, id, parent, err)
	}

	logrus.Debugf("%s id %s parent %s completed successfully", title, id, parent)

	// In safe/non-global mode, we can't tear down the service VM until things have been read.
	if !d.globalMode {
		return ioutils.NewReadCloserWrapper(tarReadCloser, func() error {
			tarReadCloser.Close()
			d.terminateServiceVM(id, fmt.Sprintf("diff %s", id), false)
			return nil
		}), nil
	}
	return tarReadCloser, nil
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes. The layer should not be mounted when calling
// this function. Another way of describing this is that ApplyDiff writes
// to a new layer (a VHD in LCOW) the contents of a tarstream it's given.
func (d *Driver) ApplyDiff(id, parent string, diff io.Reader) (int64, error) {
	logrus.Debugf("lcowdriver: applydiff: id %s", id)

	svm, err := d.startServiceVMIfNotRunning(id, nil, fmt.Sprintf("applydiff %s", id))
	if err != nil {
		return 0, err
	}
	defer d.terminateServiceVM(id, fmt.Sprintf("applydiff %s", id), false)

	// TODO @jhowardmsft - the retries are temporary to overcome platform reliablity issues.
	// Obviously this will be removed as platform bugs are fixed.
	retries := 0
	for {
		retries++
		size, err := svm.config.TarToVhd(filepath.Join(d.dataRoot, id, layerFilename), diff)
		if err != nil {
			if retries <= 10 {
				continue
			}
			return 0, err
		}
		return size, err
	}
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
// The layer should not be mounted when calling this function.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	logrus.Debugf("lcowdriver: changes: id %s parent %s", id, parent)
	// TODO @gupta-ak. Needs implementation with assistance from service VM
	return nil, nil
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	logrus.Debugf("lcowdriver: diffsize: id %s", id)
	// TODO @gupta-ak. Needs implementation with assistance from service VM
	return 0, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	logrus.Debugf("lcowdriver: getmetadata: id %s", id)
	m := make(map[string]string)
	m["dir"] = d.dir(id)
	return m, nil
}

// dir returns the absolute path to the layer.
func (d *Driver) dir(id string) string {
	return filepath.Join(d.dataRoot, filepath.Base(id))
}

// getLayerChain returns the layer chain information.
func (d *Driver) getLayerChain(id string) ([]string, error) {
	jPath := filepath.Join(d.dir(id), "layerchain.json")
	logrus.Debugf("lcowdriver: getlayerchain: id %s json %s", id, jPath)
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("lcowdriver: getlayerchain: %s unable to read layerchain file %s: %s", id, jPath, err)
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("lcowdriver: getlayerchain: %s failed to unmarshall layerchain file %s: %s", id, jPath, err)
	}
	return layerChain, nil
}

// setLayerChain stores the layer chain information on disk.
func (d *Driver) setLayerChain(id string, chain []string) error {
	content, err := json.Marshal(&chain)
	if err != nil {
		return fmt.Errorf("lcowdriver: setlayerchain: %s failed to marshall layerchain json: %s", id, err)
	}

	jPath := filepath.Join(d.dir(id), "layerchain.json")
	logrus.Debugf("lcowdriver: setlayerchain: id %s json %s", id, jPath)
	err = ioutil.WriteFile(jPath, content, 0600)
	if err != nil {
		return fmt.Errorf("lcowdriver: setlayerchain: %s failed to write layerchain file: %s", id, err)
	}
	return nil
}

// getLayerDetails is a utility for getting a file name, size and indication of
// sandbox for a VHD(x) in a folder. A read-only layer will be layer.vhd. A
// read-write layer will be sandbox.vhdx.
func getLayerDetails(folder string) (*layerDetails, error) {
	var fileInfo os.FileInfo
	ld := &layerDetails{
		isSandbox: false,
		filename:  filepath.Join(folder, layerFilename),
	}

	fileInfo, err := os.Stat(ld.filename)
	if err != nil {
		ld.filename = filepath.Join(folder, sandboxFilename)
		if fileInfo, err = os.Stat(ld.filename); err != nil {
			return nil, fmt.Errorf("failed to locate layer or sandbox in %s", folder)
		}
		ld.isSandbox = true
	}
	ld.size = fileInfo.Size()

	return ld, nil
}
