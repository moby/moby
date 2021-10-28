package buildinfo

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/urlutil"
	"github.com/pkg/errors"
)

const ImageConfigField = "moby.buildkit.buildinfo.v1"

// Merge combines and fixes build info from image config
// key moby.buildkit.buildinfo.v1.
func Merge(ctx context.Context, buildInfo map[string]string, imageConfig []byte) ([]byte, error) {
	icbi, err := imageConfigBuildInfo(imageConfig)
	if err != nil {
		return nil, err
	}

	// Iterate and combine build sources
	mbis := map[string]exptypes.BuildInfo{}
	for srcs, di := range buildInfo {
		src, err := source.FromString(srcs)
		if err != nil {
			return nil, err
		}
		switch sid := src.(type) {
		case *source.ImageIdentifier:
			for idx, bi := range icbi {
				// Use original user input from image config
				if bi.Type == exptypes.BuildInfoTypeDockerImage && bi.Alias == sid.Reference.String() {
					if _, ok := mbis[bi.Alias]; !ok {
						parsed, err := reference.ParseNormalizedNamed(bi.Ref)
						if err != nil {
							return nil, errors.Wrapf(err, "failed to parse %s", bi.Ref)
						}
						mbis[bi.Alias] = exptypes.BuildInfo{
							Type: exptypes.BuildInfoTypeDockerImage,
							Ref:  reference.TagNameOnly(parsed).String(),
							Pin:  di,
						}
						icbi = append(icbi[:idx], icbi[idx+1:]...)
					}
					break
				}
			}
			if _, ok := mbis[sid.Reference.String()]; !ok {
				mbis[sid.Reference.String()] = exptypes.BuildInfo{
					Type: exptypes.BuildInfoTypeDockerImage,
					Ref:  sid.Reference.String(),
					Pin:  di,
				}
			}
		case *source.GitIdentifier:
			sref := sid.Remote
			if len(sid.Ref) > 0 {
				sref += "#" + sid.Ref
			}
			if len(sid.Subdir) > 0 {
				sref += ":" + sid.Subdir
			}
			if _, ok := mbis[sref]; !ok {
				mbis[sref] = exptypes.BuildInfo{
					Type: exptypes.BuildInfoTypeGit,
					Ref:  urlutil.RedactCredentials(sref),
					Pin:  di,
				}
			}
		case *source.HTTPIdentifier:
			if _, ok := mbis[sid.URL]; !ok {
				mbis[sid.URL] = exptypes.BuildInfo{
					Type: exptypes.BuildInfoTypeHTTP,
					Ref:  urlutil.RedactCredentials(sid.URL),
					Pin:  di,
				}
			}
		}
	}

	// Leftovers build deps in image config. Mostly duplicated ones we
	// don't need but there is an edge case if no instruction except source's
	// one is defined (eg. FROM ...) that can be valid so take it into account.
	for _, bi := range icbi {
		if bi.Type != exptypes.BuildInfoTypeDockerImage {
			continue
		}
		if _, ok := mbis[bi.Alias]; !ok {
			parsed, err := reference.ParseNormalizedNamed(bi.Ref)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s", bi.Ref)
			}
			mbis[bi.Alias] = exptypes.BuildInfo{
				Type: exptypes.BuildInfoTypeDockerImage,
				Ref:  reference.TagNameOnly(parsed).String(),
				Pin:  bi.Pin,
			}
		}
	}

	bis := make([]exptypes.BuildInfo, 0, len(mbis))
	for _, bi := range mbis {
		bis = append(bis, bi)
	}
	sort.Slice(bis, func(i, j int) bool {
		return bis[i].Ref < bis[j].Ref
	})

	return json.Marshal(map[string][]exptypes.BuildInfo{
		"sources": bis,
	})
}

// imageConfigBuildInfo returns build dependencies from image config
func imageConfigBuildInfo(imageConfig []byte) ([]exptypes.BuildInfo, error) {
	if len(imageConfig) == 0 {
		return nil, nil
	}
	var config struct {
		BuildInfo []byte `json:"moby.buildkit.buildinfo.v1,omitempty"`
	}
	if err := json.Unmarshal(imageConfig, &config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal buildinfo from config")
	}
	if len(config.BuildInfo) == 0 {
		return nil, nil
	}
	var bi []exptypes.BuildInfo
	if err := json.Unmarshal(config.BuildInfo, &bi); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal %s", ImageConfigField)
	}
	return bi, nil
}
