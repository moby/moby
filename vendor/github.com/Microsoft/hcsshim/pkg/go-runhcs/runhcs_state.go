//go:build windows

package runhcs

import (
	"context"
	"encoding/json"
	"fmt"
)

// State outputs the state of a container.
func (r *Runhcs) State(ctx context.Context, id string) (*ContainerState, error) {
	data, err := cmdOutput(r.command(ctx, "state", id), true)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data) //nolint:errorlint // legacy code
	}
	var out ContainerState
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
