package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import "github.com/docker/docker/pkg/idtools"

func parseChownFlag(chown, ctrRootPath string, idMappings *idtools.IDMappings) (idtools.IDPair, error) {
	return idMappings.RootPair(), nil
}
