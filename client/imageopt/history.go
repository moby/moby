package imageopt

import (
	"context"
	"fmt"

	"github.com/moby/moby/client/internal/opts"
)

func (o *withSinglePlatformOption) ApplyImageHistoryOption(ctx context.Context, opts *opts.ImageHistoryOptions) error {
	if opts.ApiOptions.Platform != nil {
		return fmt.Errorf("platform already set to %v", opts.ApiOptions.Platform)
	}

	opts.ApiOptions.Platform = &o.platform
	return nil
}
