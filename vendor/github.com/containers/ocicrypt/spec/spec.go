package spec

const (
	// MediaTypeLayerEnc is MIME type used for encrypted layers.
	MediaTypeLayerEnc = "application/vnd.oci.image.layer.v1.tar+encrypted"
	// MediaTypeLayerGzipEnc is MIME type used for encrypted compressed layers.
	MediaTypeLayerGzipEnc = "application/vnd.oci.image.layer.v1.tar+gzip+encrypted"
	// MediaTypeLayerNonDistributableEnc is MIME type used for non distributable encrypted layers.
	MediaTypeLayerNonDistributableEnc = "application/vnd.oci.image.layer.nondistributable.v1.tar+encrypted"
	// MediaTypeLayerGzipEnc is MIME type used for non distributable encrypted compressed layers.
	MediaTypeLayerNonDistributableGzipEnc = "application/vnd.oci.image.layer.nondistributable.v1.tar+gzip+encrypted"
)
