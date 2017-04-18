package daemon

import (
	"strconv"

	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/idtools"
	"github.com/opencontainers/go-digest"
	"github.com/stevvooe/continuity"
)

// checksumPath computes a checksum on a filesystem path.
func checksumPath(root string, mask masker) (digest.Digest, error) {
	ctx, err := continuity.NewContext(root)
	if err != nil {
		return "", err
	}

	manifest, err := continuity.BuildManifest(ctx)
	if err != nil {
		return "", err
	}

	// Omit unwanted fields by wrapping the resource.
	for i, resource := range manifest.Resources {
		if _, ok := resource.(continuity.XAttrer); ok {
			manifest.Resources[i] = mask.maskResourceWithXAttrs(resource)
		} else {
			manifest.Resources[i] = mask.maskResource(resource)
		}
	}

	manifestBytes, err := continuity.Marshal(manifest)
	if err != nil {
		return "", err
	}

	return digest.FromBytes(manifestBytes), nil
}

// checksumFilesystem computes an overall digest of the layer identified by chainID.
func (daemon *Daemon) checksumFilesystem(containerName string, chainID layer.ChainID) (digest.Digest, error) {
	rwLayer, err := daemon.layerStore.CreateRWLayer(containerName+"-verify", chainID, nil)
	if err != nil {
		return "", err
	}
	defer daemon.layerStore.ReleaseRWLayer(rwLayer)

	rwPath, err := rwLayer.Mount("")
	if err != nil {
		return "", err
	}
	defer rwLayer.Unmount()

	uidMaps, gidMaps := daemon.GetUIDGIDMaps()
	mask := masker{uidMaps, gidMaps}

	checksum, err := checksumPath(rwPath, mask)
	if err != nil {
		return "", err
	}

	return checksum, nil
}

// masker processes resources for consistent representation across systems.
// Multiple xattrs are serialized for sorted ordering.
// With user namespaces, uids and gids are mapped to the container's view.
// For example, if root is remapped to 100000, masker converts uids/gids
// 100000 -> 0, 100042 -> 42, etc.
type masker struct {
	uidMaps, gidMaps []idtools.IDMap
}

func maskID(idStr string, idMaps []idtools.IDMap) string {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return idStr
	}
	mappedId, err := idtools.ToContainer(id, idMaps)
	if err != nil {
		return idStr
	}
	return strconv.Itoa(mappedId)
}

func (m masker) maskUID(uid string) string {
	return maskID(uid, m.uidMaps)
}

func (m masker) maskGID(gid string) string {
	return maskID(gid, m.gidMaps)
}

func (m masker) maskResource(r continuity.Resource) maskedResource {
	return maskedResource{r, m.maskUID(r.UID()), m.maskGID(r.GID())}
}

func (m masker) maskResourceWithXAttrs(r continuity.Resource) maskedResourceWithXAttrs {
	maskedXattrs := make(map[string][]byte)
	if xattrer, ok := r.(continuity.XAttrer); ok {
		maskedXattrs = xattrer.XAttrs()
	}

	return maskedResourceWithXAttrs{r, m.maskUID(r.UID()), m.maskGID(r.GID()), maskedXattrs}
}

// maskedResource emits stored UID and GID instead of the underlying resource's values.
type maskedResource struct {
	continuity.Resource
	uid string
	gid string
}

func (m maskedResource) UID() string {
	return m.uid
}

func (m maskedResource) GID() string {
	return m.gid
}

// maskedResourceWithXAttrs emits stored UID, GID, and xattrs instead of the underlying resource's values.
type maskedResourceWithXAttrs struct {
	continuity.Resource
	uid    string
	gid    string
	xattrs map[string][]byte
}

func (m maskedResourceWithXAttrs) UID() string {
	return m.uid
}

func (m maskedResourceWithXAttrs) GID() string {
	return m.gid
}

func (m maskedResourceWithXAttrs) XAttrs() map[string][]byte {
	return m.xattrs
}
