package secret

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

func getSecretsByName(ctx context.Context, client client.APIClient, names []string) ([]swarm.Secret, error) {
	args := filters.NewArgs()
	for _, n := range names {
		args.Add("names", n)
	}

	return client.SecretList(ctx, types.SecretListOptions{
		Filters: args,
	})
}

func getCliRequestedSecretIDs(ctx context.Context, client client.APIClient, names []string) ([]string, error) {
	ids := names

	// attempt to lookup secret by name
	secrets, err := getSecretsByName(ctx, client, ids)
	if err != nil {
		return nil, err
	}

	lookup := make(map[string]struct{})
	for _, id := range ids {
		lookup[id] = struct{}{}
	}

	if len(secrets) > 0 {
		ids = []string{}

		for _, s := range secrets {
			if _, ok := lookup[s.Spec.Annotations.Name]; ok {
				ids = append(ids, s.ID)
			}
		}
	}

	return ids, nil
}
