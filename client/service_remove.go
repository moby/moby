package client

import "context"

// ServiceRemoveOptions contains options for removing a service.
type ServiceRemoveOptions struct {
	// No options currently; placeholder for future use
}

// ServiceRemoveResult contains the result of removing a service.
type ServiceRemoveResult struct {
	// No fields currently; placeholder for future use
}

// ServiceRemove kills and removes a service.
func (cli *Client) ServiceRemove(ctx context.Context, serviceID string, options ServiceRemoveOptions) (ServiceRemoveResult, error) {
	serviceID, err := trimID("service", serviceID)
	if err != nil {
		return ServiceRemoveResult{}, err
	}

	resp, err := cli.delete(ctx, "/services/"+serviceID, nil, nil)
	defer ensureReaderClosed(resp)
	return ServiceRemoveResult{}, err
}
