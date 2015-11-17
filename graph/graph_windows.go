// +build windows

package graph

import "io"

// allowBaseParentImage allows images to define a custom parent that is not
// transported with push/pull but already included with the installation.
// Only used in Windows.
const allowBaseParentImage = true

// Windows does not currently support tarsplit functionality.
func (graph *Graph) disassembleAndApplyTarLayer(id, parent string, layerData io.Reader, root string) (int64, error) {
	return graph.driver.ApplyDiff(id, parent, layerData)
}
