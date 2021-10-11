package computestorage

import (
	"context"
	"encoding/json"

	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/sys/windows"
)

// SetupBaseOSLayer sets up a layer that contains a base OS for a container.
//
// `layerPath` is a path to a directory containing the layer.
//
// `vhdHandle` is an empty file handle of `options.Type == OsLayerTypeContainer`
// or else it is a file handle to the 'SystemTemplateBase.vhdx' if `options.Type
// == OsLayerTypeVm`.
//
// `options` are the options applied while processing the layer.
func SetupBaseOSLayer(ctx context.Context, layerPath string, vhdHandle windows.Handle, options OsLayerOptions) (err error) {
	title := "hcsshim.SetupBaseOSLayer"
	ctx, span := trace.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(
		trace.StringAttribute("layerPath", layerPath),
	)

	bytes, err := json.Marshal(options)
	if err != nil {
		return err
	}

	err = hcsSetupBaseOSLayer(layerPath, vhdHandle, string(bytes))
	if err != nil {
		return errors.Wrap(err, "failed to setup base OS layer")
	}
	return nil
}

// SetupBaseOSVolume sets up a volume that contains a base OS for a container.
//
// `layerPath` is a path to a directory containing the layer.
//
// `volumePath` is the path to the volume to be used for setup.
//
// `options` are the options applied while processing the layer.
func SetupBaseOSVolume(ctx context.Context, layerPath, volumePath string, options OsLayerOptions) (err error) {
	if osversion.Build() < 19645 {
		return errors.New("SetupBaseOSVolume is not present on builds older than 19645")
	}
	title := "hcsshim.SetupBaseOSVolume"
	ctx, span := trace.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(
		trace.StringAttribute("layerPath", layerPath),
		trace.StringAttribute("volumePath", volumePath),
	)

	bytes, err := json.Marshal(options)
	if err != nil {
		return err
	}

	err = hcsSetupBaseOSVolume(layerPath, volumePath, string(bytes))
	if err != nil {
		return errors.Wrap(err, "failed to setup base OS layer")
	}
	return nil
}
