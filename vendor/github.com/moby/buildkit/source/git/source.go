package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets"
	"github.com/moby/buildkit/session/sshforward"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/moby/buildkit/util/progress/logs"
	"github.com/moby/buildkit/util/urlutil"
	"github.com/moby/locker"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var validHex = regexp.MustCompile(`^[a-f0-9]{40}$`)
var defaultBranch = regexp.MustCompile(`refs/heads/(\S+)`)

type Opt struct {
	CacheAccessor cache.Accessor
}

type gitSource struct {
	cache  cache.Accessor
	locker *locker.Locker
}

// Supported returns nil if the system supports Git source
func Supported() error {
	if err := exec.Command("git", "version").Run(); err != nil {
		return errors.Wrap(err, "failed to find git binary")
	}
	return nil
}

func NewSource(opt Opt) (source.Source, error) {
	gs := &gitSource{
		cache:  opt.CacheAccessor,
		locker: locker.New(),
	}
	return gs, nil
}

func (gs *gitSource) Schemes() []string {
	return []string{srctypes.GitScheme}
}

func (gs *gitSource) Identifier(scheme, ref string, attrs map[string]string, platform *pb.Platform) (source.Identifier, error) {
	id, err := NewGitIdentifier(ref)
	if err != nil {
		return nil, err
	}

	for k, v := range attrs {
		switch k {
		case pb.AttrKeepGitDir:
			if v == "true" {
				id.KeepGitDir = true
			}
		case pb.AttrFullRemoteURL:
			if !gitutil.IsGitTransport(v) {
				v = "https://" + v
			}
			id.Remote = v
		case pb.AttrAuthHeaderSecret:
			id.AuthHeaderSecret = v
		case pb.AttrAuthTokenSecret:
			id.AuthTokenSecret = v
		case pb.AttrKnownSSHHosts:
			id.KnownSSHHosts = v
		case pb.AttrMountSSHSock:
			id.MountSSHSock = v
		}
	}

	return id, nil
}

// needs to be called with repo lock
func (gs *gitSource) mountRemote(ctx context.Context, remote string, authArgs []string, g session.Group) (target string, release func() error, retErr error) {
	sis, err := searchGitRemote(ctx, gs.cache, remote)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to search metadata for %s", urlutil.RedactCredentials(remote))
	}

	var remoteRef cache.MutableRef
	for _, si := range sis {
		remoteRef, err = gs.cache.GetMutable(ctx, si.ID())
		if err != nil {
			if errors.Is(err, cache.ErrLocked) {
				// should never really happen as no other function should access this metadata, but lets be graceful
				bklog.G(ctx).Warnf("mutable ref for %s  %s was locked: %v", urlutil.RedactCredentials(remote), si.ID(), err)
				continue
			}
			return "", nil, errors.Wrapf(err, "failed to get mutable ref for %s", urlutil.RedactCredentials(remote))
		}
		break
	}

	initializeRepo := false
	if remoteRef == nil {
		remoteRef, err = gs.cache.New(ctx, nil, g, cache.CachePolicyRetain, cache.WithDescription(fmt.Sprintf("shared git repo for %s", urlutil.RedactCredentials(remote))))
		if err != nil {
			return "", nil, errors.Wrapf(err, "failed to create new mutable for %s", urlutil.RedactCredentials(remote))
		}
		initializeRepo = true
	}

	releaseRemoteRef := func() error {
		return remoteRef.Release(context.TODO())
	}

	defer func() {
		if retErr != nil && remoteRef != nil {
			releaseRemoteRef()
		}
	}()

	mount, err := remoteRef.Mount(ctx, false, g)
	if err != nil {
		return "", nil, err
	}

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	if err != nil {
		return "", nil, err
	}

	defer func() {
		if retErr != nil {
			lm.Unmount()
		}
	}()

	git := gitCLI(
		gitutil.WithGitDir(dir),
		gitutil.WithArgs(authArgs...),
	)

	if initializeRepo {
		// Explicitly set the Git config 'init.defaultBranch' to the
		// implied default to suppress "hint:" output about not having a
		// default initial branch name set which otherwise spams unit
		// test logs.
		if _, err := git.Run(ctx, "-c", "init.defaultBranch=master", "init", "--bare"); err != nil {
			return "", nil, errors.Wrapf(err, "failed to init repo at %s", dir)
		}

		if _, err := git.Run(ctx, "remote", "add", "origin", remote); err != nil {
			return "", nil, errors.Wrapf(err, "failed add origin repo at %s", dir)
		}

		// save new remote metadata
		md := cacheRefMetadata{remoteRef}
		if err := md.setGitRemote(remote); err != nil {
			return "", nil, err
		}
	}
	return dir, func() error {
		err := lm.Unmount()
		if err1 := releaseRemoteRef(); err == nil {
			err = err1
		}
		return err
	}, nil
}

type gitSourceHandler struct {
	*gitSource
	src      GitIdentifier
	cacheKey string
	sm       *session.Manager
	authArgs []string
}

func (gs *gitSourceHandler) shaToCacheKey(sha string) string {
	key := sha
	if gs.src.KeepGitDir {
		key += ".git"
	}
	if gs.src.Subdir != "" {
		key += ":" + gs.src.Subdir
	}
	return key
}

func (gs *gitSource) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, _ solver.Vertex) (source.SourceInstance, error) {
	gitIdentifier, ok := id.(*GitIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid git identifier %v", id)
	}

	return &gitSourceHandler{
		src:       *gitIdentifier,
		gitSource: gs,
		sm:        sm,
	}, nil
}

type authSecret struct {
	token bool
	name  string
}

func (gs *gitSourceHandler) authSecretNames() (sec []authSecret, _ error) {
	u, err := url.Parse(gs.src.Remote)
	if err != nil {
		return nil, err
	}
	if gs.src.AuthHeaderSecret != "" {
		sec = append(sec, authSecret{name: gs.src.AuthHeaderSecret + "." + u.Host})
	}
	if gs.src.AuthTokenSecret != "" {
		sec = append(sec, authSecret{name: gs.src.AuthTokenSecret + "." + u.Host, token: true})
	}
	if gs.src.AuthHeaderSecret != "" {
		sec = append(sec, authSecret{name: gs.src.AuthHeaderSecret})
	}
	if gs.src.AuthTokenSecret != "" {
		sec = append(sec, authSecret{name: gs.src.AuthTokenSecret, token: true})
	}
	return sec, nil
}

func (gs *gitSourceHandler) getAuthToken(ctx context.Context, g session.Group) error {
	if gs.authArgs != nil {
		return nil
	}
	sec, err := gs.authSecretNames()
	if err != nil {
		return err
	}
	return gs.sm.Any(ctx, g, func(ctx context.Context, _ string, caller session.Caller) error {
		for _, s := range sec {
			dt, err := secrets.GetSecret(ctx, caller, s.name)
			if err != nil {
				if errors.Is(err, secrets.ErrNotFound) {
					continue
				}
				return err
			}
			if s.token {
				dt = []byte("basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("x-access-token:%s", dt))))
			}
			gs.authArgs = []string{"-c", "http." + tokenScope(gs.src.Remote) + ".extraheader=Authorization: " + string(dt)}
			break
		}
		return nil
	})
}

func (gs *gitSourceHandler) mountSSHAuthSock(ctx context.Context, sshID string, g session.Group) (string, func() error, error) {
	var caller session.Caller
	err := gs.sm.Any(ctx, g, func(ctx context.Context, _ string, c session.Caller) error {
		if err := sshforward.CheckSSHID(ctx, c, sshID); err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
				return errors.Errorf("no SSH key %q forwarded from the client", sshID)
			}

			return err
		}
		caller = c
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	usr, err := user.Current()
	if err != nil {
		return "", nil, err
	}

	// best effort, default to root
	uid, _ := strconv.Atoi(usr.Uid)
	gid, _ := strconv.Atoi(usr.Gid)

	sock, cleanup, err := sshforward.MountSSHSocket(ctx, caller, sshforward.SocketOpt{
		ID:   sshID,
		UID:  uid,
		GID:  gid,
		Mode: 0700,
	})
	if err != nil {
		return "", nil, err
	}

	return sock, cleanup, nil
}

func (gs *gitSourceHandler) mountKnownHosts() (string, func() error, error) {
	if gs.src.KnownSSHHosts == "" {
		return "", nil, errors.Errorf("no configured known hosts forwarded from the client")
	}
	knownHosts, err := os.CreateTemp("", "")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error {
		return os.Remove(knownHosts.Name())
	}
	_, err = knownHosts.Write([]byte(gs.src.KnownSSHHosts))
	if err != nil {
		cleanup()
		return "", nil, err
	}
	err = knownHosts.Close()
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return knownHosts.Name(), cleanup, nil
}

func (gs *gitSourceHandler) CacheKey(ctx context.Context, g session.Group, index int) (string, string, solver.CacheOpts, bool, error) {
	remote := gs.src.Remote
	gs.locker.Lock(remote)
	defer gs.locker.Unlock(remote)

	if ref := gs.src.Ref; ref != "" && isCommitSHA(ref) {
		cacheKey := gs.shaToCacheKey(ref)
		gs.cacheKey = cacheKey
		return cacheKey, ref, nil, true, nil
	}

	gs.getAuthToken(ctx, g)

	git, cleanup, err := gs.gitCli(ctx, g)
	if err != nil {
		return "", "", nil, false, err
	}
	defer cleanup()

	ref := gs.src.Ref
	if ref == "" {
		ref, err = getDefaultBranch(ctx, git, gs.src.Remote)
		if err != nil {
			return "", "", nil, false, err
		}
	}

	// TODO: should we assume that remote tag is immutable? add a timer?

	buf, err := git.Run(ctx, "ls-remote", "origin", ref, ref+"^{}")
	if err != nil {
		return "", "", nil, false, errors.Wrapf(err, "failed to fetch remote %s", urlutil.RedactCredentials(remote))
	}
	lines := strings.Split(string(buf), "\n")

	var (
		partialRef      = "refs/" + strings.TrimPrefix(ref, "refs/")
		headRef         = "refs/heads/" + strings.TrimPrefix(ref, "refs/heads/")
		tagRef          = "refs/tags/" + strings.TrimPrefix(ref, "refs/tags/")
		annotatedTagRef = tagRef + "^{}"
	)
	var sha, headSha, tagSha string
	for _, line := range lines {
		lineSha, lineRef, _ := strings.Cut(line, "\t")
		switch lineRef {
		case headRef:
			headSha = lineSha
		case tagRef, annotatedTagRef:
			tagSha = lineSha
		case partialRef:
			sha = lineSha
		}
	}

	// git-checkout prefers branches in case of ambiguity
	if sha == "" {
		sha = headSha
	}
	if sha == "" {
		sha = tagSha
	}
	if sha == "" {
		return "", "", nil, false, errors.Errorf("repository does not contain ref %s, output: %q", ref, string(buf))
	}
	if !isCommitSHA(sha) {
		return "", "", nil, false, errors.Errorf("invalid commit sha %q", sha)
	}

	cacheKey := gs.shaToCacheKey(sha)
	gs.cacheKey = cacheKey
	return cacheKey, sha, nil, true, nil
}

func (gs *gitSourceHandler) Snapshot(ctx context.Context, g session.Group) (out cache.ImmutableRef, retErr error) {
	cacheKey := gs.cacheKey
	if cacheKey == "" {
		var err error
		cacheKey, _, _, _, err = gs.CacheKey(ctx, g, 0)
		if err != nil {
			return nil, err
		}
	}

	gs.getAuthToken(ctx, g)

	snapshotKey := cacheKey + ":" + gs.src.Subdir
	gs.locker.Lock(snapshotKey)
	defer gs.locker.Unlock(snapshotKey)

	sis, err := searchGitSnapshot(ctx, gs.cache, snapshotKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search metadata for %s", snapshotKey)
	}
	if len(sis) > 0 {
		return gs.cache.Get(ctx, sis[0].ID(), nil)
	}

	gs.locker.Lock(gs.src.Remote)
	defer gs.locker.Unlock(gs.src.Remote)

	git, cleanup, err := gs.gitCli(ctx, g)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	gitDir, err := git.GitDir(ctx)
	if err != nil {
		return nil, err
	}

	ref := gs.src.Ref
	if ref == "" {
		ref, err = getDefaultBranch(ctx, git, gs.src.Remote)
		if err != nil {
			return nil, err
		}
	}

	doFetch := true
	if isCommitSHA(ref) {
		// skip fetch if commit already exists
		if _, err := git.Run(ctx, "cat-file", "-e", ref+"^{commit}"); err == nil {
			doFetch = false
		}
	}

	if doFetch {
		// make sure no old lock files have leaked
		os.RemoveAll(filepath.Join(gitDir, "shallow.lock"))

		args := []string{"fetch"}
		if !isCommitSHA(ref) { // TODO: find a branch from ls-remote?
			args = append(args, "--depth=1", "--no-tags")
		} else {
			if _, err := os.Lstat(filepath.Join(gitDir, "shallow")); err == nil {
				args = append(args, "--unshallow")
			}
		}
		args = append(args, "origin")
		if !isCommitSHA(ref) {
			args = append(args, "--force", ref+":tags/"+ref)
			// local refs are needed so they would be advertised on next fetches. Force is used
			// in case the ref is a branch and it now points to a different commit sha
			// TODO: is there a better way to do this?
		}
		if _, err := git.Run(ctx, args...); err != nil {
			return nil, errors.Wrapf(err, "failed to fetch remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		_, err = git.Run(ctx, "reflog", "expire", "--all", "--expire=now")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to expire reflog for remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
	}

	checkoutRef, err := gs.cache.New(ctx, nil, g, cache.WithRecordType(client.UsageRecordTypeGitCheckout), cache.WithDescription(fmt.Sprintf("git snapshot for %s#%s", urlutil.RedactCredentials(gs.src.Remote), ref)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new mutable for %s", urlutil.RedactCredentials(gs.src.Remote))
	}

	defer func() {
		if retErr != nil && checkoutRef != nil {
			checkoutRef.Release(context.TODO())
		}
	}()

	mount, err := checkoutRef.Mount(ctx, false, g)
	if err != nil {
		return nil, err
	}
	lm := snapshot.LocalMounter(mount)
	checkoutDir, err := lm.Mount()
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil && lm != nil {
			lm.Unmount()
		}
	}()

	subdir := path.Clean(gs.src.Subdir)
	if subdir == "/" {
		subdir = "."
	}

	if gs.src.KeepGitDir && subdir == "." {
		checkoutDirGit := filepath.Join(checkoutDir, ".git")
		if err := os.MkdirAll(checkoutDir, 0711); err != nil {
			return nil, err
		}
		checkoutGit := git.New(gitutil.WithWorkTree(checkoutDir), gitutil.WithGitDir(checkoutDirGit))
		_, err = checkoutGit.Run(ctx, "-c", "init.defaultBranch=master", "init")
		if err != nil {
			return nil, err
		}
		// Defense-in-depth: clone using the file protocol to disable local-clone
		// optimizations which can be abused on some versions of Git to copy unintended
		// host files into the build context.
		_, err = checkoutGit.Run(ctx, "remote", "add", "origin", "file://"+gitDir)
		if err != nil {
			return nil, err
		}

		gitCatFileBuf, err := git.Run(ctx, "cat-file", "-t", ref)
		if err != nil {
			return nil, err
		}
		isAnnotatedTag := strings.TrimSpace(string(gitCatFileBuf)) == "tag"

		pullref := ref
		if isAnnotatedTag {
			pullref += ":refs/tags/" + pullref
		} else if isCommitSHA(ref) {
			pullref = "refs/buildkit/" + identity.NewID()
			_, err = git.Run(ctx, "update-ref", pullref, ref)
			if err != nil {
				return nil, err
			}
		} else {
			pullref += ":" + pullref
		}
		_, err = checkoutGit.Run(ctx, "fetch", "-u", "--depth=1", "origin", pullref)
		if err != nil {
			return nil, err
		}
		_, err = checkoutGit.Run(ctx, "checkout", "FETCH_HEAD")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		_, err = checkoutGit.Run(ctx, "remote", "set-url", "origin", urlutil.RedactCredentials(gs.src.Remote))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to set remote origin to %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		_, err = checkoutGit.Run(ctx, "reflog", "expire", "--all", "--expire=now")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to expire reflog for remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		if err := os.Remove(filepath.Join(checkoutDirGit, "FETCH_HEAD")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Wrapf(err, "failed to remove FETCH_HEAD for remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		gitDir = checkoutDirGit
	} else {
		cd := checkoutDir
		if subdir != "." {
			cd, err = os.MkdirTemp(cd, "checkout")
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create temporary checkout dir")
			}
		}
		checkoutGit := git.New(gitutil.WithWorkTree(cd), gitutil.WithGitDir(gitDir))
		_, err = checkoutGit.Run(ctx, "checkout", ref, "--", ".")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		if subdir != "." {
			d, err := os.Open(filepath.Join(cd, subdir))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to open subdir %v", subdir)
			}
			defer func() {
				if d != nil {
					d.Close()
				}
			}()
			names, err := d.Readdirnames(0)
			if err != nil {
				return nil, err
			}
			for _, n := range names {
				if err := os.Rename(filepath.Join(cd, subdir, n), filepath.Join(checkoutDir, n)); err != nil {
					return nil, err
				}
			}
			if err := d.Close(); err != nil {
				return nil, err
			}
			d = nil // reset defer
			if err := os.RemoveAll(cd); err != nil {
				return nil, err
			}
		}
	}

	git = git.New(gitutil.WithWorkTree(checkoutDir), gitutil.WithGitDir(gitDir))
	_, err = git.Run(ctx, "submodule", "update", "--init", "--recursive", "--depth=1")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update submodules for %s", urlutil.RedactCredentials(gs.src.Remote))
	}

	if idmap := mount.IdentityMapping(); idmap != nil {
		u := idmap.RootPair()
		err := filepath.WalkDir(gitDir, func(p string, _ os.DirEntry, _ error) error {
			return os.Lchown(p, u.UID, u.GID)
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to remap git checkout")
		}
	}

	lm.Unmount()
	lm = nil

	snap, err := checkoutRef.Commit(ctx)
	if err != nil {
		return nil, err
	}
	checkoutRef = nil

	defer func() {
		if retErr != nil {
			snap.Release(context.TODO())
		}
	}()

	md := cacheRefMetadata{snap}
	if err := md.setGitSnapshot(snapshotKey); err != nil {
		return nil, err
	}
	return snap, nil
}

func (gs *gitSourceHandler) gitCli(ctx context.Context, g session.Group, opts ...gitutil.Option) (*gitutil.GitCLI, func() error, error) {
	var cleanups []func() error
	cleanup := func() error {
		var err error
		for _, c := range cleanups {
			if err1 := c(); err == nil {
				err = err1
			}
		}
		cleanups = nil
		return err
	}
	var err error

	gitDir, unmountGitDir, err := gs.mountRemote(ctx, gs.src.Remote, gs.authArgs, g)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cleanups = append(cleanups, unmountGitDir)

	var sock string
	if gs.src.MountSSHSock != "" {
		var unmountSock func() error
		sock, unmountSock, err = gs.mountSSHAuthSock(ctx, gs.src.MountSSHSock, g)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		cleanups = append(cleanups, unmountSock)
	}

	var knownHosts string
	if gs.src.KnownSSHHosts != "" {
		var unmountKnownHosts func() error
		knownHosts, unmountKnownHosts, err = gs.mountKnownHosts()
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		cleanups = append(cleanups, unmountKnownHosts)
	}

	opts = append([]gitutil.Option{
		gitutil.WithGitDir(gitDir),
		gitutil.WithArgs(gs.authArgs...),
		gitutil.WithSSHAuthSock(sock),
		gitutil.WithSSHKnownHosts(knownHosts),
	}, opts...)
	return gitCLI(opts...), cleanup, err
}

func isCommitSHA(str string) bool {
	return validHex.MatchString(str)
}

func tokenScope(remote string) string {
	// generally we can only use the token for fetching main remote but in case of github.com we do best effort
	// to try reuse same token for all github.com remotes. This is the same behavior actions/checkout uses
	for _, pfx := range []string{"https://github.com/", "https://www.github.com/"} {
		if strings.HasPrefix(remote, pfx) {
			return pfx
		}
	}
	return remote
}

// getDefaultBranch gets the default branch of a repository using ls-remote
func getDefaultBranch(ctx context.Context, git *gitutil.GitCLI, remoteURL string) (string, error) {
	buf, err := git.Run(ctx, "ls-remote", "--symref", remoteURL, "HEAD")
	if err != nil {
		return "", errors.Wrapf(err, "error fetching default branch for repository %s", urlutil.RedactCredentials(remoteURL))
	}

	ss := defaultBranch.FindAllStringSubmatch(string(buf), -1)
	if len(ss) == 0 || len(ss[0]) != 2 {
		return "", errors.Errorf("could not find default branch for repository: %s", urlutil.RedactCredentials(remoteURL))
	}
	return ss[0][1], nil
}

const keyGitRemote = "git-remote"
const gitRemoteIndex = keyGitRemote + "::"
const keyGitSnapshot = "git-snapshot"
const gitSnapshotIndex = keyGitSnapshot + "::"

func search(ctx context.Context, store cache.MetadataStore, key string, idx string) ([]cacheRefMetadata, error) {
	var results []cacheRefMetadata
	mds, err := store.Search(ctx, idx+key)
	if err != nil {
		return nil, err
	}
	for _, md := range mds {
		results = append(results, cacheRefMetadata{md})
	}
	return results, nil
}

func searchGitRemote(ctx context.Context, store cache.MetadataStore, remote string) ([]cacheRefMetadata, error) {
	return search(ctx, store, remote, gitRemoteIndex)
}

func searchGitSnapshot(ctx context.Context, store cache.MetadataStore, key string) ([]cacheRefMetadata, error) {
	return search(ctx, store, key, gitSnapshotIndex)
}

type cacheRefMetadata struct {
	cache.RefMetadata
}

func (md cacheRefMetadata) setGitSnapshot(key string) error {
	return md.SetString(keyGitSnapshot, key, gitSnapshotIndex+key)
}

func (md cacheRefMetadata) setGitRemote(key string) error {
	return md.SetString(keyGitRemote, key, gitRemoteIndex+key)
}

func gitCLI(opts ...gitutil.Option) *gitutil.GitCLI {
	opts = append([]gitutil.Option{
		gitutil.WithExec(runWithStandardUmask),
		gitutil.WithStreams(func(ctx context.Context) (stdout, stderr io.WriteCloser, flush func()) {
			return logs.NewLogStreams(ctx, false)
		}),
	}, opts...)
	return gitutil.NewGitCLI(opts...)
}
