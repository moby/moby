//go:build windows

package cim

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
)

// A BlockCIMLayerWriter implements the CIMLayerWriter interface to allow writing
// container image layers in the blocked cim format.
type BlockCIMLayerWriter struct {
	*cimLayerWriter
	// the layer that we are writing
	layer *cimfs.BlockCIM
	// parent layers
	parentLayers []*cimfs.BlockCIM
	// added files maintains a map of all files that have been added to this layer
	addedFiles map[string]struct{}
}

var _ CIMLayerWriter = &BlockCIMLayerWriter{}

// NewBlockCIMLayerWriterWithOpts returns a writer for writing image layers in the block CIM format. The writer's behavior can be
// controlled with the supports opts.
func NewBlockCIMLayerWriterWithOpts(ctx context.Context, layer *cimfs.BlockCIM, parentLayers []*cimfs.BlockCIM, opts ...cimfs.BlockCIMOpt) (_ *BlockCIMLayerWriter, err error) {
	if layer.Type != cimfs.BlockCIMTypeSingleFile {
		// we only support writing single file CIMs for now because in layer
		// writing process we still need to write some files (registry hives)
		// outside the CIM. We currently use the parent directory of the CIM (i.e
		// the parent directory of block path in this case) for this. This can't
		// be reliably done with the block device CIM since the block path
		// provided will be a volume path. However, once we get rid of hive rollup
		// step during layer import we should be able to support block device
		// CIMs.
		return nil, ErrBlockCIMWriterNotSupported
	}

	parentLayerPaths := make([]string, 0, len(parentLayers))
	for _, pl := range parentLayers {
		if pl.Type != layer.Type {
			return nil, ErrBlockCIMParentTypeMismatch
		}
		parentLayerPaths = append(parentLayerPaths, filepath.Dir(pl.BlockPath))
	}

	// We always want to write layers with consistent flag
	bcimOpts := append([]cimfs.BlockCIMOpt{cimfs.WithConsistentCIM()}, opts...)

	cim, err := cimfs.CreateBlockCIMWithOptions(ctx, layer, bcimOpts...)
	if err != nil {
		return nil, fmt.Errorf("error in creating a new cim: %w", err)
	}
	defer func() {
		if err != nil {
			cErr := cim.Close()
			if cErr != nil {
				log.G(ctx).WithError(err).Warnf("failed to close cim after error: %s", cErr)
			}
		}
	}()

	// std file writer writes registry hives outside the CIM for 2 reasons.  1. We can
	// merge the hives of this layer with the parent layer hives and then write the
	// merged hives into the CIM.  2. When importing child layer of this layer, we
	// have access to the merges hives of this layer.
	sfw, err := newStdFileWriter(filepath.Dir(layer.BlockPath), parentLayerPaths)
	if err != nil {
		return nil, fmt.Errorf("error in creating new standard file writer: %w", err)
	}

	return &BlockCIMLayerWriter{
		layer:        layer,
		parentLayers: parentLayers,
		addedFiles:   make(map[string]struct{}),
		cimLayerWriter: &cimLayerWriter{
			ctx:              ctx,
			cimWriter:        cim,
			stdFileWriter:    sfw,
			layerPath:        filepath.Dir(layer.BlockPath),
			parentLayerPaths: parentLayerPaths,
		},
	}, nil
}

// NewBlockCIMLayerWriter writes the layer files in the block CIM format.
func NewBlockCIMLayerWriter(ctx context.Context, layer *cimfs.BlockCIM, parentLayers []*cimfs.BlockCIM) (_ *BlockCIMLayerWriter, err error) {
	return NewBlockCIMLayerWriterWithOpts(ctx, layer, parentLayers)
}

// Add adds a file to the layer with given metadata.
func (cw *BlockCIMLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo, fileSize int64, securityDescriptor []byte, extendedAttributes []byte, reparseData []byte) error {
	cw.addedFiles[name] = struct{}{}
	if name == wclayer.UtilityVMPath && len(cw.parentLayers) > 0 {
		// If there are UtilityVM files in non base layers, we will have to merge
		// those files with the parent layer UtilityVM files - either during image
		// pull or at runtime (i.e when starting the UVM). In order to merge at image pull time, we will have
		// to read parent layer block CIMs and copy all the UtilityVM files from
		// those CIMs into this block CIM one by one i.e effectively merge all
		// parent layer UtilityVM files in this layer. Or we will need to be able
		// to boot the UtilityVM with merged block CIMs. None of these options are
		// implemented yet so error out if we see that.
		return fmt.Errorf("UtilityVM files in non base layers is not supported for block CIMs")
	}
	return cw.cimLayerWriter.Add(name, fileInfo, fileSize, securityDescriptor, extendedAttributes, reparseData)
}

// Remove removes a file that was present in a parent layer from the layer.
func (cw *BlockCIMLayerWriter) Remove(name string) error {
	// set active write to nil so that we panic if layer tar is incorrectly formatted.
	cw.activeWriter = nil
	err := cw.cimWriter.AddTombstone(name)
	if err != nil {
		return fmt.Errorf("failed to remove file : %w", err)
	}
	return nil
}

// AddLink adds a hard link to the layer. Note that the link added here is evaluated only
// at the CIM merge time. So an invalid link will not throw an error here.
func (cw *BlockCIMLayerWriter) AddLink(name string, target string) error {
	// set active write to nil so that we panic if layer tar is incorrectly formatted.
	cw.activeWriter = nil

	// when adding links to a block CIM, we need to know if the target file is present
	// in this same block CIM or if it is coming from one of the parent layers. If the
	// file is in the same CIM we add a standard hard link. If the file is not in the
	// same CIM we add a special type of link called merged link. This merged link is
	// resolved when all the individual block CIM layers are merged. In order to
	// reliably know if the target is a part of the CIM or not, we wait until all
	// files are added and then lookup the added entries in a map to make the
	// decision.
	pendingLinkOp := func(c *cimfs.CimFsWriter) error {
		if _, ok := cw.addedFiles[target]; ok {
			// target was added in this layer - add a normal link. Once a
			// hardlink is added that hardlink also becomes a valid target for
			// other links so include it in the map.
			cw.addedFiles[name] = struct{}{}
			return c.AddLink(target, name)
		} else {
			// target is from a parent layer - add a merged link
			return c.AddMergedLink(target, name)
		}
	}
	cw.pendingOps = append(cw.pendingOps, pendingCimOpFunc(pendingLinkOp))
	return nil

}
