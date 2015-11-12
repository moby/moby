// +build windows

package graph

// allowBaseParentImage allows images to define a custom parent that is not
// transported with push/pull but already included with the installation.
// Only used in Windows.
const allowBaseParentImage = true
