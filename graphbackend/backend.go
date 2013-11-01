package graphbackend

type Image interface {
	Layers() ([]string, error)
}

type GraphBackend interface {
	//	Create(img *Image) error
	//	Delete(img *Image) error
	Mount(img Image, root string) error
	Unmount(root string) error
	Mounted(root string) (bool, error)
	//	UnmountAll(img *Image) error
	//	Changes(img *Image, dest string) ([]Change, error)
	//	Layer(img *Image, dest string) (Archive, error)
}
