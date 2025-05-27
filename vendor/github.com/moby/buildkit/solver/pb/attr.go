package pb

const AttrKeepGitDir = "git.keepgitdir"
const AttrFullRemoteURL = "git.fullurl"
const AttrAuthHeaderSecret = "git.authheadersecret"
const AttrAuthTokenSecret = "git.authtokensecret"
const AttrKnownSSHHosts = "git.knownsshhosts"
const AttrMountSSHSock = "git.mountsshsock"
const AttrGitChecksum = "git.checksum"

const AttrLocalSessionID = "local.session"
const AttrLocalUniqueID = "local.unique"
const AttrIncludePatterns = "local.includepattern"
const AttrFollowPaths = "local.followpaths"
const AttrExcludePatterns = "local.excludepatterns"
const AttrSharedKeyHint = "local.sharedkeyhint"
const AttrMetadataTransfer = "local.metadatatransfer"
const AttrMetadataTransferExclude = "local.metadatatransferexclude"

const AttrLLBDefinitionFilename = "llbbuild.filename"

const AttrHTTPChecksum = "http.checksum"
const AttrHTTPFilename = "http.filename"
const AttrHTTPPerm = "http.perm"
const AttrHTTPUID = "http.uid"
const AttrHTTPGID = "http.gid"
const AttrHTTPAuthHeaderSecret = "http.authheadersecret"
const AttrHTTPHeaderPrefix = "http.header."

const AttrImageResolveMode = "image.resolvemode"
const AttrImageResolveModeDefault = "default"
const AttrImageResolveModeForcePull = "pull"
const AttrImageResolveModePreferLocal = "local"
const AttrImageRecordType = "image.recordtype"
const AttrImageLayerLimit = "image.layerlimit"

const AttrOCILayoutSessionID = "oci.session"
const AttrOCILayoutStoreID = "oci.store"
const AttrOCILayoutLayerLimit = "oci.layerlimit"

const AttrLocalDiffer = "local.differ"
const AttrLocalDifferNone = "none"
const AttrLocalDifferMetadata = "metadata"

type IsFileAction = isFileAction_Action
