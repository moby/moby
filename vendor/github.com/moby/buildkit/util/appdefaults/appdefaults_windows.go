package appdefaults

const (
	Address   = "npipe:////./pipe/buildkitd"
	Root      = ".buildstate"
	ConfigDir = ""
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
