package v2

import (
	"github.com/docker/docker-registry/common"
	"github.com/gorilla/mux"
)

// The following are definitions of the name under which all V2 routes are
// registered. These symbols can be used to look up a route based on the name.
const (
	RouteNameBase            = "base"
	RouteNameManifest        = "manifest"
	RouteNameTags            = "tags"
	RouteNameBlob            = "blob"
	RouteNameBlobUpload      = "blob-upload"
	RouteNameBlobUploadChunk = "blob-upload-chunk"
)

var allEndpoints = []string{
	RouteNameManifest,
	RouteNameTags,
	RouteNameBlob,
	RouteNameBlobUpload,
	RouteNameBlobUploadChunk,
}

// Router builds a gorilla router with named routes for the various API
// methods. This can be used directly by both server implementations and
// clients.
func Router() *mux.Router {
	router := mux.NewRouter().
		StrictSlash(true)

	// GET /v2/	Check	Check that the registry implements API version 2(.1)
	router.
		Path("/v2/").
		Name(RouteNameBase)

	// GET      /v2/<name>/manifest/<tag>	Image Manifest	Fetch the image manifest identified by name and tag.
	// PUT      /v2/<name>/manifest/<tag>	Image Manifest	Upload the image manifest identified by name and tag.
	// DELETE   /v2/<name>/manifest/<tag>	Image Manifest	Delete the image identified by name and tag.
	router.
		Path("/v2/{name:" + common.RepositoryNameRegexp.String() + "}/manifests/{tag:" + common.TagNameRegexp.String() + "}").
		Name(RouteNameManifest)

	// GET	/v2/<name>/tags/list	Tags	Fetch the tags under the repository identified by name.
	router.
		Path("/v2/{name:" + common.RepositoryNameRegexp.String() + "}/tags/list").
		Name(RouteNameTags)

	// GET	/v2/<name>/blob/<digest>	Layer	Fetch the blob identified by digest.
	router.
		Path("/v2/{name:" + common.RepositoryNameRegexp.String() + "}/blobs/{digest:[a-zA-Z0-9-_+.]+:[a-zA-Z0-9-_+.=]+}").
		Name(RouteNameBlob)

	// POST	/v2/<name>/blob/upload/	Layer Upload	Initiate an upload of the layer identified by tarsum.
	router.
		Path("/v2/{name:" + common.RepositoryNameRegexp.String() + "}/blobs/uploads/").
		Name(RouteNameBlobUpload)

	// GET	/v2/<name>/blob/upload/<uuid>	Layer Upload	Get the status of the upload identified by tarsum and uuid.
	// PUT	/v2/<name>/blob/upload/<uuid>	Layer Upload	Upload all or a chunk of the upload identified by tarsum and uuid.
	// DELETE	/v2/<name>/blob/upload/<uuid>	Layer Upload	Cancel the upload identified by layer and uuid
	router.
		Path("/v2/{name:" + common.RepositoryNameRegexp.String() + "}/blobs/uploads/{uuid}").
		Name(RouteNameBlobUploadChunk)

	return router
}
