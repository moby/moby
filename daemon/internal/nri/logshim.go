package nri

import (
	"context"

	"github.com/containerd/log"
	nrilog "github.com/containerd/nri/pkg/log"
)

type logShim struct{}

// logShim implements interface nrilog.Logger.
var _ nrilog.Logger = (*logShim)(nil)

func (nls *logShim) Debugf(ctx context.Context, format string, args ...any) {
	log.G(ctx).Debugf("NRI: "+format, args...)
}

func (nls *logShim) Infof(ctx context.Context, format string, args ...any) {
	log.G(ctx).Infof("NRI: "+format, args...)
}

func (nls *logShim) Warnf(ctx context.Context, format string, args ...any) {
	log.G(ctx).Warnf("NRI: "+format, args...)
}

func (nls *logShim) Errorf(ctx context.Context, format string, args ...any) {
	log.G(ctx).Errorf("NRI: "+format, args...)
}
