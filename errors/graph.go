package errors

// This file contains all of the errors that can be generated from the
// docker/graph component.

import (
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
)

var (
	// ErrorCodeNoSuchImage is generated when we can't find the image.
	ErrorCodeNoSuchImage = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGE",
		Message:        "could not find image: %v",
		Description:    "The specified image can not be found",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeImageIDConflict is generated when the expected id does not
	// match the id of the loaded image.
	ErrorCodeImageIDConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEIDCONFLICT",
		Message:        "Image stored at '%s' has wrong id '%s'",
		Description:    "The id of the loaded image does not match the expected id",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeImageSizingConflict is generated when the image size can not
	// be determined.
	ErrorCodeImageSizingConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGESIZINGCONFLICT",
		Message:        "unable to calculate size of image id %q: %s",
		Description:    "The size of the image could not be calculated",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeMakeTempFailure is generated when the attempt to create a temp
	// directory fails.
	ErrorCodeMakeTempFailure = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MAKETEMPFAILURE",
		Message:        "mktemp failed: %s",
		Description:    "Creating a specified temp directory has failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeDriverRootFSCreateFailure is generated when the attempt to create
	// the root file system fails in a driver.
	ErrorCodeDriverRootFSCreateFailure = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "DRIVERROOTFSCREATEFAILURE",
		Message:        "Driver %s failed to create image rootfs %s: %s",
		Description:    "The root file system could not be created for a driver",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadImageSizeWriteOP is generated when the attempt to write the
	// image size to the graph image directory root fails.
	ErrorCodeBadImageSizeWriteOP = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADIMAGESIZEWRITEOP",
		Message:        "Error storing image size in %s/%s: %s",
		Description:    "Failure writing image size to graph image directory",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeBadDigestWriteOP is generated when the attempt to write the
	// digest for the image has failed.
	ErrorCodeBadDigestWriteOP = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "BADDIGESTWRITEOP",
		Message:        "Error storing digest in %s/%s: %s",
		Description:    "Failure writing digest for an image",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeReadJSONImage is generated when the attempt to read in the
	// JSON for the image has failed
	ErrorCodeReadJSONImage = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "READJSONIMAGE",
		Message:        "Failed to read json for image %s: %s",
		Description:    "JSON for image could not be created",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGraphTarLayer is generated when the attempt to create a
	// gzip.NewReader for the TarLayer has failed.
	ErrorCodeGraphTarLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GRAPHTARLAYER",
		Message:        "[graph] error with %s:  %s",
		Description:    "Creating gzip.NewReader for TarLayer failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGraphGetParent is generated when the attempt to get a parent
	// has failed.
	ErrorCodeGraphGetParent = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GRAPHGETPARENT",
		Message:        "Error while getting parent image: %v",
		Description:    "The parent of a specified image was not found",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeGraphParentDepthLimit is generated when the depth of an image,
	// is too large to support creating a container from it on a daemon.
	ErrorCodeGraphParentDepthLimit = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GRAPHPARENTDEPTHLIMIT",
		Message:        "Cannot create container with more than %d parents",
		Description:    "The depth check for the number of allowed parents was exceeded on a daemon",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeLoadNotSupported is generated when the load api
	// has been called on a platform that does not support it.
	ErrorCodeLoadNotSupported = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "LOADNOTSUPPORTED",
		Message:        "Load is not supported on this platform",
		Description:    "Load is not supported on the platform",
		HTTPStatusCode: http.StatusNotImplemented,
	})

	// ErrorCodeRegistryVersion is generated when the version of the registry
	// does not match the supported list of registries.
	ErrorCodeRegistryVersion = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REGISTRYVERSION",
		Message:        "unknown version %d for registry %s",
		Description:    "The version of the registry is unknown",
		HTTPStatusCode: http.StatusNotImplemented,
	})

	// ErrorCodeImageEndpoints is generated when attempts to pull an image does
	// not return endpoints for the image.
	ErrorCodeImageEndpoints = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEENDPOINTS",
		Message:        "no endpoints found for %s",
		Description:    "The image has no endpoints",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeImageNotFoundInV1Repository is generated when the http 404 status
	// not found is returned on the get request.
	ErrorCodeImageNotFoundInV1Repository = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGENOTFOUNDINV1REPOSITORY",
		Message:        "Error: image %s not found",
		Description:    "The specified image was not found in the repository",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeTagNotFoundInV1Repository is generated when tag is not found
	// in the repository.
	ErrorCodeTagNotFoundInV1Repository = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TAGNOTFOUNDINV1REPOSITORY",
		Message:        "Tag %s not found in repository %s",
		Description:    "The specified tag was not found in the repository",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeImagePullErrorFromV1Repository is generated when an error occurs
	// while pulling the image.
	ErrorCodeImagePullErrorFromV1Repository = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEPULLERRORFROMV1REPOSITORY",
		Message:        "Error pulling image (%s) from %s, %v",
		Description:    "The pull of the specified image failed with the specified error",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeParsingImageJSON is generated when a failure occurs while parsing
	// the JSON for a pulled image.
	ErrorCodeParsingImageJSON = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "PARSINGIMAGEJSON",
		Message:        "Failed to parse json: %s",
		Description:    "The JSON could not be parsed for the specified image",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeWriteDigestToFSFailed is generated when a failure occurs verifying
	// the writing of the digest to the FS.
	ErrorCodeWriteDigestToFSFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WRITEDIGESTTOFSFAILED",
		Message:        "filesystem layer verification failed for digest %s",
		Description:    "The digest could not be written to the file system",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeMarshalPublicKey is generated when a failure occurs
	// marshalling the JSON public key.
	ErrorCodeMarshalPublicKey = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MARSHALPUBLICKEY",
		Message:        "error marshalling public key: %s",
		Description:    "Marshalling of the specified public has failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeKeyCheck is generated when a failure occurs checking a key for
	// read/write permission.
	ErrorCodeKeyCheck = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "KEYCHECK",
		Message:        "error running key check: %s",
		Description:    "The key does not have the required read/write permission",
		HTTPStatusCode: http.StatusForbidden,
	})

	// ErrorCodeWriteImageToFSFailed is generated when a failure occurs verifying
	// the writing of the image to the FS.
	ErrorCodeWriteImageToFSFailed = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "WRITEIMAGETOFSFAILED",
		Message:        "image verification failed for digest %s",
		Description:    "The writing of the image to the file system and subsequent digest verification has failed",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoManifest is generated when the manifest does not exist for a
	// given tag.
	ErrorCodeNoManifest = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOMANIFEST",
		Message:        "image manifest does not exist for tag %q",
		Description:    "The manifest for the specified tag does was not found",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeManifestWithUnknownSchema is generated when the schema is not
	// recognized.
	ErrorCodeManifestWithUnknownSchema = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MANIFESTWITHUNKNOWNSCHEMA",
		Message:        "unsupported schema version %d for tag %q",
		Description:    "The schema version is unknown",
		HTTPStatusCode: http.StatusNotImplemented,
	})

	// ErrorCodeManifestHistoryConflict is generated when the number of FS layers
	// is not equal to the expected number of layers for a tag.
	ErrorCodeManifestHistoryConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MANIFESTHISOTRYCONFLICT",
		Message:        "length of history not equal to number of layers for tag %q",
		Description:    "The number of file system layers is not equal to the expected number of layers for the specified tag",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeEmptyManifestHistory is generated when the length of the number of
	// FS layers for a tag is zero.
	ErrorCodeEmptyManifestHistory = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "EMPTYMANIFESTHISTORY",
		Message:        "no FSLayers in manifest for tag %q",
		Description:    "The number of file system layers for the specified tag is zero",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeFromManifestVerify is generated to ouput errors returned from
	// manifest.Verify.
	ErrorCodeFromManifestVerify = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "FROMMANIFESTVERIFY",
		Message:        "error verifying manifest for tag %q: %v",
		Description:    "The specified error has been returned from manifest.Verify for the specified tag",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeManifestKeys is generated when the verification of the keys in the
	// has failed.
	ErrorCodeManifestKeys = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "MANIFESTKEYS",
		Message:        "error verifying manifest keys: %v",
		Description:    "The verification for the keys of the manifest returned the specified error",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeUnknownRegistry is generated when the version of the registry
	// is not known.
	ErrorCodeUnknownRegistry = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNKNOWNREGISTRY",
		Message:        "unknown version %d for registry %s",
		Description:    "The specified version of the specified registry is unknown",
		HTTPStatusCode: http.StatusNotImplemented,
	})

	// ErrorCodeNoRepository is generated when the Repository being used
	// does not exist.
	ErrorCodeNoRepository = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOREPOSITORY",
		Message:        "Repository does not exist: %s",
		Description:    "The specified repository was not found",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeNoEndpointsForRepository is generated when the Repository being
	// used does not have endpoints.
	ErrorCodeNoEndpointsForRepository = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOENDPOINTSFORREPOSITORY",
		Message:        "no endpoints found for %s",
		Description:    "The specified repository has not endpoints",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoImagesFound is generated when no images are present for a
	// tag being pushed to a Repository.
	ErrorCodeNoImagesFound = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOIMAGESFOUND",
		Message:        "No images found for the requested repository / tag",
		Description:    "No images are present for a tag that is being pushed to the repository",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRegistryBusy is generated when either a push or a pull
	// is already in progress when a request to push is made for a registry.
	ErrorCodeRegistryBusy = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REGISTRYBUSY",
		Message:        "push or pull %s is already in progress",
		Description:    "A push or pull for the specified registry is already in progress while processing this request to push",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeImageAtPath is generated when the generation of a JSON
	// object fails for an image at a path.
	ErrorCodeImageAtPath = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEATPATH",
		Message:        "Cannot retrieve the path for {%s}: %s",
		Description:    "The generation of a JSON object has failed for the specified image at the specified path",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeGenerateLayer is generated when the generation of the layer
	// archive fails.
	ErrorCodeGenerateLayer = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GENERATELAYER",
		Message:        "Failed to generate layer archive: %s",
		Description:    "The specified error resulted from attempting to generate a layer archive",
		HTTPStatusCode: http.StatusInternalServerError,
	})

	// ErrorCodeNoSuchTag is generated when a requested tag does not exist
	// in a respository.
	ErrorCodeNoSuchTag = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHTAG",
		Message:        "Tag does not exist for %s",
		Description:    "The tag was not found in the specified registry",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeGettingTagsForImage is generated when a problem occurs identifying
	// tags for an image.
	ErrorCodeGettingTagsForImage = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "GETTINGTAGSFORIMAGE",
		Message:        "error getting tags for %s: %s",
		Description:    "The specified error occured while identifying tags for the specified image",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoTagsInPush is generated for a push to a repository that has
	// no tags.
	ErrorCodeNoTagsInPush = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOTAGSINPUSH",
		Message:        "no tags to push for %s",
		Description:    "The push to the specified registry has no tags",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoSuchTagV2Repos is generated when a requested tag does not exist
	// in a respository.
	ErrorCodeNoSuchTagV2Repos = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHTAGV2REPOS",
		Message:        "tag does not exist: %s",
		Description:    "The specified tag does not exist in the repository",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeImageAtPathV2Repos is generated when the generation of a JSON
	// object fails for an image at a path.
	ErrorCodeImageAtPathV2Repos = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEATPATHV2REPOS",
		Message:        "cannot retrieve the path for %s: %s",
		Description:    "The generation of the JSON object for the specified image failed with the specified error",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeImageDigest is generated when an error occurs in getting the
	// digest for an image.
	ErrorCodeImageDigest = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "IMAGEDIGEST",
		Message:        "error getting image checksum: %v",
		Description:    "The specified error occured while retrieving the digest for an image",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeNoSuchImageInTagStore is generated when the image being searched
	// does not exist in a given TagStore.
	ErrorCodeNoSuchImageInTagStore = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHIMAGEINTAGSTORE",
		Message:        "No such image %s",
		Description:    "The specified image does not exist in the given TagStore",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeNoSuchRepositoryInTagStore is generated when the repository does
	// not exist in a given TagStore.
	ErrorCodeNoSuchRepositoryInTagStore = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "NOSUCHREPOSITORYINTAGSTORE",
		Message:        "No such repository: %s",
		Description:    "The specified repository does not exist in the given TagStore",
		HTTPStatusCode: http.StatusNotFound,
	})

	// ErrorCodeStoreImageConflict is generated when an image is already present
	// in a store and the parameter -f for replacing the image not used.
	ErrorCodeStoreImageConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STOREIMAGECONFLICT",
		Message:        "Conflict: Tag %s is already set to image %s, if you want to replace it, please use -f option",
		Description:    "The specified tag is already set for the specified image and the force replace option was not used",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeStoreDigestConflict is generated when a digest is already present
	// in a store for an image.
	ErrorCodeStoreDigestConflict = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "STOREDIGESTCONFLICT",
		Message:        "Conflict: Digest %s is already set to image %s",
		Description:    "The specified digest has already been set to the specified image",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRepositoryNameIsEmpty is generated when a repository name is
	// empty.
	ErrorCodeRepositoryNameIsEmpty = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REPOSITORYNAMEISEMPTY",
		Message:        "Repository name can't be empty",
		Description:    "An attempt to use a repository name that is empty",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeRepositoryNameConflictWithScratch is generated on an attempt to
	// use `scratch` as a repository name.
	ErrorCodeRepositoryNameConflictWithScratch = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "REPOSITORYNAMECONFLICTWITHSCRATCH",
		Message:        "'scratch' is a reserved name",
		Description:    "The name `scratch` is a reserved repository name",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeUnknownPoolType is generated when the type of pool being removed
	// is invalid.
	ErrorCodeUnknownPoolType = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "UNKNOWNPOOLTYPE",
		Message:        "Unknown pool type",
		Description:    "The pool type being removed is of an unknown pool type",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeTagNameIsEmpty is generated when a tag name is
	// empty.
	ErrorCodeTagNameIsEmpty = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TAGNAMEISEMPTY",
		Message:        "tag name can't be empty",
		Description:    "The tag name being used is empty",
		HTTPStatusCode: http.StatusConflict,
	})

	// ErrorCodeTagNameFormat is generated when the format of a tag name is
	// invalid.
	ErrorCodeTagNameFormat = errcode.Register(errGroup, errcode.ErrorDescriptor{
		Value:          "TAGNAMEFORMAT",
		Message:        "Illegal tag name (%s): only [A-Za-z0-9_.-] are allowed ('.' and '-' are NOT allowed in the initial), minimum 1, maximum 128 in length",
		Description:    "The format of the specified tag name is invalid",
		HTTPStatusCode: http.StatusConflict,
	})
)
