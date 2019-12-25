// generated code - DO NOT EDIT

package drivers // import "github.com/docker/docker/volume/drivers"

import (
	"errors"
	"time"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/volume"
)

const (
	longTimeout  = 2 * time.Minute
	shortTimeout = 1 * time.Minute
)

type client interface {
	CallWithOptions(string, interface{}, interface{}, ...func(*plugins.RequestOpts)) error
}

type volumeDriverProxy struct {
	client
}

type volumeDriverProxyCreateRequest struct {
	Name string
	Opts map[string]string
}

type volumeDriverProxyCreateResponse struct {
	Err string
}

func (pp *volumeDriverProxy) Create(name string, opts map[string]string) (err error) {
	var (
		req volumeDriverProxyCreateRequest
		ret volumeDriverProxyCreateResponse
	)

	req.Name = name
	req.Opts = opts

	if err = pp.CallWithOptions("VolumeDriver.Create", req, &ret, plugins.WithRequestTimeout(longTimeout)); err != nil {
		return
	}

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyRemoveRequest struct {
	Name string
}

type volumeDriverProxyRemoveResponse struct {
	Err string
}

func (pp *volumeDriverProxy) Remove(name string) (err error) {
	var (
		req volumeDriverProxyRemoveRequest
		ret volumeDriverProxyRemoveResponse
	)

	req.Name = name

	if err = pp.CallWithOptions("VolumeDriver.Remove", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyPathRequest struct {
	Name string
}

type volumeDriverProxyPathResponse struct {
	Mountpoint string
	Err        string
}

func (pp *volumeDriverProxy) Path(name string) (mountpoint string, err error) {
	var (
		req volumeDriverProxyPathRequest
		ret volumeDriverProxyPathResponse
	)

	req.Name = name

	if err = pp.CallWithOptions("VolumeDriver.Path", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	mountpoint = ret.Mountpoint

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyMountRequest struct {
	Name string
	ID   string
}

type volumeDriverProxyMountResponse struct {
	Mountpoint string
	Err        string
}

func (pp *volumeDriverProxy) Mount(name string, id string) (mountpoint string, err error) {
	var (
		req volumeDriverProxyMountRequest
		ret volumeDriverProxyMountResponse
	)

	req.Name = name
	req.ID = id

	if err = pp.CallWithOptions("VolumeDriver.Mount", req, &ret, plugins.WithRequestTimeout(longTimeout)); err != nil {
		return
	}

	mountpoint = ret.Mountpoint

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyUnmountRequest struct {
	Name string
	ID   string
}

type volumeDriverProxyUnmountResponse struct {
	Err string
}

func (pp *volumeDriverProxy) Unmount(name string, id string) (err error) {
	var (
		req volumeDriverProxyUnmountRequest
		ret volumeDriverProxyUnmountResponse
	)

	req.Name = name
	req.ID = id

	if err = pp.CallWithOptions("VolumeDriver.Unmount", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyListRequest struct {
}

type volumeDriverProxyListResponse struct {
	Volumes []*proxyVolume
	Err     string
}

func (pp *volumeDriverProxy) List() (volumes []*proxyVolume, err error) {
	var (
		req volumeDriverProxyListRequest
		ret volumeDriverProxyListResponse
	)

	if err = pp.CallWithOptions("VolumeDriver.List", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	volumes = ret.Volumes

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyGetRequest struct {
	Name string
}

type volumeDriverProxyGetResponse struct {
	Volume *proxyVolume
	Err    string
}

func (pp *volumeDriverProxy) Get(name string) (volume *proxyVolume, err error) {
	var (
		req volumeDriverProxyGetRequest
		ret volumeDriverProxyGetResponse
	)

	req.Name = name

	if err = pp.CallWithOptions("VolumeDriver.Get", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	volume = ret.Volume

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}

type volumeDriverProxyCapabilitiesRequest struct {
}

type volumeDriverProxyCapabilitiesResponse struct {
	Capabilities volume.Capability
	Err          string
}

func (pp *volumeDriverProxy) Capabilities() (capabilities volume.Capability, err error) {
	var (
		req volumeDriverProxyCapabilitiesRequest
		ret volumeDriverProxyCapabilitiesResponse
	)

	if err = pp.CallWithOptions("VolumeDriver.Capabilities", req, &ret, plugins.WithRequestTimeout(shortTimeout)); err != nil {
		return
	}

	capabilities = ret.Capabilities

	if ret.Err != "" {
		err = errors.New(ret.Err)
	}

	return
}
