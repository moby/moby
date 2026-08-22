package client

import (
	"context"
	"net/url"

	"github.com/moby/moby/api/types/swarm"
)

// ServiceUpdateInterruptOptions holds parameters to interrupt a service
// update with.
type ServiceUpdateInterruptOptions struct {
	Version swarm.Version

	// Disposition controls what happens to task replacements the update
	// already completed when the interrupt takes effect. It defaults to
	// [swarm.ServiceUpdateInterruptHold] if empty.
	Disposition swarm.ServiceUpdateInterruptDisposition
}

// ServiceUpdateInterruptResult holds the result of [Client.ServiceUpdateInterrupt].
type ServiceUpdateInterruptResult struct{}

// ServiceUpdateInterrupt interrupts a service update or rollback that is
// currently in progress, stopping it from scheduling further task
// replacements once the tasks currently being replaced finish or fail. It
// is a no-op that succeeds if the service has no update in progress.
//
// The version number is required to avoid conflicting writes. It must be
// the value as set *before* the call. You can find this value in the
// [swarm.Service.Meta] field, which can be found using
// [Client.ServiceInspectWithRaw].
//
// Interrupting does not wait for the interruption to take effect: the
// service's UpdateStatus.State becomes "interrupted" once it does, which
// the caller should watch for using [Client.ServiceInspectWithRaw] or the
// events API.
func (cli *Client) ServiceUpdateInterrupt(ctx context.Context, serviceID string, options ServiceUpdateInterruptOptions) (ServiceUpdateInterruptResult, error) {
	serviceID, err := trimID("service", serviceID)
	if err != nil {
		return ServiceUpdateInterruptResult{}, err
	}

	query := url.Values{}
	query.Set("version", options.Version.String())
	if options.Disposition != "" {
		query.Set("disposition", string(options.Disposition))
	}

	resp, err := cli.post(ctx, "/services/"+serviceID+"/interrupt", query, nil, nil)
	defer ensureReaderClosed(resp)
	return ServiceUpdateInterruptResult{}, err
}
