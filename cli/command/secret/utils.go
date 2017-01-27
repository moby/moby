package secret

import (
	"fmt"
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

	if len(secrets) > 0 {
		found := make(map[string]struct{})
	next:
		for _, term := range terms {
			// attempt to lookup secret by full ID
			for _, s := range secrets {
				if s.ID == term {
					found[s.ID] = struct{}{}
					continue next
				}
			}
			// attempt to lookup secret by full name
			for _, s := range secrets {
				if s.Spec.Annotations.Name == term {
					found[s.ID] = struct{}{}
					continue next
				}
			}
			// attempt to lookup secret by partial ID (prefix)
			// return error if more than one matches found (ambiguous)
			n := 0
			for _, s := range secrets {
				if strings.HasPrefix(s.ID, term) {
					found[s.ID] = struct{}{}
					n++
				}
			}
			if n > 1 {
				return nil, fmt.Errorf("secret %s is ambiguous (%d matches found)", term, n)
			}
		}

		// We already collected all the IDs found.
		// Now we will remove duplicates by converting the map to slice
		ids := []string{}
		for id := range found {
			ids = append(ids, id)
		}

		return ids, nil
	}

	return terms, nil
}
