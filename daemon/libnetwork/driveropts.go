package libnetwork

import (
	"context"

	"github.com/containerd/log"
)

// ApplyDefaultDriverOpts merges default driver options into opts.
// Existing keys in opts take precedence over defaults.
func ApplyDefaultDriverOpts(ctx context.Context, opts map[string]string, driver, network string, defaults map[string]map[string]string) {
	if defaults == nil {
		return
	}
	if defaultOpts, ok := defaults[driver]; ok {
		for k, v := range defaultOpts {
			if _, ok := opts[k]; !ok {
				log.G(ctx).WithFields(log.Fields{"driver": driver, "network": network, k: v}).Debug("Applying network default option")
				opts[k] = v
			}
		}
	}
}
