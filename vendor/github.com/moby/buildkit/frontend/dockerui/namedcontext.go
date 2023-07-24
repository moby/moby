package dockerui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/exporter/containerimage/image"
	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/util/imageutil"
	"github.com/pkg/errors"
)

const (
	contextPrefix       = "context:"
	inputMetadataPrefix = "input-metadata:"
	maxContextRecursion = 10
)

func (bc *Client) namedContext(ctx context.Context, name string, nameWithPlatform string, opt ContextOpt) (*llb.State, *image.Image, error) {
	return bc.namedContextRecursive(ctx, name, nameWithPlatform, opt, 0)
}

func (bc *Client) namedContextRecursive(ctx context.Context, name string, nameWithPlatform string, opt ContextOpt, count int) (*llb.State, *image.Image, error) {
	opts := bc.bopts.Opts
	v, ok := opts[contextPrefix+nameWithPlatform]
	if !ok {
		return nil, nil, nil
	}

	if count > maxContextRecursion {
		return nil, nil, errors.New("context recursion limit exceeded; this may indicate a cycle in the provided source policies: " + v)
	}

	vv := strings.SplitN(v, ":", 2)
	if len(vv) != 2 {
		return nil, nil, errors.Errorf("invalid context specifier %s for %s", v, nameWithPlatform)
	}

	// allow git@ without protocol for SSH URLs for backwards compatibility
	if strings.HasPrefix(vv[0], "git@") {
		vv[0] = "git"
	}
	switch vv[0] {
	case "docker-image":
		ref := strings.TrimPrefix(vv[1], "//")
		if ref == EmptyImageName {
			st := llb.Scratch()
			return &st, nil, nil
		}

		imgOpt := []llb.ImageOption{
			llb.WithCustomName("[context " + nameWithPlatform + "] " + ref),
		}
		if opt.Platform != nil {
			imgOpt = append(imgOpt, llb.Platform(*opt.Platform))
		}

		named, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, nil, err
		}

		named = reference.TagNameOnly(named)

		ref, dgst, data, err := bc.client.ResolveImageConfig(ctx, named.String(), llb.ResolveImageConfigOpt{
			Platform:     opt.Platform,
			ResolveMode:  opt.ResolveMode,
			LogName:      fmt.Sprintf("[context %s] load metadata for %s", nameWithPlatform, ref),
			ResolverType: llb.ResolverTypeRegistry,
		})
		if err != nil {
			e := &imageutil.ResolveToNonImageError{}
			if errors.As(err, &e) {
				return bc.namedContextRecursive(ctx, e.Updated, name, opt, count+1)
			}
			return nil, nil, err
		}

		var img image.Image
		if err := json.Unmarshal(data, &img); err != nil {
			return nil, nil, err
		}
		img.Created = nil

		st := llb.Image(ref, imgOpt...)
		st, err = st.WithImageConfig(data)
		if err != nil {
			return nil, nil, err
		}
		if opt.CaptureDigest != nil {
			*opt.CaptureDigest = dgst
		}
		return &st, &img, nil
	case "git":
		st, ok := DetectGitContext(v, true)
		if !ok {
			return nil, nil, errors.Errorf("invalid git context %s", v)
		}
		return st, nil, nil
	case "http", "https":
		st, ok := DetectGitContext(v, true)
		if !ok {
			httpst := llb.HTTP(v, llb.WithCustomName("[context "+nameWithPlatform+"] "+v))
			st = &httpst
		}
		return st, nil, nil
	case "oci-layout":
		refSpec := strings.TrimPrefix(vv[1], "//")
		ref, err := reference.Parse(refSpec)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse oci-layout reference %q", refSpec)
		}
		named, ok := ref.(reference.Named)
		if !ok {
			return nil, nil, errors.Errorf("oci-layout reference %q has no name", ref.String())
		}
		dgstd, ok := named.(reference.Digested)
		if !ok {
			return nil, nil, errors.Errorf("oci-layout reference %q has no digest", named.String())
		}

		// for the dummy ref primarily used in log messages, we can use the
		// original name, since the store key may not be significant
		dummyRef, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse oci-layout reference %q", name)
		}
		dummyRef, err = reference.WithDigest(dummyRef, dgstd.Digest())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not wrap %q with digest", name)
		}

		// TODO: How should source policy be handled here with a dummy ref?
		_, dgst, data, err := bc.client.ResolveImageConfig(ctx, dummyRef.String(), llb.ResolveImageConfigOpt{
			Platform:     opt.Platform,
			ResolveMode:  opt.ResolveMode,
			LogName:      fmt.Sprintf("[context %s] load metadata for %s", nameWithPlatform, dummyRef.String()),
			ResolverType: llb.ResolverTypeOCILayout,
			Store: llb.ResolveImageConfigOptStore{
				SessionID: bc.bopts.SessionID,
				StoreID:   named.Name(),
			},
		})
		if err != nil {
			return nil, nil, err
		}

		var img image.Image
		if err := json.Unmarshal(data, &img); err != nil {
			return nil, nil, errors.Wrap(err, "could not parse oci-layout image config")
		}

		ociOpt := []llb.OCILayoutOption{
			llb.WithCustomName("[context " + nameWithPlatform + "] OCI load from client"),
			llb.OCIStore(bc.bopts.SessionID, named.Name()),
		}
		if opt.Platform != nil {
			ociOpt = append(ociOpt, llb.Platform(*opt.Platform))
		}
		st := llb.OCILayout(
			dummyRef.String(),
			ociOpt...,
		)
		st, err = st.WithImageConfig(data)
		if err != nil {
			return nil, nil, err
		}
		if opt.CaptureDigest != nil {
			*opt.CaptureDigest = dgst
		}
		return &st, &img, nil
	case "local":
		st := llb.Local(vv[1],
			llb.SessionID(bc.bopts.SessionID),
			llb.FollowPaths([]string{DefaultDockerignoreName}),
			llb.SharedKeyHint("context:"+nameWithPlatform+"-"+DefaultDockerignoreName),
			llb.WithCustomName("[context "+nameWithPlatform+"] load "+DefaultDockerignoreName),
			llb.Differ(llb.DiffNone, false),
		)
		def, err := st.Marshal(ctx)
		if err != nil {
			return nil, nil, err
		}
		res, err := bc.client.Solve(ctx, client.SolveRequest{
			Evaluate:   true,
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, nil, err
		}
		ref, err := res.SingleRef()
		if err != nil {
			return nil, nil, err
		}
		var excludes []string
		if !opt.NoDockerignore {
			dt, _ := ref.ReadFile(ctx, client.ReadRequest{
				Filename: DefaultDockerignoreName,
			}) // error ignored

			if len(dt) != 0 {
				excludes, err = dockerignore.ReadAll(bytes.NewBuffer(dt))
				if err != nil {
					return nil, nil, err
				}
			}
		}
		st = llb.Local(vv[1],
			llb.WithCustomName("[context "+nameWithPlatform+"] load from client"),
			llb.SessionID(bc.bopts.SessionID),
			llb.SharedKeyHint("context:"+nameWithPlatform),
			llb.ExcludePatterns(excludes),
		)
		return &st, nil, nil
	case "input":
		inputs, err := bc.client.Inputs(ctx)
		if err != nil {
			return nil, nil, err
		}
		st, ok := inputs[vv[1]]
		if !ok {
			return nil, nil, errors.Errorf("invalid input %s for %s", vv[1], nameWithPlatform)
		}
		md, ok := opts[inputMetadataPrefix+vv[1]]
		if ok {
			m := make(map[string][]byte)
			if err := json.Unmarshal([]byte(md), &m); err != nil {
				return nil, nil, errors.Wrapf(err, "failed to parse input metadata %s", md)
			}
			var img *image.Image
			if dtic, ok := m[exptypes.ExporterImageConfigKey]; ok {
				st, err = st.WithImageConfig(dtic)
				if err != nil {
					return nil, nil, err
				}
				if err := json.Unmarshal(dtic, &img); err != nil {
					return nil, nil, errors.Wrapf(err, "failed to parse image config for %s", nameWithPlatform)
				}
			}
			return &st, img, nil
		}
		return &st, nil, nil
	default:
		return nil, nil, errors.Errorf("unsupported context source %s for %s", vv[0], nameWithPlatform)
	}
}
