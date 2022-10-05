package fsutils // import "github.com/docker/docker/pkg/fsutils"

import "github.com/containerd/continuity/fs"

// SupportsDType returns whether the filesystem mounted on path supports d_type.
//
// Deprecated: use github.com/containerd/continuity/fs.SupportsDType
var SupportsDType = fs.SupportsDType
