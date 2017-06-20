package client

import (
	"golang.org/x/net/context"
)

// ServiceRemoveReplica removes a replica referenced by a task.
func (cli *Client) ServiceRemoveReplica(ctx context.Context, serviceID string, slot string) error {
	resp, err := cli.delete(ctx, "/services/"+serviceID+"/replicas/"+slot, nil, nil)
	ensureReaderClosed(resp)
	return err
}
