// +build experimental

package storage

import (
	"errors"
	"fmt"

	"github.com/docker/docker/pkg/archive"
)

type storageDriverProxy struct {
	name   string
	client pluginClient
}

type storageDriverRequest struct {
	ID         string `json:",omitempty"`
	Parent     string `json:",omitempty"`
	MountLabel string `json:",omitempty"`
}

type storageDriverResponse struct {
	Err      string            `json:",omitempty"`
	Dir      string            `json:",omitempty"`
	Exists   bool              `json:",omitempty"`
	Status   [][2]string       `json:",omitempty"`
	Changes  []archive.Change  `json:",omitempty"`
	Size     int64             `json:",omitempty"`
	Metadata map[string]string `json:",omitempty"`
}

type storageDriverInitRequest struct {
	Home string
	Opts []string
}

func (d *storageDriverProxy) Init(home string, opts []string) error {
	args := &storageDriverInitRequest{
		Home: home,
		Opts: opts,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Init", args, &ret); err != nil {
		return err
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) String() string {
	return d.name
}

func (d *storageDriverProxy) CreateReadWrite(id, parent, mountLabel string, storageOpt map[string]string) error {
	args := &storageDriverRequest{
		ID:         id,
		Parent:     parent,
		MountLabel: mountLabel,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.CreateReadWrite", args, &ret); err != nil {
		return err
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) Create(id, parent, mountLabel string, storageOpt map[string]string) error {
	args := &storageDriverRequest{
		ID:         id,
		Parent:     parent,
		MountLabel: mountLabel,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Create", args, &ret); err != nil {
		return err
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) Remove(id string) error {
	args := &storageDriverRequest{ID: id}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Remove", args, &ret); err != nil {
		return err
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) Get(id, mountLabel string) (string, error) {
	args := &storageDriverRequest{
		ID:         id,
		MountLabel: mountLabel,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Get", args, &ret); err != nil {
		return "", err
	}
	var err error
	if ret.Err != "" {
		err = errors.New(ret.Err)
	}
	return ret.Dir, err
}

func (d *storageDriverProxy) Put(id string) error {
	args := &storageDriverRequest{ID: id}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Put", args, &ret); err != nil {
		return err
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) Exists(id string) bool {
	args := &storageDriverRequest{ID: id}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Exists", args, &ret); err != nil {
		return false
	}
	return ret.Exists
}

func (d *storageDriverProxy) Status() [][2]string {
	args := &storageDriverRequest{}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Status", args, &ret); err != nil {
		return nil
	}
	return ret.Status
}

func (d *storageDriverProxy) GetMetadata(id string) (map[string]string, error) {
	args := &storageDriverRequest{
		ID: id,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.GetMetadata", args, &ret); err != nil {
		return nil, err
	}
	if ret.Err != "" {
		return nil, errors.New(ret.Err)
	}
	return ret.Metadata, nil
}

func (d *storageDriverProxy) Cleanup() error {
	args := &storageDriverRequest{}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Cleanup", args, &ret); err != nil {
		return nil
	}
	if ret.Err != "" {
		return errors.New(ret.Err)
	}
	return nil
}

func (d *storageDriverProxy) Diff(id, parent string) (archive.Archive, error) {
	args := &storageDriverRequest{
		ID:     id,
		Parent: parent,
	}
	body, err := d.client.Stream("StorageDriver.Diff", args)
	if err != nil {
		return nil, err
	}
	return archive.Archive(body), nil
}

func (d *storageDriverProxy) Changes(id, parent string) ([]archive.Change, error) {
	args := &storageDriverRequest{
		ID:     id,
		Parent: parent,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.Changes", args, &ret); err != nil {
		return nil, err
	}
	if ret.Err != "" {
		return nil, errors.New(ret.Err)
	}

	return ret.Changes, nil
}

func (d *storageDriverProxy) ApplyDiff(id, parent string, diff archive.Reader) (int64, error) {
	var ret storageDriverResponse
	if err := d.client.SendFile(fmt.Sprintf("StorageDriver.ApplyDiff?id=%s&parent=%s", id, parent), diff, &ret); err != nil {
		return -1, err
	}
	if ret.Err != "" {
		return -1, errors.New(ret.Err)
	}
	return ret.Size, nil
}

func (d *storageDriverProxy) DiffSize(id, parent string) (int64, error) {
	args := &storageDriverRequest{
		ID:     id,
		Parent: parent,
	}
	var ret storageDriverResponse
	if err := d.client.Call("StorageDriver.DiffSize", args, &ret); err != nil {
		return -1, err
	}
	if ret.Err != "" {
		return -1, errors.New(ret.Err)
	}
	return ret.Size, nil
}
