//+build windows

package windows

import (
	"bufio"
	"bytes"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/archive/tar"
	"github.com/Microsoft/go-winio/backuptar"
	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/longpath"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/system"
	"github.com/vbatts/tar-split/tar/storage"
)

// filterDriver is an HCSShim driver type for the Windows Filter driver.
const filterDriver = 1

// init registers the windows graph drivers to the register.
func init() {
	graphdriver.Register("windowsfilter", InitFilter)
	reexec.Register("docker-windows-write-layer", writeLayer)
}

type checker struct {
}

func (c *checker) IsMounted(path string) bool {
	return false
}

// Driver represents a windows graph driver.
type Driver struct {
	// info stores the shim driver information
	info hcsshim.DriverInfo
	ctr  *graphdriver.RefCounter
	// it is safe for windows to use a cache here because it does not support
	// restoring containers when the daemon dies.
	cacheMu sync.Mutex
	cache   map[string]string
}

func isTP5OrOlder() bool {
	return system.GetOSVersion().Build <= 14300
}

// InitFilter returns a new Windows storage filter driver.
func InitFilter(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("WindowsGraphDriver InitFilter at %s", home)
	d := &Driver{
		info: hcsshim.DriverInfo{
			HomeDir: home,
			Flavour: filterDriver,
		},
		cache: make(map[string]string),
		ctr:   graphdriver.NewRefCounter(&checker{}),
	}
	return d, nil
}

// String returns the string representation of a driver. This should match
// the name the graph driver has been registered with.
func (d *Driver) String() string {
	return "windowsfilter"
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Windows", ""},
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	rID, err := d.resolveID(id)
	if err != nil {
		return false
	}
	result, err := hcsshim.LayerExists(d.info, rID)
	if err != nil {
		return false
	}
	return result
}

// CreateReadWrite creates a layer that is writable for use as a container
// file system.
func (d *Driver) CreateReadWrite(id, parent, mountLabel string, storageOpt map[string]string) error {
	return d.create(id, parent, mountLabel, false, storageOpt)
}

// Create creates a new read-only layer with the given id.
func (d *Driver) Create(id, parent, mountLabel string, storageOpt map[string]string) error {
	return d.create(id, parent, mountLabel, true, storageOpt)
}

func (d *Driver) create(id, parent, mountLabel string, readOnly bool, storageOpt map[string]string) error {
	if len(storageOpt) != 0 {
		return fmt.Errorf("--storage-opt is not supported for windows")
	}

	rPId, err := d.resolveID(parent)
	if err != nil {
		return err
	}

	parentChain, err := d.getLayerChain(rPId)
	if err != nil {
		return err
	}

	var layerChain []string

	if rPId != "" {
		parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(parentPath, "Files")); err == nil {
			// This is a legitimate parent layer (not the empty "-init" layer),
			// so include it in the layer chain.
			layerChain = []string{parentPath}
		}
	}

	layerChain = append(layerChain, parentChain...)

	if readOnly {
		if err := hcsshim.CreateLayer(d.info, id, rPId); err != nil {
			return err
		}
	} else {
		var parentPath string
		if len(layerChain) != 0 {
			parentPath = layerChain[0]
		}

		if isTP5OrOlder() {
			// Pre-create the layer directory, providing an ACL to give the Hyper-V Virtual Machines
			// group access. This is necessary to ensure that Hyper-V containers can access the
			// virtual machine data. This is not necessary post-TP5.
			path, err := syscall.UTF16FromString(filepath.Join(d.info.HomeDir, id))
			if err != nil {
				return err
			}
			// Give system and administrators full control, and VMs read, write, and execute.
			// Mark these ACEs as inherited.
			sd, err := winio.SddlToSecurityDescriptor("D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FRFWFX;;;S-1-5-83-0)")
			if err != nil {
				return err
			}
			err = syscall.CreateDirectory(&path[0], &syscall.SecurityAttributes{
				Length:             uint32(unsafe.Sizeof(syscall.SecurityAttributes{})),
				SecurityDescriptor: uintptr(unsafe.Pointer(&sd[0])),
			})
			if err != nil {
				return err
			}
		}

		if err := hcsshim.CreateSandboxLayer(d.info, id, parentPath, layerChain); err != nil {
			return err
		}
	}

	if _, err := os.Lstat(d.dir(parent)); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return fmt.Errorf("Cannot create layer with missing parent %s: %s", parent, err)
	}

	if err := d.setLayerChain(id, layerChain); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return err
	}

	return nil
}

// dir returns the absolute path to the layer.
func (d *Driver) dir(id string) string {
	return filepath.Join(d.info.HomeDir, filepath.Base(id))
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}
	os.RemoveAll(filepath.Join(d.info.HomeDir, "sysfile-backups", rID)) // ok to fail
	return hcsshim.DestroyLayer(d.info, rID)
}

// Get returns the rootfs path for the id. This will mount the dir at it's given path.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	logrus.Debugf("WindowsGraphDriver Get() id %s mountLabel %s", id, mountLabel)
	var dir string

	rID, err := d.resolveID(id)
	if err != nil {
		return "", err
	}
	if count := d.ctr.Increment(rID); count > 1 {
		return d.cache[rID], nil
	}

	// Getting the layer paths must be done outside of the lock.
	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		d.ctr.Decrement(rID)
		return "", err
	}

	if err := hcsshim.ActivateLayer(d.info, rID); err != nil {
		d.ctr.Decrement(rID)
		return "", err
	}
	if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
		d.ctr.Decrement(rID)
		if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", id, err)
		}
		return "", err
	}

	mountPath, err := hcsshim.GetLayerMountPath(d.info, rID)
	if err != nil {
		d.ctr.Decrement(rID)
		if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", id, err)
		}
		return "", err
	}
	d.cacheMu.Lock()
	d.cache[rID] = mountPath
	d.cacheMu.Unlock()

	// If the layer has a mount path, use that. Otherwise, use the
	// folder path.
	if mountPath != "" {
		dir = mountPath
	} else {
		dir = d.dir(id)
	}

	return dir, nil
}

// Put adds a new layer to the driver.
func (d *Driver) Put(id string) error {
	logrus.Debugf("WindowsGraphDriver Put() id %s", id)

	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}
	if count := d.ctr.Decrement(rID); count > 0 {
		return nil
	}
	d.cacheMu.Lock()
	delete(d.cache, rID)
	d.cacheMu.Unlock()

	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return err
	}
	return hcsshim.DeactivateLayer(d.info, rID)
}

// Cleanup ensures the information the driver stores is properly removed.
func (d *Driver) Cleanup() error {
	return nil
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
// The layer should be mounted when calling this function
func (d *Driver) Diff(id, parent string) (_ archive.Archive, err error) {
	rID, err := d.resolveID(id)
	if err != nil {
		return
	}

	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		return
	}

	// this is assuming that the layer is unmounted
	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return nil, err
	}
	prepare := func() {
		if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", rID, err)
		}
	}

	arch, err := d.exportLayer(rID, layerChain)
	if err != nil {
		prepare()
		return
	}
	return ioutils.NewReadCloserWrapper(arch, func() error {
		err := arch.Close()
		prepare()
		return err
	}), nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
// The layer should be mounted when calling this function
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	rID, err := d.resolveID(id)
	if err != nil {
		return nil, err
	}
	parentChain, err := d.getLayerChain(rID)
	if err != nil {
		return nil, err
	}

	// this is assuming that the layer is unmounted
	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return nil, err
	}
	defer func() {
		if err := hcsshim.PrepareLayer(d.info, rID, parentChain); err != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", rID, err)
		}
	}()

	var changes []archive.Change
	err = winio.RunWithPrivilege(winio.SeBackupPrivilege, func() error {
		r, err := hcsshim.NewLayerReader(d.info, id, parentChain)
		if err != nil {
			return err
		}
		defer r.Close()

		for {
			name, _, fileInfo, err := r.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			name = filepath.ToSlash(name)
			if fileInfo == nil {
				changes = append(changes, archive.Change{Path: name, Kind: archive.ChangeDelete})
			} else {
				// Currently there is no way to tell between an add and a modify.
				changes = append(changes, archive.Change{Path: name, Kind: archive.ChangeModify})
			}
		}
	})
	if err != nil {
		return nil, err
	}

	return changes, nil
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
// The layer should not be mounted when calling this function
func (d *Driver) ApplyDiff(id, parent string, diff archive.Reader) (int64, error) {
	var layerChain []string
	if parent != "" {
		rPId, err := d.resolveID(parent)
		if err != nil {
			return 0, err
		}
		parentChain, err := d.getLayerChain(rPId)
		if err != nil {
			return 0, err
		}
		parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
		if err != nil {
			return 0, err
		}
		layerChain = append(layerChain, parentPath)
		layerChain = append(layerChain, parentChain...)
	}

	size, err := d.importLayer(id, diff, layerChain)
	if err != nil {
		return 0, err
	}

	if err = d.setLayerChain(id, layerChain); err != nil {
		return 0, err
	}

	return size, nil
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return
	}

	changes, err := d.Changes(id, rPId)
	if err != nil {
		return
	}

	layerFs, err := d.Get(id, "")
	if err != nil {
		return
	}
	defer d.Put(id)

	return archive.ChangesSize(layerFs, changes), nil
}

// CustomImageInfo is the object returned by the driver describing the base
// image.
type CustomImageInfo struct {
	ID          string
	Name        string
	Version     string
	Path        string
	Size        int64
	CreatedTime time.Time
	OSVersion   string   `json:"-"`
	OSFeatures  []string `json:"-"`
}

// GetCustomImageInfos returns the image infos for window specific
// base images which should always be present.
func (d *Driver) GetCustomImageInfos() ([]CustomImageInfo, error) {
	strData, err := hcsshim.GetSharedBaseImages()
	if err != nil {
		return nil, fmt.Errorf("Failed to restore base images: %s", err)
	}

	type customImageInfoList struct {
		Images []CustomImageInfo
	}

	var infoData customImageInfoList

	if err = json.Unmarshal([]byte(strData), &infoData); err != nil {
		err = fmt.Errorf("JSON unmarshal returned error=%s", err)
		logrus.Error(err)
		return nil, err
	}

	var images []CustomImageInfo

	for _, imageData := range infoData.Images {
		folderName := filepath.Base(imageData.Path)

		// Use crypto hash of the foldername to generate a docker style id.
		h := sha512.Sum384([]byte(folderName))
		id := fmt.Sprintf("%x", h[:32])

		if err := d.Create(id, "", "", nil); err != nil {
			return nil, err
		}
		// Create the alternate ID file.
		if err := d.setID(id, folderName); err != nil {
			return nil, err
		}

		imageData.ID = id

		// For now, hard code that all base images except nanoserver depend on win32k support
		if imageData.Name != "NanoServer" {
			imageData.OSFeatures = append(imageData.OSFeatures, "win32k")
		}

		versionData := strings.Split(imageData.Version, ".")
		if len(versionData) != 4 {
			logrus.Warn("Could not parse Windows version %s", imageData.Version)
		} else {
			// Include just major.minor.build, skip the fourth version field, which does not influence
			// OS compatibility.
			imageData.OSVersion = strings.Join(versionData[:3], ".")
		}

		images = append(images, imageData)
	}

	return images, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	m := make(map[string]string)
	m["dir"] = d.dir(id)
	return m, nil
}

func writeTarFromLayer(r hcsshim.LayerReader, w io.Writer) error {
	t := tar.NewWriter(w)
	for {
		name, size, fileInfo, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if fileInfo == nil {
			// Write a whiteout file.
			hdr := &tar.Header{
				Name: filepath.ToSlash(filepath.Join(filepath.Dir(name), archive.WhiteoutPrefix+filepath.Base(name))),
			}
			err := t.WriteHeader(hdr)
			if err != nil {
				return err
			}
		} else {
			err = backuptar.WriteTarFileFromBackupStream(t, r, name, size, fileInfo)
			if err != nil {
				return err
			}
		}
	}
	return t.Close()
}

// exportLayer generates an archive from a layer based on the given ID.
func (d *Driver) exportLayer(id string, parentLayerPaths []string) (archive.Archive, error) {
	archive, w := io.Pipe()
	go func() {
		err := winio.RunWithPrivilege(winio.SeBackupPrivilege, func() error {
			r, err := hcsshim.NewLayerReader(d.info, id, parentLayerPaths)
			if err != nil {
				return err
			}

			err = writeTarFromLayer(r, w)
			cerr := r.Close()
			if err == nil {
				err = cerr
			}
			return err
		})
		w.CloseWithError(err)
	}()

	return archive, nil
}

func writeLayerFromTar(r archive.Reader, w hcsshim.LayerWriter) (int64, error) {
	t := tar.NewReader(r)
	hdr, err := t.Next()
	totalSize := int64(0)
	buf := bufio.NewWriter(nil)
	for err == nil {
		base := path.Base(hdr.Name)
		if strings.HasPrefix(base, archive.WhiteoutPrefix) {
			name := path.Join(path.Dir(hdr.Name), base[len(archive.WhiteoutPrefix):])
			err = w.Remove(filepath.FromSlash(name))
			if err != nil {
				return 0, err
			}
			hdr, err = t.Next()
		} else if hdr.Typeflag == tar.TypeLink {
			err = w.AddLink(filepath.FromSlash(hdr.Name), filepath.FromSlash(hdr.Linkname))
			if err != nil {
				return 0, err
			}
			hdr, err = t.Next()
		} else {
			var (
				name     string
				size     int64
				fileInfo *winio.FileBasicInfo
			)
			name, size, fileInfo, err = backuptar.FileInfoFromHeader(hdr)
			if err != nil {
				return 0, err
			}
			err = w.Add(filepath.FromSlash(name), fileInfo)
			if err != nil {
				return 0, err
			}
			buf.Reset(w)

			// Add the Hyper-V Virtual Machine group ACE to the security descriptor
			// for TP5 so that Xenons can access all files. This is not necessary
			// for post-TP5 builds.
			if isTP5OrOlder() {
				if sddl, ok := hdr.Winheaders["sd"]; ok {
					var ace string
					if hdr.Typeflag == tar.TypeDir {
						ace = "(A;OICI;0x1200a9;;;S-1-5-83-0)"
					} else {
						ace = "(A;;0x1200a9;;;S-1-5-83-0)"
					}
					if hdr.Winheaders["sd"], ok = addAceToSddlDacl(sddl, ace); !ok {
						logrus.Debugf("failed to add VM ACE to %s", sddl)
					}
				}
			}

			hdr, err = backuptar.WriteBackupStreamFromTarFile(buf, t, hdr)
			ferr := buf.Flush()
			if ferr != nil {
				err = ferr
			}
			totalSize += size
		}
	}
	if err != io.EOF {
		return 0, err
	}
	return totalSize, nil
}

func addAceToSddlDacl(sddl, ace string) (string, bool) {
	daclStart := strings.Index(sddl, "D:")
	if daclStart < 0 {
		return sddl, false
	}

	dacl := sddl[daclStart:]
	daclEnd := strings.Index(dacl, "S:")
	if daclEnd < 0 {
		daclEnd = len(dacl)
	}
	dacl = dacl[:daclEnd]

	if strings.Contains(dacl, ace) {
		return sddl, true
	}

	i := 2
	for i+1 < len(dacl) {
		if dacl[i] != '(' {
			return sddl, false
		}

		if dacl[i+1] == 'A' {
			break
		}

		i += 2
		for p := 1; i < len(dacl) && p > 0; i++ {
			if dacl[i] == '(' {
				p++
			} else if dacl[i] == ')' {
				p--
			}
		}
	}

	return sddl[:daclStart+i] + ace + sddl[daclStart+i:], true
}

// importLayer adds a new layer to the tag and graph store based on the given data.
func (d *Driver) importLayer(id string, layerData archive.Reader, parentLayerPaths []string) (size int64, err error) {
	cmd := reexec.Command(append([]string{"docker-windows-write-layer", d.info.HomeDir, id}, parentLayerPaths...)...)
	output := bytes.NewBuffer(nil)
	cmd.Stdin = layerData
	cmd.Stdout = output
	cmd.Stderr = output

	if err = cmd.Start(); err != nil {
		return
	}

	if err = cmd.Wait(); err != nil {
		return 0, fmt.Errorf("re-exec error: %v: output: %s", err, output)
	}

	return strconv.ParseInt(output.String(), 10, 64)
}

// writeLayer is the re-exec entry point for writing a layer from a tar file
func writeLayer() {
	home := os.Args[1]
	id := os.Args[2]
	parentLayerPaths := os.Args[3:]

	err := func() error {
		err := winio.EnableProcessPrivileges([]string{winio.SeBackupPrivilege, winio.SeRestorePrivilege})
		if err != nil {
			return err
		}

		info := hcsshim.DriverInfo{
			Flavour: filterDriver,
			HomeDir: home,
		}

		w, err := hcsshim.NewLayerWriter(info, id, parentLayerPaths)
		if err != nil {
			return err
		}

		size, err := writeLayerFromTar(os.Stdin, w)
		if err != nil {
			return err
		}

		err = w.Close()
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stdout, size)
		return nil
	}()

	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveID computes the layerID information based on the given id.
func (d *Driver) resolveID(id string) (string, error) {
	content, err := ioutil.ReadFile(filepath.Join(d.dir(id), "layerID"))
	if os.IsNotExist(err) {
		return id, nil
	} else if err != nil {
		return "", err
	}
	return string(content), nil
}

// setID stores the layerId in disk.
func (d *Driver) setID(id, altID string) error {
	err := ioutil.WriteFile(filepath.Join(d.dir(id), "layerId"), []byte(altID), 0600)
	if err != nil {
		return err
	}
	return nil
}

// getLayerChain returns the layer chain information.
func (d *Driver) getLayerChain(id string) ([]string, error) {
	jPath := filepath.Join(d.dir(id), "layerchain.json")
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("Unable to read layerchain file - %s", err)
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall layerchain json - %s", err)
	}

	return layerChain, nil
}

// setLayerChain stores the layer chain information in disk.
func (d *Driver) setLayerChain(id string, chain []string) error {
	content, err := json.Marshal(&chain)
	if err != nil {
		return fmt.Errorf("Failed to marshall layerchain json - %s", err)
	}

	jPath := filepath.Join(d.dir(id), "layerchain.json")
	err = ioutil.WriteFile(jPath, content, 0600)
	if err != nil {
		return fmt.Errorf("Unable to write layerchain file - %s", err)
	}

	return nil
}

type fileGetCloserWithBackupPrivileges struct {
	path string
}

func (fg *fileGetCloserWithBackupPrivileges) Get(filename string) (io.ReadCloser, error) {
	var f *os.File
	// Open the file while holding the Windows backup privilege. This ensures that the
	// file can be opened even if the caller does not actually have access to it according
	// to the security descriptor.
	err := winio.RunWithPrivilege(winio.SeBackupPrivilege, func() error {
		path := longpath.AddPrefix(filepath.Join(fg.path, filename))
		p, err := syscall.UTF16FromString(path)
		if err != nil {
			return err
		}
		h, err := syscall.CreateFile(&p[0], syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			return &os.PathError{Op: "open", Path: path, Err: err}
		}
		f = os.NewFile(uintptr(h), path)
		return nil
	})
	return f, err
}

func (fg *fileGetCloserWithBackupPrivileges) Close() error {
	return nil
}

type fileGetDestroyCloser struct {
	storage.FileGetter
	path string
}

func (f *fileGetDestroyCloser) Close() error {
	// TODO: activate layers and release here?
	return os.RemoveAll(f.path)
}

// DiffGetter returns a FileGetCloser that can read files from the directory that
// contains files for the layer differences. Used for direct access for tar-split.
func (d *Driver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	id, err := d.resolveID(id)
	if err != nil {
		return nil, err
	}

	return &fileGetCloserWithBackupPrivileges{d.dir(id)}, nil
}
