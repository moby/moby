package blkiodev

import (
	"fmt"
)

// WeightDevice is a structure that hold device:weight pair
type WeightDevice struct {
	Path   string
	Weight uint16
}

func (w *WeightDevice) String() string {
	return fmt.Sprintf("%s:%d", w.Path, w.Weight)
}
