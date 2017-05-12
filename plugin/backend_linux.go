// +build linux

package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/distribution"
	progressutils "github.com/docker/docker/distribution/utils"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/authorization"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/plugin/v2"
	refstore "github.com/docker/docker/reference"
	"github.com/opencontainers/go-digest"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const mediaTypePluginConfig string = "application/vnd.oci.image.manifest.v1+json"

var acceptedPluginFilterTags = map[string]bool{
	"enabled":    true,
	"capability": true,
}

const (
	manifestFileName = "manifest.json"
)

// Disable deactivates a plugin. This means resources (volumes, networks) cant use them.
func (pm *Manager) Disable(refOrID string, config *types.PluginDisableConfig) error {
	p, err := pm.config.Store.GetV2Plugin(refOrID)
	if err != nil {
		return err
	}
	pm.mu.RLock()
	c := pm.cMap[p]
	pm.mu.RUnlock()

	if !config.ForceDisable && p.GetRefCount() > 0 {
		return fmt.Errorf("plugin %s is in use", p.Name())
	}

	for _, typ := range p.GetTypes() {
		if typ.Capability == authorization.AuthZApiImplements {
			authzList := pm.config.AuthzMiddleware.GetAuthzPlugins()
			for i, authPlugin := range authzList {
				if authPlugin.Name() == p.Name() {
					// Remove plugin from authzmiddleware chain
					authzList = append(authzList[:i], authzList[i+1:]...)
					pm.config.AuthzMiddleware.SetAuthzPlugins(authzList)
				}
			}
		}
	}

	if err := pm.disable(p, c); err != nil {
		return err
	}
	pm.config.LogPluginEvent(p.GetID(), refOrID, "disable")
	return nil
}

// Enable activates a plugin, which implies that they are ready to be used by containers.
func (pm *Manager) Enable(refOrID string, config *types.PluginEnableConfig) error {
	p, err := pm.config.Store.GetV2Plugin(refOrID)
	if err != nil {
		return err
	}

	c := &controller{timeoutInSecs: config.Timeout}
	if err := pm.enable(p, c, false); err != nil {
		return err
	}
	pm.config.LogPluginEvent(p.GetID(), refOrID, "enable")
	return nil
}

// Inspect examines a plugin config
func (pm *Manager) Inspect(refOrID string) (tp *types.Plugin, err error) {
	p, err := pm.config.Store.GetV2Plugin(refOrID)
	if err != nil {
		return nil, err
	}

	return &p.PluginObj, nil
}

func (pm *Manager) pull(ctx context.Context, ref reference.Named, config *distribution.ImagePullConfig, outStream io.Writer) error {
	if outStream != nil {
		// Include a buffer so that slow client connections don't affect
		// transfer performance.
		progressChan := make(chan progress.Progress, 100)

		writesDone := make(chan struct{})

		defer func() {
			close(progressChan)
			<-writesDone
		}()

		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithCancel(ctx)

		go func() {
			progressutils.WriteDistributionProgress(cancelFunc, outStream, progressChan)
			close(writesDone)
		}()

		config.ProgressOutput = progress.ChanOutput(progressChan)
	} else {
		config.ProgressOutput = progress.DiscardOutput()
	}
	return distribution.Pull(ctx, ref, config)
}

type tempConfigStore struct {
	config       []byte
	configDigest digest.Digest
}

func (s *tempConfigStore) Put(c []byte) (digest.Digest, error) {
	dgst := digest.FromBytes(c)

	s.config = c
	s.configDigest = dgst

	return dgst, nil
}

func (s *tempConfigStore) Get(d digest.Digest) ([]byte, error) {
	if d != s.configDigest {
		return nil, fmt.Errorf("digest not found")
	}
	return s.config, nil
}

func (s *tempConfigStore) RootFSFromConfig(c []byte) (*image.RootFS, error) {
	return configToRootFS(c)
}

func computePrivileges(c types.PluginConfig) (types.PluginPrivileges, error) {
	var privileges types.PluginPrivileges
	if c.Network.Type != "null" && c.Network.Type != "bridge" && c.Network.Type != "" {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "network",
			Description: "permissions to access a network",
			Value:       []string{c.Network.Type},
		})
	}
	if c.IpcHost {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "host ipc namespace",
			Description: "allow access to host ipc namespace",
			Value:       []string{"true"},
		})
	}
	if c.PidHost {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "host pid namespace",
			Description: "allow access to host pid namespace",
			Value:       []string{"true"},
		})
	}
	for _, mount := range c.Mounts {
		if mount.Source != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "mount",
				Description: "host path to mount",
				Value:       []string{*mount.Source},
			})
		}
	}
	for _, device := range c.Linux.Devices {
		if device.Path != nil {
			privileges = append(privileges, types.PluginPrivilege{
				Name:        "device",
				Description: "host device to access",
				Value:       []string{*device.Path},
			})
		}
	}
	if c.Linux.AllowAllDevices {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "allow-all-devices",
			Description: "allow 'rwm' access to all devices",
			Value:       []string{"true"},
		})
	}
	if len(c.Linux.Capabilities) > 0 {
		privileges = append(privileges, types.PluginPrivilege{
			Name:        "capabilities",
			Description: "list of additional capabilities required",
			Value:       c.Linux.Capabilities,
		})
	}

	return privileges, nil
}

// Privileges pulls a plugin config and computes the privileges required to install it.
func (pm *Manager) Privileges(ctx context.Context, ref reference.Named, metaHeader http.Header, authConfig *types.AuthConfig) (types.PluginPrivileges, error) {
	// create image store instance
	cs := &tempConfigStore{}

	// DownloadManager not defined because only pulling configuration.
	pluginPullConfig := &distribution.ImagePullConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeader,
			AuthConfig:       authConfig,
			RegistryService:  pm.config.RegistryService,
			ImageEventLogger: func(string, string, string) {},
			ImageStore:       cs,
		},
		Schema2Types: distribution.PluginTypes,
	}

	if err := pm.pull(ctx, ref, pluginPullConfig, nil); err != nil {
		return nil, err
	}

	if cs.config == nil {
		return nil, errors.New("no configuration pulled")
	}
	var config types.PluginConfig
	if err := json.Unmarshal(cs.config, &config); err != nil {
		return nil, err
	}

	return computePrivileges(config)
}

// Upgrade upgrades a plugin
func (pm *Manager) Upgrade(ctx context.Context, ref reference.Named, name string, metaHeader http.Header, authConfig *types.AuthConfig, privileges types.PluginPrivileges, outStream io.Writer) (err error) {
	p, err := pm.config.Store.GetV2Plugin(name)
	if err != nil {
		return errors.Wrap(err, "plugin must be installed before upgrading")
	}

	if p.IsEnabled() {
		return fmt.Errorf("plugin must be disabled before upgrading")
	}

	pm.muGC.RLock()
	defer pm.muGC.RUnlock()

	// revalidate because Pull is public
	if _, err := reference.ParseNormalizedNamed(name); err != nil {
		return errors.Wrapf(err, "failed to parse %q", name)
	}

	tmpRootFSDir, err := ioutil.TempDir(pm.tmpDir(), ".rootfs")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRootFSDir)

	dm := &downloadManager{
		tmpDir:    tmpRootFSDir,
		blobStore: pm.blobStore,
	}

	pluginPullConfig := &distribution.ImagePullConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeader,
			AuthConfig:       authConfig,
			RegistryService:  pm.config.RegistryService,
			ImageEventLogger: pm.config.LogPluginEvent,
			ImageStore:       dm,
		},
		DownloadManager: dm, // todo: reevaluate if possible to substitute distribution/xfer dependencies instead
		Schema2Types:    distribution.PluginTypes,
	}

	err = pm.pull(ctx, ref, pluginPullConfig, outStream)
	if err != nil {
		go pm.GC()
		return err
	}

	if err := pm.upgradePlugin(p, dm.configDigest, dm.blobs, tmpRootFSDir, &privileges); err != nil {
		return err
	}
	p.PluginObj.PluginReference = ref.String()
	return nil
}

// Pull pulls a plugin, check if the correct privileges are provided and install the plugin.
func (pm *Manager) Pull(ctx context.Context, ref reference.Named, name string, metaHeader http.Header, authConfig *types.AuthConfig, privileges types.PluginPrivileges, outStream io.Writer) (err error) {
	pm.muGC.RLock()
	defer pm.muGC.RUnlock()

	// revalidate because Pull is public
	nameref, err := reference.ParseNormalizedNamed(name)
	if err != nil {
		return errors.Wrapf(err, "failed to parse %q", name)
	}
	name = reference.FamiliarString(reference.TagNameOnly(nameref))

	if err := pm.config.Store.validateName(name); err != nil {
		return err
	}

	tmpRootFSDir, err := ioutil.TempDir(pm.tmpDir(), ".rootfs")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRootFSDir)

	dm := &downloadManager{
		tmpDir:    tmpRootFSDir,
		blobStore: pm.blobStore,
	}

	pluginPullConfig := &distribution.ImagePullConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeader,
			AuthConfig:       authConfig,
			RegistryService:  pm.config.RegistryService,
			ImageEventLogger: pm.config.LogPluginEvent,
			ImageStore:       dm,
		},
		DownloadManager: dm, // todo: reevaluate if possible to substitute distribution/xfer dependencies instead
		Schema2Types:    distribution.PluginTypes,
	}

	err = pm.pull(ctx, ref, pluginPullConfig, outStream)
	if err != nil {
		go pm.GC()
		return err
	}

	p, err := pm.createPlugin(name, dm.configDigest, dm.blobs, tmpRootFSDir, &privileges, nil)
	if err != nil {
		return err
	}
	p.PluginObj.PluginReference = ref.String()

	return nil
}

// List displays the list of plugins and associated metadata.
func (pm *Manager) List(pluginFilters filters.Args) ([]types.Plugin, error) {
	if err := pluginFilters.Validate(acceptedPluginFilterTags); err != nil {
		return nil, err
	}

	enabledOnly := false
	disabledOnly := false
	if pluginFilters.Include("enabled") {
		if pluginFilters.ExactMatch("enabled", "true") {
			enabledOnly = true
		} else if pluginFilters.ExactMatch("enabled", "false") {
			disabledOnly = true
		} else {
			return nil, fmt.Errorf("Invalid filter 'enabled=%s'", pluginFilters.Get("enabled"))
		}
	}

	plugins := pm.config.Store.GetAll()
	out := make([]types.Plugin, 0, len(plugins))

next:
	for _, p := range plugins {
		if enabledOnly && !p.PluginObj.Enabled {
			continue
		}
		if disabledOnly && p.PluginObj.Enabled {
			continue
		}
		if pluginFilters.Include("capability") {
			for _, f := range p.GetTypes() {
				if !pluginFilters.Match("capability", f.Capability) {
					continue next
				}
			}
		}
		out = append(out, p.PluginObj)
	}
	return out, nil
}

// Push pushes a plugin to the store.
func (pm *Manager) Push(ctx context.Context, name string, metaHeader http.Header, authConfig *types.AuthConfig, outStream io.Writer) error {
	p, err := pm.config.Store.GetV2Plugin(name)
	if err != nil {
		return err
	}

	ref, err := reference.ParseNormalizedNamed(p.Name())
	if err != nil {
		return errors.Wrapf(err, "plugin has invalid name %v for push", p.Name())
	}

	var po progress.Output
	if outStream != nil {
		// Include a buffer so that slow client connections don't affect
		// transfer performance.
		progressChan := make(chan progress.Progress, 100)

		writesDone := make(chan struct{})

		defer func() {
			close(progressChan)
			<-writesDone
		}()

		var cancelFunc context.CancelFunc
		ctx, cancelFunc = context.WithCancel(ctx)

		go func() {
			progressutils.WriteDistributionProgress(cancelFunc, outStream, progressChan)
			close(writesDone)
		}()

		po = progress.ChanOutput(progressChan)
	} else {
		po = progress.DiscardOutput()
	}

	// TODO: replace these with manager
	is := &pluginConfigStore{
		pm:     pm,
		plugin: p,
	}
	ls := &pluginLayerProvider{
		pm:     pm,
		plugin: p,
	}
	rs := &pluginReference{
		name:     ref,
		pluginID: p.Config,
	}

	uploadManager := xfer.NewLayerUploadManager(3)

	imagePushConfig := &distribution.ImagePushConfig{
		Config: distribution.Config{
			MetaHeaders:      metaHeader,
			AuthConfig:       authConfig,
			ProgressOutput:   po,
			RegistryService:  pm.config.RegistryService,
			ReferenceStore:   rs,
			ImageEventLogger: pm.config.LogPluginEvent,
			ImageStore:       is,
			RequireSchema2:   true,
		},
		ConfigMediaType: schema2.MediaTypePluginConfig,
		LayerStore:      ls,
		UploadManager:   uploadManager,
	}

	return distribution.Push(ctx, ref, imagePushConfig)
}

type pluginReference struct {
	name     reference.Named
	pluginID digest.Digest
}

func (r *pluginReference) References(id digest.Digest) []reference.Named {
	if r.pluginID != id {
		return nil
	}
	return []reference.Named{r.name}
}

func (r *pluginReference) ReferencesByName(ref reference.Named) []refstore.Association {
	return []refstore.Association{
		{
			Ref: r.name,
			ID:  r.pluginID,
		},
	}
}

func (r *pluginReference) Get(ref reference.Named) (digest.Digest, error) {
	if r.name.String() != ref.String() {
		return digest.Digest(""), refstore.ErrDoesNotExist
	}
	return r.pluginID, nil
}

func (r *pluginReference) AddTag(ref reference.Named, id digest.Digest, force bool) error {
	// Read only, ignore
	return nil
}
func (r *pluginReference) AddDigest(ref reference.Canonical, id digest.Digest, force bool) error {
	// Read only, ignore
	return nil
}
func (r *pluginReference) Delete(ref reference.Named) (bool, error) {
	// Read only, ignore
	return false, nil
}

type pluginConfigStore struct {
	pm     *Manager
	plugin *v2.Plugin
}

func (s *pluginConfigStore) Put([]byte) (digest.Digest, error) {
	return digest.Digest(""), errors.New("cannot store config on push")
}

func (s *pluginConfigStore) Get(d digest.Digest) ([]byte, error) {
	if s.plugin.Config != d {
		return nil, errors.New("plugin not found")
	}
	rwc, err := s.pm.blobStore.Get(d)
	if err != nil {
		return nil, err
	}
	defer rwc.Close()
	return ioutil.ReadAll(rwc)
}

func (s *pluginConfigStore) RootFSFromConfig(c []byte) (*image.RootFS, error) {
	return configToRootFS(c)
}

type pluginLayerProvider struct {
	pm     *Manager
	plugin *v2.Plugin
}

func (p *pluginLayerProvider) Get(id layer.ChainID) (distribution.PushLayer, error) {
	rootFS := rootFSFromPlugin(p.plugin.PluginObj.Config.Rootfs)
	var i int
	for i = 1; i <= len(rootFS.DiffIDs); i++ {
		if layer.CreateChainID(rootFS.DiffIDs[:i]) == id {
			break
		}
	}
	if i > len(rootFS.DiffIDs) {
		return nil, errors.New("layer not found")
	}
	return &pluginLayer{
		pm:      p.pm,
		diffIDs: rootFS.DiffIDs[:i],
		blobs:   p.plugin.Blobsums[:i],
	}, nil
}

type pluginLayer struct {
	pm      *Manager
	diffIDs []layer.DiffID
	blobs   []digest.Digest
}

func (l *pluginLayer) ChainID() layer.ChainID {
	return layer.CreateChainID(l.diffIDs)
}

func (l *pluginLayer) DiffID() layer.DiffID {
	return l.diffIDs[len(l.diffIDs)-1]
}

func (l *pluginLayer) Parent() distribution.PushLayer {
	if len(l.diffIDs) == 1 {
		return nil
	}
	return &pluginLayer{
		pm:      l.pm,
		diffIDs: l.diffIDs[:len(l.diffIDs)-1],
		blobs:   l.blobs[:len(l.diffIDs)-1],
	}
}

func (l *pluginLayer) Open() (io.ReadCloser, error) {
	return l.pm.blobStore.Get(l.blobs[len(l.diffIDs)-1])
}

func (l *pluginLayer) Size() (int64, error) {
	return l.pm.blobStore.Size(l.blobs[len(l.diffIDs)-1])
}

func (l *pluginLayer) MediaType() string {
	return schema2.MediaTypeLayer
}

func (l *pluginLayer) Release() {
	// Nothing needs to be release, no references held
}

// Remove deletes plugin's root directory.
func (pm *Manager) Remove(name string, config *types.PluginRmConfig) error {
	p, err := pm.config.Store.GetV2Plugin(name)
	pm.mu.RLock()
	c := pm.cMap[p]
	pm.mu.RUnlock()

	if err != nil {
		return err
	}

	if !config.ForceRemove {
		if p.GetRefCount() > 0 {
			return fmt.Errorf("plugin %s is in use", p.Name())
		}
		if p.IsEnabled() {
			return fmt.Errorf("plugin %s is enabled", p.Name())
		}
	}

	if p.IsEnabled() {
		if err := pm.disable(p, c); err != nil {
			logrus.Errorf("failed to disable plugin '%s': %s", p.Name(), err)
		}
	}

	defer func() {
		go pm.GC()
	}()

	id := p.GetID()
	pm.config.Store.Remove(p)
	pluginDir := filepath.Join(pm.config.Root, id)
	if err := recursiveUnmount(pm.config.Root); err != nil {
		logrus.WithField("dir", pm.config.Root).WithField("id", id).Warn(err)
	}
	if err := os.RemoveAll(pluginDir); err != nil {
		logrus.Warnf("unable to remove %q from plugin remove: %v", pluginDir, err)
	}
	pm.config.LogPluginEvent(id, name, "remove")
	return nil
}

func getMounts(root string) ([]string, error) {
	infos, err := mount.GetMounts()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read mount table")
	}

	var mounts []string
	for _, m := range infos {
		if strings.HasPrefix(m.Mountpoint, root) {
			mounts = append(mounts, m.Mountpoint)
		}
	}

	return mounts, nil
}

func recursiveUnmount(root string) error {
	mounts, err := getMounts(root)
	if err != nil {
		return err
	}

	// sort in reverse-lexicographic order so the root mount will always be last
	sort.Sort(sort.Reverse(sort.StringSlice(mounts)))

	for i, m := range mounts {
		if err := mount.Unmount(m); err != nil {
			if i == len(mounts)-1 {
				return errors.Wrapf(err, "error performing recursive unmount on %s", root)
			}
			logrus.WithError(err).WithField("mountpoint", m).Warn("could not unmount")
		}
	}

	return nil
}

// Set sets plugin args
func (pm *Manager) Set(name string, args []string) error {
	p, err := pm.config.Store.GetV2Plugin(name)
	if err != nil {
		return err
	}
	if err := p.Set(args); err != nil {
		return err
	}
	return pm.save(p)
}

// CreateFromContext creates a plugin from the given pluginDir which contains
// both the rootfs and the config.json and a repoName with optional tag.
func (pm *Manager) CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *types.PluginCreateOptions) (err error) {
	pm.muGC.RLock()
	defer pm.muGC.RUnlock()

	ref, err := reference.ParseNormalizedNamed(options.RepoName)
	if err != nil {
		return errors.Wrapf(err, "failed to parse reference %v", options.RepoName)
	}
	if _, ok := ref.(reference.Canonical); ok {
		return errors.Errorf("canonical references are not permitted")
	}
	name := reference.FamiliarString(reference.TagNameOnly(ref))

	if err := pm.config.Store.validateName(name); err != nil { // fast check, real check is in createPlugin()
		return err
	}

	tmpRootFSDir, err := ioutil.TempDir(pm.tmpDir(), ".rootfs")
	if err != nil {
		return errors.Wrap(err, "failed to create temp directory")
	}
	defer os.RemoveAll(tmpRootFSDir)

	var configJSON []byte
	rootFS := splitConfigRootFSFromTar(tarCtx, &configJSON)

	rootFSBlob, err := pm.blobStore.New()
	if err != nil {
		return err
	}
	defer rootFSBlob.Close()
	gzw := gzip.NewWriter(rootFSBlob)
	layerDigester := digest.Canonical.Digester()
	rootFSReader := io.TeeReader(rootFS, io.MultiWriter(gzw, layerDigester.Hash()))

	if err := chrootarchive.Untar(rootFSReader, tmpRootFSDir, nil); err != nil {
		return err
	}
	if err := rootFS.Close(); err != nil {
		return err
	}

	if configJSON == nil {
		return errors.New("config not found")
	}

	if err := gzw.Close(); err != nil {
		return errors.Wrap(err, "error closing gzip writer")
	}

	var config types.PluginConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return errors.Wrap(err, "failed to parse config")
	}

	if err := pm.validateConfig(config); err != nil {
		return err
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	rootFSBlobsum, err := rootFSBlob.Commit()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			go pm.GC()
		}
	}()

	config.Rootfs = &types.PluginConfigRootfs{
		Type:    "layers",
		DiffIds: []string{layerDigester.Digest().String()},
	}

	config.DockerVersion = dockerversion.Version

	configBlob, err := pm.blobStore.New()
	if err != nil {
		return err
	}
	defer configBlob.Close()
	if err := json.NewEncoder(configBlob).Encode(config); err != nil {
		return errors.Wrap(err, "error encoding json config")
	}
	configBlobsum, err := configBlob.Commit()
	if err != nil {
		return err
	}

	p, err := pm.createPlugin(name, configBlobsum, []digest.Digest{rootFSBlobsum}, tmpRootFSDir, nil, nil)
	if err != nil {
		return err
	}
	p.PluginObj.PluginReference = name

	pm.config.LogPluginEvent(p.PluginObj.ID, name, "create")

	return nil
}

// SavePlugin saves the plugin in outStream
// The tar layout follows the v1.0 OCI image spec defined here:
// https://github.com/opencontainers/image-spec/blob/master/image-layout.md
func (pm *Manager) SavePlugin(plugin string, outStream io.Writer) error {
	srcP, err := pm.config.Store.GetV2Plugin(plugin)
	if err != nil {
		return err
	}

	pm.muGC.RLock()
	defer pm.muGC.RUnlock()

	ref, err := reference.ParseNormalizedNamed(plugin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse reference %v", plugin)
	}
	if _, ok := ref.(reference.Canonical); ok {
		return errors.Errorf("canonical references are not permitted")
	}

	// 1. create tempDir to save the tar
	tempDir, err := ioutil.TempDir("", "docker-plugin-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// 2. save config and rootfs into tmpdir
	manifest := ociNewManifest()

	for i, dgst := range append([]digest.Digest{srcP.Config}, srcP.Blobsums...) {
		rdr, err := pm.blobStore.Get(dgst)
		if err != nil {
			return err
		}
		defer rdr.Close()

		size, err := ociWriteBlob(tempDir, dgst, rdr)
		if err != nil {
			return err
		}

		if i == 0 {
			manifest.Config.Digest = dgst
			manifest.Config.Size = size
		} else {
			manifest.Layers = append(manifest.Layers, ocispecv1.Descriptor{
				Digest:    dgst,
				Size:      size,
				MediaType: ocispecv1.MediaTypeImageLayerGzip,
			})
		}
	}

	manifestBytes, err := json.Marshal(&manifest)
	if err != nil {
		return errors.Wrap(err, "error marshaling plugin manifest")
	}
	manifestDigest := digest.FromBytes(manifestBytes)
	manifestSize, err := ociWriteBlob(tempDir, manifestDigest, bytes.NewReader(manifestBytes))
	if err != nil {
		return err
	}

	if err := ociWriteLayout(tempDir); err != nil {
		return err
	}
	if err := ociWriteIndex(tempDir, []ocispecv1.Descriptor{
		// TODO(@cpuguy83): support platform field?
		// Haven't found where this is stored if at all yet
		{
			MediaType:   ocispecv1.MediaTypeImageManifest,
			Size:        manifestSize,
			Digest:      manifestDigest,
			Annotations: map[string]string{"com.docker.plugin.ref.name": srcP.PluginObj.Name},
		},
	}); err != nil {
		return err
	}

	// 4. create a tar archive of tempDir
	fs, err := archive.Tar(tempDir, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	// 5. stream it back to the outStream
	if _, err = io.Copy(outStream, fs); err != nil {
		return err
	}

	pm.config.LogPluginEvent(srcP.PluginObj.ID, plugin, "save")
	return nil
}

func safePath(base, path string) (string, error) {
	return symlink.FollowSymlinkInScope(filepath.Join(base, path), base)
}

// TODO: remove this, won't be used
type manifestItem struct {
	Name   string
	Layers []string
	Config string
}

// LoadPlugin loads a plugin from input tar archive
// The input archive *must* be in the OCI image spec format defined here:
// https://github.com/opencontainers/image-spec/blob/master/image-layout.md
func (pm *Manager) LoadPlugin(input io.ReadCloser, outStream io.Writer, quiet bool) (err error) {
	outStream = streamformatter.NewStdoutWriter(outStream)

	// 1. create the tempdir used to untar and then load
	tmpDir, err := ioutil.TempDir("", "docker-plugin-import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := chrootarchive.Untar(input, tmpDir, nil); err != nil {
		return err
	}

	f, err := os.Open(filepath.Join(tmpDir, "index.json"))
	if err != nil {
		return errors.Wrap(err, "error opening oci index")
	}
	defer f.Close()
	var index ocispecv1.Index

	if err := json.NewDecoder(f).Decode(&index); err != nil {
		return errors.Wrap(err, "error reading oci index")
	}

	for _, m := range index.Manifests {
		if m.MediaType != ocispecv1.MediaTypeImageManifest {
			continue
		}

		f, err := os.Open(filepath.Join(tmpDir, "blobs", m.Digest.Algorithm().String(), m.Digest.Hex()))
		if err != nil {
			return errors.Wrap(err, "error opening manifest")
		}

		var manifest ocispecv1.Manifest
		if err := json.NewDecoder(f).Decode(&manifest); err != nil {
			return errors.Wrap(err, "error reading manifest")
		}
		if manifest.Config.MediaType != schema2.MediaTypePluginConfig {
			continue
		}

		// TODO(@cpuguy83): verify digests?
		f, err = os.Open(filepath.Join(tmpDir, "blobs", manifest.Config.Digest.Algorithm().String(), manifest.Config.Digest.Hex()))
		if err != nil {
			return errors.Wrap(err, "error opening plugin config")
		}
		defer f.Close()

		configBlob, err := pm.blobStore.New()
		if err != nil {
			return err
		}
		defer configBlob.Close()
		r := io.TeeReader(f, configBlob)

		var config types.PluginConfig
		if err := json.NewDecoder(r).Decode(&config); err != nil {
			return errors.Wrap(err, "error reading plugin config")
		}
		if _, err := configBlob.Commit(); err != nil {
			return err
		}
		privileges, err := computePrivileges(config)
		if err != nil {
			return err
		}

		// create a tmpdir to extract the plugin rootfs
		tmpRootFS, err := ioutil.TempDir(tmpDir, ".rootfs")
		if err != nil {
			return errors.Wrap(err, "error creating rootfs dir")
		}
		defer os.RemoveAll(tmpRootFS)

		layerDigests := make([]digest.Digest, 0, len(manifest.Layers))
		for _, l := range manifest.Layers {
			if l.MediaType != ocispecv1.MediaTypeImageLayerGzip {
				continue
			}
			compressedTar, err := os.Open(filepath.Join(tmpDir, "blobs", l.Digest.Algorithm().String(), l.Digest.Hex()))
			if err != nil {
				return err
			}
			defer compressedTar.Close()
			// XXX: Instead of using the blobStore interface to create, write and commit
			// to blobstore, why not simply copy the files and rename them?
			rootfsBlob, err := pm.blobStore.New()
			if err != nil {
				return err
			}
			defer rootfsBlob.Close()

			size, err := io.Copy(rootfsBlob, compressedTar)
			if err != nil {
				return errors.Wrap(err, "error reading layer")
			}
			if size != l.Size {
				return errors.New("layer size mismatch")
			}

			if _, err := rootfsBlob.Commit(); err != nil {
				return err
			}

			// decompress for plugin rootfs
			if _, err := compressedTar.Seek(0, 0); err != nil {
				return errors.Wrap(err, "error seeking to beginning of layer stream")
			}
			decompressedTar, err := archive.DecompressStream(compressedTar)
			if err != nil {
				return err
			}
			if err = chrootarchive.Untar(decompressedTar, tmpRootFS, nil); err != nil {
				return err
			}
			layerDigests = append(layerDigests, l.Digest)
		}

		name := m.Annotations["com.docker.plugin.ref.name"]
		p, err := pm.createPlugin(name, manifest.Config.Digest, layerDigests, tmpRootFS, &privileges, &config)
		if err != nil {
			return err
		}

		p.PluginObj.PluginReference = name
		pm.config.LogPluginEvent(p.PluginObj.ID, name, "load")

		outStream.Write([]byte(fmt.Sprintf("Loaded plugin ID: %s\n", p.PluginObj.ID)))

		// call mountPropUpdate to finalize mount magic for volume plugins.
		mountPropUpdate(p)
	}

	return nil
}

func (pm *Manager) validateConfig(config types.PluginConfig) error {
	return nil // TODO:
}

func splitConfigRootFSFromTar(in io.ReadCloser, config *[]byte) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		tarReader := tar.NewReader(in)
		tarWriter := tar.NewWriter(pw)
		defer in.Close()

		hasRootFS := false

		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				if !hasRootFS {
					pw.CloseWithError(errors.Wrap(err, "no rootfs found"))
					return
				}
				// Signals end of archive.
				tarWriter.Close()
				pw.Close()
				return
			}
			if err != nil {
				pw.CloseWithError(errors.Wrap(err, "failed to read from tar"))
				return
			}

			content := io.Reader(tarReader)
			name := path.Clean(hdr.Name)
			if path.IsAbs(name) {
				name = name[1:]
			}
			if name == configFileName {
				dt, err := ioutil.ReadAll(content)
				if err != nil {
					pw.CloseWithError(errors.Wrapf(err, "failed to read %s", configFileName))
					return
				}
				*config = dt
			}
			if parts := strings.Split(name, "/"); len(parts) != 0 && parts[0] == rootFSFileName {
				hdr.Name = path.Clean(path.Join(parts[1:]...))
				if hdr.Typeflag == tar.TypeLink && strings.HasPrefix(strings.ToLower(hdr.Linkname), rootFSFileName+"/") {
					hdr.Linkname = hdr.Linkname[len(rootFSFileName)+1:]
				}
				if err := tarWriter.WriteHeader(hdr); err != nil {
					pw.CloseWithError(errors.Wrap(err, "error writing tar header"))
					return
				}
				if _, err := pools.Copy(tarWriter, content); err != nil {
					pw.CloseWithError(errors.Wrap(err, "error copying tar data"))
					return
				}
				hasRootFS = true
			} else {
				io.Copy(ioutil.Discard, content)
			}
		}
	}()
	return pr
}
