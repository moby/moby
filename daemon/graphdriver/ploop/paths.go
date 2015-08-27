package ploop

import "path"

const (
	ddxml = "DiskDescriptor.xml"
)

// Returns path to ploop image directory for given id
func (d *Driver) dir(id string) string {
	// Assuming that id doesn't contain "/" characters
	return path.Join(d.home, "img", id)
}

// Returns path to ploop's DiskDescriptor.xml for given id
func (d *Driver) dd(id string) string {
	return path.Join(d.dir(id), ddxml)
}

// Returns path to ploop's image for given id
func (d *Driver) img(id string) string {
	return path.Join(d.dir(id), imagePrefix)
}

// Returns a mount point for given id
func (d *Driver) mnt(id string) string {
	return path.Join(d.home, "mnt", id)
}
