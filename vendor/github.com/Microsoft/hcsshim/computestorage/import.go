package computestorage

import (
	"context"
	"encoding/json"

	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

// ImportLayer imports a container layer.
//
// `layerPath` is a path to a directory to import the layer to. If the directory
// does not exist it will be automatically created.
//
// `sourceFolderpath` is a pre-existing folder that contains the layer to
// import.
//
// `layerData` is the parent layer data.
func ImportLayer(ctx context.Context, layerPath, sourceFolderPath string, layerData LayerData) (err error) {
	title := "hcsshim.ImportLayer"
	ctx, span := trace.StartSpan(ctx, title) //nolint:ineffassign,staticcheck
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(
		trace.StringAttribute("layerPath", layerPath),
		trace.StringAttribute("sourceFolderPath", sourceFolderPath),
	)

	bytes, err := json.Marshal(layerData)
	if err != nil {
		return err
	}

	err = hcsImportLayer(layerPath, sourceFolderPath, string(bytes))
	if err != nil {
		return errors.Wrap(err, "failed to import layer")
	}
	return nil
}
