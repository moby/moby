package appdefaults

const (
	Address = "npipe:////./pipe/buildkitd"
	Root    = ".buildstate"
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
