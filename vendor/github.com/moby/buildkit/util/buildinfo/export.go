package buildinfo

import "github.com/pkg/errors"

// ExportMode represents the export mode for buildinfo opt.
type ExportMode int

const (
	// ExportNone doesn't export build dependencies.
	ExportNone ExportMode = 0
	// ExportImageConfig exports build dependencies to
	// the image config.
	ExportImageConfig = 1 << iota
	// ExportMetadata exports build dependencies as metadata to
	// the exporter response.
	ExportMetadata
	// ExportAll exports build dependencies as metadata and
	// image config.
	ExportAll = -1
)

// ExportDefault is the default export mode for buildinfo opt.
const ExportDefault = ExportAll

// ParseExportMode returns the export mode matching a string.
func ParseExportMode(s string) (ExportMode, error) {
	switch s {
	case "none":
		return ExportNone, nil
	case "imageconfig":
		return ExportImageConfig, nil
	case "metadata":
		return ExportMetadata, nil
	case "all":
		return ExportAll, nil
	default:
		return 0, errors.Errorf("unknown buildinfo export mode: %s", s)
	}
}
