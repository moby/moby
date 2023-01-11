package registry

import "github.com/docker/docker/api/types/registry"

// ParseSearchIndexInfo will use repository name to get back an indexInfo.
//
// TODO(thaJeztah) this function is only used by the CLI, and used to get
// information of the registry (to provide credentials if needed). We should
// move this function (or equivalent) to the CLI, as it's doing too much just
// for that.
func ParseSearchIndexInfo(reposName string) (*registry.IndexInfo, error) {
	indexName, _ := splitReposSearchTerm(reposName)

	indexInfo, err := newIndexInfo(emptyServiceConfig, indexName)
	if err != nil {
		return nil, err
	}
	return indexInfo, nil
}
