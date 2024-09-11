package dockerui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/imageutil"
	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/pkg/errors"
)

const (
	contextPrefix       = "context:"
	inputMetadataPrefix = "input-metadata:"
	maxContextRecursion = 10
)

func (bc *Client) namedContext(ctx context.Context, name string, nameWithPlatform string, opt ContextOpt) (*llb.State, *dockerspec.DockerOCIImage, error) {
	return bc.namedContextRecursive(ctx, name, nameWithPlatform, opt, 0)
}

func (bc *Client) namedContextRecursive(ctx context.Context, name string, nameWithPlatform string, opt ContextOpt, count int) (*llb.State, *dockerspec.DockerOCIImage, error) {
	opts := bc.bopts.Opts
	contextKey := contextPrefix + nameWithPlatform
	v, ok := opts[contextKey]
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

		ref, dgst, data, err := bc.client.ResolveImageConfig(ctx, named.String(), sourceresolver.Opt{
			LogName:  fmt.Sprintf("[context %s] load metadata for %s", nameWithPlatform, ref),
			Platform: opt.Platform,
			ImageOpt: &sourceresolver.ResolveImageOpt{
				ResolveMode: opt.ResolveMode,
			},
		})
		if err != nil {
			e := &imageutil.ResolveToNonImageError{}
			if errors.As(err, &e) {
				before, after, ok := strings.Cut(e.Updated, "://")
				if !ok {
					return nil, nil, errors.Errorf("could not parse ref: %s", e.Updated)
				}

				bc.bopts.Opts[contextKey] = before + ":" + after
				return bc.namedContextRecursive(ctx, name, nameWithPlatform, opt, count+1)
			}
			return nil, nil, err
		}

		var img dockerspec.DockerOCIImage
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

		_, dgst, data, err := bc.client.ResolveImageConfig(ctx, dummyRef.String(), sourceresolver.Opt{
			LogName:  fmt.Sprintf("[context %s] load metadata for %s", nameWithPlatform, dummyRef.String()),
			Platform: opt.Platform,
			OCILayoutOpt: &sourceresolver.ResolveOCILayoutOpt{
				Store: sourceresolver.ResolveImageConfigOptStore{
					SessionID: bc.bopts.SessionID,
					StoreID:   named.Name(),
				},
			},
		})
		if err != nil {
			return nil, nil, err
		}

		var img dockerspec.DockerOCIImage
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
		sessionID := bc.bopts.SessionID
		if v, ok := bc.localsSessionIDs[vv[1]]; ok {
			sessionID = v
		}
		st := llb.Local(vv[1],
			llb.SessionID(sessionID),
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
				excludes, err = ignorefile.ReadAll(bytes.NewBuffer(dt))
				if err != nil {
					return nil, nil, errors.Wrapf(err, "failed parsing %s", DefaultDockerignoreName)
				}
			}
		}

		localOutput := &asyncLocalOutput{
			name:             vv[1],
			nameWithPlatform: nameWithPlatform,
			sessionID:        sessionID,
			excludes:         excludes,
			extraOpts:        opt.AsyncLocalOpts,
		}
		st = llb.NewState(localOutput)
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
			var img *dockerspec.DockerOCIImage
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

// asyncLocalOutput is an llb.Output that computes an llb.Local
// on-demand instead of at the time of initialization.
type asyncLocalOutput struct {
	llb.Output
	name             string
	nameWithPlatform string
	sessionID        string
	excludes         []string
	extraOpts        func() []llb.LocalOption
	once             sync.Once
}

func (a *asyncLocalOutput) ToInput(ctx context.Context, constraints *llb.Constraints) (*pb.Input, error) {
	a.once.Do(a.do)
	return a.Output.ToInput(ctx, constraints)
}

func (a *asyncLocalOutput) Vertex(ctx context.Context, constraints *llb.Constraints) llb.Vertex {
	a.once.Do(a.do)
	return a.Output.Vertex(ctx, constraints)
}

func (a *asyncLocalOutput) do() {
	var extraOpts []llb.LocalOption
	if a.extraOpts != nil {
		extraOpts = a.extraOpts()
	}
	opts := append([]llb.LocalOption{
		llb.WithCustomName("[context " + a.nameWithPlatform + "] load from client"),
		llb.SessionID(a.sessionID),
		llb.SharedKeyHint("context:" + a.nameWithPlatform),
		llb.ExcludePatterns(a.excludes),
	}, extraOpts...)

	st := llb.Local(a.name, opts...)
	a.Output = st.Output()
}
