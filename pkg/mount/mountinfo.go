package mount

type MountInfo struct {
	Id, Parent, Major, Minor int
	Root, Mountpoint, Opts   string
	Fstype, Source, VfsOpts  string
}
