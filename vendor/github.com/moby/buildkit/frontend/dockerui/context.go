package dockerui

import (
	"archive/tar"
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dfgitutil"
	"github.com/moby/buildkit/frontend/gateway/client"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/pkg/errors"
)

const (
	DefaultLocalNameContext    = "context"
	DefaultLocalNameDockerfile = "dockerfile"
	DefaultDockerfileName      = "Dockerfile"
	DefaultDockerignoreName    = ".dockerignore"
	EmptyImageName             = "scratch"
)

const (
	keyFilename       = "filename"
	keyContextSubDir  = "contextsubdir"
	keyNameContext    = "contextkey"
	keyNameDockerfile = "dockerfilekey"
)

var httpPrefix = regexp.MustCompile(`^https?://`)

type buildContext struct {
	context              *llb.State // set if not local
	dockerfile           *llb.State // override remoteContext if set
	contextLocalName     string
	dockerfileLocalName  string
	filename             string
	forceLocalDockerfile bool
}

func (bc *Client) marshalOpts() []llb.ConstraintsOpt {
	return []llb.ConstraintsOpt{llb.WithCaps(bc.bopts.Caps)}
}

func (bc *Client) initContext(ctx context.Context) (*buildContext, error) {
	opts := bc.bopts.Opts
	gwcaps := bc.bopts.Caps

	localNameContext := DefaultLocalNameContext
	if v, ok := opts[keyNameContext]; ok {
		localNameContext = v
	}

	bctx := &buildContext{
		contextLocalName:    DefaultLocalNameContext,
		dockerfileLocalName: DefaultLocalNameDockerfile,
		filename:            DefaultDockerfileName,
	}

	if v, ok := opts[keyFilename]; ok {
		bctx.filename = v
	}

	if v, ok := opts[keyNameDockerfile]; ok {
		bctx.forceLocalDockerfile = true
		bctx.dockerfileLocalName = v
	}

	var keepGit *bool
	if v, err := strconv.ParseBool(opts[keyContextKeepGitDirArg]); err == nil {
		keepGit = &v
	}
	if st, ok, err := DetectGitContext(opts[localNameContext], keepGit); ok {
		if err != nil {
			return nil, err
		}
		bctx.context = st
		bctx.dockerfile = st
	} else if st, filename, ok := DetectHTTPContext(opts[localNameContext]); ok {
		def, err := st.Marshal(ctx, bc.marshalOpts()...)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal httpcontext")
		}
		res, err := bc.client.Solve(ctx, client.SolveRequest{
			Definition: def.ToPB(),
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve httpcontext")
		}

		ref, err := res.SingleRef()
		if err != nil {
			return nil, err
		}

		dt, err := ref.ReadFile(ctx, client.ReadRequest{
			Filename: filename,
			Range: &client.FileRange{
				Length: 1024,
			},
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read downloaded context")
		}
		if isArchive(dt) {
			bc := llb.Scratch().File(llb.Copy(*st, filepath.Join("/", filename), "/", &llb.CopyInfo{
				AttemptUnpack: true,
			}))
			bctx.context = &bc
		} else {
			bctx.filename = filename
			bctx.context = st
		}
		bctx.dockerfile = bctx.context
	} else if (&gwcaps).Supports(gwpb.CapFrontendInputs) == nil {
		inputs, err := bc.client.Inputs(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get frontend inputs")
		}

		if !bctx.forceLocalDockerfile {
			inputDockerfile, ok := inputs[bctx.dockerfileLocalName]
			if ok {
				bctx.dockerfile = &inputDockerfile
			}
		}

		inputCtx, ok := inputs[DefaultLocalNameContext]
		if ok {
			bctx.context = &inputCtx
		}
	}

	if bctx.context != nil {
		if sub, ok := opts[keyContextSubDir]; ok {
			bctx.context = scopeToSubDir(bctx.context, sub)
		}
	}

	return bctx, nil
}

func DetectGitContext(ref string, keepGit *bool) (*llb.State, bool, error) {
	g, isGit, err := dfgitutil.ParseGitRef(ref)
	if err != nil {
		return nil, isGit, err
	}
	gitOpts := []llb.GitOption{
		llb.GitRef(g.Ref),
		WithInternalName("load git source " + ref),
	}
	if g.KeepGitDir != nil && *g.KeepGitDir {
		gitOpts = append(gitOpts, llb.KeepGitDir())
	}
	if keepGit != nil && *keepGit {
		gitOpts = append(gitOpts, llb.KeepGitDir())
	}
	if g.SubDir != "" {
		gitOpts = append(gitOpts, llb.GitSubDir(g.SubDir))
	}
	if g.Checksum != "" {
		gitOpts = append(gitOpts, llb.GitChecksum(g.Checksum))
	}
	if g.Submodules != nil && !*g.Submodules {
		gitOpts = append(gitOpts, llb.GitSkipSubmodules())
	}

	st := llb.Git(g.Remote, "", gitOpts...)
	return &st, true, nil
}

func DetectHTTPContext(ref string) (*llb.State, string, bool) {
	filename := "context"
	if httpPrefix.MatchString(ref) {
		st := llb.HTTP(ref, llb.Filename(filename), WithInternalName("load remote build context"))
		return &st, filename, true
	}
	return nil, "", false
}

func isArchive(header []byte) bool {
	for _, m := range [][]byte{
		{0x42, 0x5A, 0x68},                   // bzip2
		{0x1F, 0x8B, 0x08},                   // gzip
		{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, // xz
	} {
		if len(header) < len(m) {
			continue
		}
		if bytes.Equal(m, header[:len(m)]) {
			return true
		}
	}

	r := tar.NewReader(bytes.NewBuffer(header))
	_, err := r.Next()
	return err == nil
}

func scopeToSubDir(c *llb.State, dir string) *llb.State {
	bc := llb.Scratch().File(llb.Copy(*c, dir, "/", &llb.CopyInfo{
		CopyDirContentsOnly: true,
	}))
	return &bc
}
