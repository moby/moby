package secret

import (
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

// GetSecretsByNameOrIDPrefixes returns secrets given a list of ids or names
func GetSecretsByNameOrIDPrefixes(ctx context.Context, client client.APIClient, terms []string) ([]swarm.Secret, error) {
	args := filters.NewArgs()
	for _, n := range terms {
		args.Add("names", n)
		args.Add("id", n)
	}

	return client.SecretList(ctx, types.SecretListOptions{
		Filters: args,
	})
}

func getCliRequestedSecretIDs(ctx context.Context, client client.APIClient, terms []string) ([]string, error) {
	secrets, err := GetSecretsByNameOrIDPrefixes(ctx, client, terms)
	if err != nil {
		return nil, err
	}

	for index, term := range terms {
		for _, s := range secrets {
			// attempt to lookup secret by full ID
			if s.ID == term {
				break
			}

			// attempt to lookup secret by full name
			if s.Spec.Annotations.Name == term {
				terms[index] = s.ID
				break
			}

			// attempt to lookup secret by partial ID (prefix)
			if strings.HasPrefix(s.ID, term) {
				terms[index] = s.ID
				break
			}
		}
	}

	return terms, nil
}
