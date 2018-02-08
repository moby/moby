package cgroups

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewNetCls(root string) *netclsController {
	return &netclsController{
		root: filepath.Join(root, string(NetCLS)),
	}
}

type netclsController struct {
	root string
}

func (n *netclsController) Name() Name {
	return NetCLS
}

func (n *netclsController) Path(path string) string {
	return filepath.Join(n.root, path)
}

func (n *netclsController) Create(path string, resources *specs.LinuxResources) error {
	if err := os.MkdirAll(n.Path(path), defaultDirPerm); err != nil {
		return err
	}
	if resources.Network != nil && resources.Network.ClassID != nil && *resources.Network.ClassID > 0 {
		return ioutil.WriteFile(
			filepath.Join(n.Path(path), "net_cls.classid"),
			[]byte(strconv.FormatUint(uint64(*resources.Network.ClassID), 10)),
			defaultFilePerm,
		)
	}
	return nil
}
