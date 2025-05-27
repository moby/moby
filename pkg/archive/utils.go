package archive

import (
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/go-archive"
)

// ToArchiveOpt converts an [TarOptions] to a [archive.TarOptions].
//
// Deprecated: use [archive.TarOptions] instead, this utility is for internal use to transition to the [github.com/moby/go-archive] module.
func ToArchiveOpt(options *TarOptions) *archive.TarOptions {
	return toArchiveOpt(options)
}

func toArchiveOpt(options *TarOptions) *archive.TarOptions {
	if options == nil {
		return nil
	}

	var chownOpts *archive.ChownOpts
	if options.ChownOpts != nil {
		chownOpts = &archive.ChownOpts{
			UID: options.ChownOpts.UID,
			GID: options.ChownOpts.GID,
		}
	}

	return &archive.TarOptions{
		IncludeFiles:         options.IncludeFiles,
		ExcludePatterns:      options.ExcludePatterns,
		Compression:          options.Compression,
		NoLchown:             options.NoLchown,
		IDMap:                idtools.ToUserIdentityMapping(options.IDMap),
		ChownOpts:            chownOpts,
		IncludeSourceDir:     options.IncludeSourceDir,
		WhiteoutFormat:       options.WhiteoutFormat,
		NoOverwriteDirNonDir: options.NoOverwriteDirNonDir,
		RebaseNames:          options.RebaseNames,
		InUserNS:             options.InUserNS,
		BestEffortXattrs:     options.BestEffortXattrs,
	}
}
