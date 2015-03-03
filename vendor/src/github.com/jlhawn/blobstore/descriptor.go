package blobstore

type blobInfo struct {
	Digest     string   `json:"digest"`
	MediaType  string   `json:"mediaType"`
	Size       uint64   `json:"size"`
	References []string `json:"references"`
}

type descriptor struct {
	blobInfo
}

func newDescriptor(info blobInfo) Descriptor {
	return &descriptor{blobInfo: info}
}

func (d *descriptor) Digest() string {
	return d.blobInfo.Digest
}

func (d *descriptor) MediaType() string {
	return d.blobInfo.MediaType
}

func (d *descriptor) Size() uint64 {
	return d.blobInfo.Size
}

func (d *descriptor) References() []string {
	return d.blobInfo.References
}
