package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/moby/moby/v2/daemon/container"
	createspecv0 "github.com/moby/moby/v2/extpoints/createspec/v0"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// runCreateSpecHooks runs the create-spec extension point on the container's OCI
// runtime spec: it lets extensions reshape the spec in sequence, validates the
// result, and applies it back onto spec. A provider's veto aborts the start.
//
// The spec is handed over as canonical runtime-spec JSON, so a provider works
// against the actual OCI schema rather than a re-modeled subset.
func (daemon *Daemon) runCreateSpecHooks(ctx context.Context, c *container.Container, spec *specs.Spec) error {
	if daemon.extensionHost == nil {
		return nil
	}
	// Skip building the request entirely when no extension implements the point,
	// so an unused hook costs nothing (no full-spec marshal) on the start path.
	enabled, err := createspecv0.Enabled(daemon.extensionHost)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal OCI spec: %w", err)
	}
	req := &createspecv0.SpecRequest{
		ContainerID: c.ID,
		Name:        c.Name,
		Spec:        raw,
		Labels:      c.Config.Labels,
	}
	adjusted, err := createspecv0.CreateSpec(ctx, daemon.extensionHost, req)
	if err != nil {
		return err
	}
	req.Spec = adjusted
	if err := createspecv0.Validate(ctx, daemon.extensionHost, req); err != nil {
		return err
	}
	// Only re-decode when a provider actually changed the spec; a no-op pass must
	// not pay for a full unmarshal-and-replace.
	if !bytes.Equal(adjusted, raw) {
		var out specs.Spec
		if err := json.Unmarshal(adjusted, &out); err != nil {
			return fmt.Errorf("unmarshal adjusted OCI spec: %w", err)
		}
		*spec = out
	}
	return nil
}
