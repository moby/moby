//go:build windows

package runhcs

import (
	"context"
	"encoding/json"
	"fmt"
)

// Ps displays the processes running inside a container.
func (r *Runhcs) Ps(ctx context.Context, id string) ([]int, error) {
	data, err := cmdOutput(r.command(ctx, "ps", "--format=json", id), true)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data) //nolint:errorlint // legacy code
	}
	var out []int
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
