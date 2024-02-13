package appdefaults

import (
	"os"
	"path/filepath"
)

const (
	Address = "npipe:////./pipe/buildkitd"
)

var (
	Root                 = filepath.Join(os.Getenv("ProgramData"), "buildkitd", ".buildstate")
	ConfigDir            = filepath.Join(os.Getenv("ProgramData"), "buildkitd")
	DefaultCNIBinDir     = filepath.Join(ConfigDir, "bin")
	DefaultCNIConfigPath = filepath.Join(ConfigDir, "cni.json")
)

func UserAddress() string {
	return Address
}

func EnsureUserAddressDir() error {
	return nil
}

func UserRoot() string {
	return Root
}

func UserConfigDir() string {
	return ConfigDir
}
