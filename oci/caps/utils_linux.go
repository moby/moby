package caps // import "github.com/docker/docker/oci/caps"
import (
	"sync"

	ccaps "github.com/containerd/containerd/pkg/cap"
	"github.com/sirupsen/logrus"
)

var initCapsOnce sync.Once

func initCaps() {
	initCapsOnce.Do(func() {
		rawCaps := ccaps.Known()
		curCaps, err := ccaps.Current()
		if err != nil {
			logrus.WithError(err).Error("failed to get capabilities from current environment")
			allCaps = rawCaps
		} else {
			allCaps = curCaps
		}
		knownCaps = make(map[string]*struct{}, len(rawCaps))
		for _, capName := range rawCaps {
			// For now, we assume the capability is available if we failed to
			// get the capabilities from the current environment. This keeps the
			// old (pre-detection) behavior, and prevents creating containers with
			// no capabilities. The OCI runtime or kernel may still refuse capa-
			// bilities that are not available, and produce an error in that case.
			if len(curCaps) > 0 && !inSlice(curCaps, capName) {
				knownCaps[capName] = nil
				continue
			}
			knownCaps[capName] = &struct{}{}
		}
	})
}
