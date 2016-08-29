package cluster

import (
	"fmt"

	swarmapi "github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

func getSwarm(ctx context.Context, c swarmapi.ControlClient) (*swarmapi.Cluster, error) {
	rl, err := c.ListClusters(ctx, &swarmapi.ListClustersRequest{})
	if err != nil {
		return nil, err
	}

	if len(rl.Clusters) == 0 {
		return nil, fmt.Errorf("swarm not found")
	}

	// TODO: assume one cluster only
	return rl.Clusters[0], nil
}

func getNode(ctx context.Context, c swarmapi.ControlClient, input string) (*swarmapi.Node, error) {
	// GetNode to match via full ID.
	rg, err := c.GetNode(ctx, &swarmapi.GetNodeRequest{NodeID: input})
	if err != nil {
		// If any error (including NotFound), ListNodes to match via full name.
		rl, err := c.ListNodes(ctx, &swarmapi.ListNodesRequest{Filters: &swarmapi.ListNodesRequest_Filters{Names: []string{input}}})

		if err != nil || len(rl.Nodes) == 0 {
			// If any error or 0 result, ListNodes to match via ID prefix.
			rl, err = c.ListNodes(ctx, &swarmapi.ListNodesRequest{Filters: &swarmapi.ListNodesRequest_Filters{IDPrefixes: []string{input}}})
		}

		if err != nil {
			return nil, err
		}

		if len(rl.Nodes) == 0 {
			return nil, fmt.Errorf("node %s not found", input)
		}

		if l := len(rl.Nodes); l > 1 {
			return nil, fmt.Errorf("node %s is ambiguous (%d matches found)", input, l)
		}

		return rl.Nodes[0], nil
	}
	return rg.Node, nil
}

func getService(ctx context.Context, c swarmapi.ControlClient, input string) (*swarmapi.Service, error) {
	// GetService to match via full ID.
	rg, err := c.GetService(ctx, &swarmapi.GetServiceRequest{ServiceID: input})
	if err != nil {
		// If any error (including NotFound), ListServices to match via full name.
		rl, err := c.ListServices(ctx, &swarmapi.ListServicesRequest{Filters: &swarmapi.ListServicesRequest_Filters{Names: []string{input}}})
		if err != nil || len(rl.Services) == 0 {
			// If any error or 0 result, ListServices to match via ID prefix.
			rl, err = c.ListServices(ctx, &swarmapi.ListServicesRequest{Filters: &swarmapi.ListServicesRequest_Filters{IDPrefixes: []string{input}}})
		}

		if err != nil {
			return nil, err
		}

		if len(rl.Services) == 0 {
			return nil, fmt.Errorf("service %s not found", input)
		}

		if l := len(rl.Services); l > 1 {
			return nil, fmt.Errorf("service %s is ambiguous (%d matches found)", input, l)
		}

		return rl.Services[0], nil
	}
	return rg.Service, nil
}

func getTask(ctx context.Context, c swarmapi.ControlClient, input string) (*swarmapi.Task, error) {
	// GetTask to match via full ID.
	rg, err := c.GetTask(ctx, &swarmapi.GetTaskRequest{TaskID: input})
	if err != nil {
		// If any error (including NotFound), ListTasks to match via full name.
		rl, err := c.ListTasks(ctx, &swarmapi.ListTasksRequest{Filters: &swarmapi.ListTasksRequest_Filters{Names: []string{input}}})

		if err != nil || len(rl.Tasks) == 0 {
			// If any error or 0 result, ListTasks to match via ID prefix.
			rl, err = c.ListTasks(ctx, &swarmapi.ListTasksRequest{Filters: &swarmapi.ListTasksRequest_Filters{IDPrefixes: []string{input}}})
		}

		if err != nil {
			return nil, err
		}

		if len(rl.Tasks) == 0 {
			return nil, fmt.Errorf("task %s not found", input)
		}

		if l := len(rl.Tasks); l > 1 {
			return nil, fmt.Errorf("task %s is ambiguous (%d matches found)", input, l)
		}

		return rl.Tasks[0], nil
	}
	return rg.Task, nil
}
