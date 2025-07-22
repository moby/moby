package auxprogress

import "github.com/moby/moby/api/types/auxprogress"

// ManifestPushedInsteadOfIndex is a note that is sent when a manifest is pushed
// instead of an index.  It is sent when the pushed image is an multi-platform
// index, but the whole index couldn't be pushed.
type ManifestPushedInsteadOfIndex = auxprogress.ManifestPushedInsteadOfIndex

// ContentMissing is a note that is sent when push fails because the content is missing.
type ContentMissing = auxprogress.ContentMissing
