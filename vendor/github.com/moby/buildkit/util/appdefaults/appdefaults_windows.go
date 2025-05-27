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
	defaultContainerdDir = filepath.Join(os.Getenv("ProgramFiles"), "containerd")
	DefaultCNIBinDir     = filepath.Join(defaultContainerdDir, "cni", "bin")
	DefaultCNIConfigPath = filepath.Join(defaultContainerdDir, "cni", "conf", "0-containerd-nat.conf")
)

var (
	UserCNIConfigPath = DefaultCNIConfigPath
	CDISpecDirs       = []string{filepath.Join(os.Getenv("ProgramData"), "buildkitd", "cdi")}
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

func TraceSocketPath(inUserNS bool) string {
	return `\\.\pipe\buildkit-otel-grpc`
}
