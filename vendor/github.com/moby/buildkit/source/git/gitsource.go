package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/locker"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/secrets"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/progress/logs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

var validHex = regexp.MustCompile(`^[a-f0-9]{40}$`)

type Opt struct {
	CacheAccessor cache.Accessor
	MetadataStore *metadata.Store
}

type gitSource struct {
	md     *metadata.Store
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
		md:     opt.MetadataStore,
		cache:  opt.CacheAccessor,
		locker: locker.New(),
	}
	return gs, nil
}

func (gs *gitSource) ID() string {
	return source.GitScheme
}

// needs to be called with repo lock
func (gs *gitSource) mountRemote(ctx context.Context, remote string, auth []string) (target string, release func(), retErr error) {
	remoteKey := "git-remote::" + remote

	sis, err := gs.md.Search(remoteKey)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to search metadata for %s", remote)
	}

	var remoteRef cache.MutableRef
	for _, si := range sis {
		remoteRef, err = gs.cache.GetMutable(ctx, si.ID())
		if err != nil {
			if errors.Is(err, cache.ErrLocked) {
				// should never really happen as no other function should access this metadata, but lets be graceful
				logrus.Warnf("mutable ref for %s  %s was locked: %v", remote, si.ID(), err)
				continue
			}
			return "", nil, errors.Wrapf(err, "failed to get mutable ref for %s", remote)
		}
		break
	}

	initializeRepo := false
	if remoteRef == nil {
		remoteRef, err = gs.cache.New(ctx, nil, cache.CachePolicyRetain, cache.WithDescription(fmt.Sprintf("shared git repo for %s", remote)))
		if err != nil {
			return "", nil, errors.Wrapf(err, "failed to create new mutable for %s", remote)
		}
		initializeRepo = true
	}

	releaseRemoteRef := func() {
		remoteRef.Release(context.TODO())
	}

	defer func() {
		if retErr != nil && remoteRef != nil {
			releaseRemoteRef()
		}
	}()

	mount, err := remoteRef.Mount(ctx, false)
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

	if initializeRepo {
		if _, err := gitWithinDir(ctx, dir, "", auth, "init", "--bare"); err != nil {
			return "", nil, errors.Wrapf(err, "failed to init repo at %s", dir)
		}

		if _, err := gitWithinDir(ctx, dir, "", auth, "remote", "add", "origin", remote); err != nil {
			return "", nil, errors.Wrapf(err, "failed add origin repo at %s", dir)
		}

		// same new remote metadata
		si, _ := gs.md.Get(remoteRef.ID())
		v, err := metadata.NewValue(remoteKey)
		v.Index = remoteKey
		if err != nil {
			return "", nil, err
		}

		if err := si.Update(func(b *bolt.Bucket) error {
			return si.SetValue(b, "git-remote", v)
		}); err != nil {
			return "", nil, err
		}
	}
	return dir, func() {
		lm.Unmount()
		releaseRemoteRef()
	}, nil
}

type gitSourceHandler struct {
	*gitSource
	src      source.GitIdentifier
	cacheKey string
	sm       *session.Manager
	auth     []string
}

func (gs *gitSourceHandler) shaToCacheKey(sha string) string {
	key := sha
	if gs.src.KeepGitDir {
		key += ".git"
	}
	return key
}

func (gs *gitSource) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager) (source.SourceInstance, error) {
	gitIdentifier, ok := id.(*source.GitIdentifier)
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
	if gs.auth != nil {
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
			gs.auth = []string{"-c", "http.extraheader=Authorization: " + string(dt)}
			break
		}
		return nil
	})
}

func (gs *gitSourceHandler) CacheKey(ctx context.Context, g session.Group, index int) (string, bool, error) {
	remote := gs.src.Remote
	ref := gs.src.Ref
	if ref == "" {
		ref = "master"
	}
	gs.locker.Lock(remote)
	defer gs.locker.Unlock(remote)

	if isCommitSHA(ref) {
		ref = gs.shaToCacheKey(ref)
		gs.cacheKey = ref
		return ref, true, nil
	}

	gs.getAuthToken(ctx, g)

	gitDir, unmountGitDir, err := gs.mountRemote(ctx, remote, gs.auth)
	if err != nil {
		return "", false, err
	}
	defer unmountGitDir()

	// TODO: should we assume that remote tag is immutable? add a timer?

	buf, err := gitWithinDir(ctx, gitDir, "", gs.auth, "ls-remote", "origin", ref)
	if err != nil {
		return "", false, errors.Wrapf(err, "failed to fetch remote %s", remote)
	}
	out := buf.String()
	idx := strings.Index(out, "\t")
	if idx == -1 {
		return "", false, errors.Errorf("repository does not contain ref %s, output: %q", ref, string(out))
	}

	sha := string(out[:idx])
	if !isCommitSHA(sha) {
		return "", false, errors.Errorf("invalid commit sha %q", sha)
	}
	sha = gs.shaToCacheKey(sha)
	gs.cacheKey = sha
	return sha, true, nil
}

func (gs *gitSourceHandler) Snapshot(ctx context.Context, g session.Group) (out cache.ImmutableRef, retErr error) {
	ref := gs.src.Ref
	if ref == "" {
		ref = "master"
	}

	cacheKey := gs.cacheKey
	if cacheKey == "" {
		var err error
		cacheKey, _, err = gs.CacheKey(ctx, g, 0)
		if err != nil {
			return nil, err
		}
	}

	gs.getAuthToken(ctx, g)

	snapshotKey := "git-snapshot::" + cacheKey + ":" + gs.src.Subdir
	gs.locker.Lock(snapshotKey)
	defer gs.locker.Unlock(snapshotKey)

	sis, err := gs.md.Search(snapshotKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search metadata for %s", snapshotKey)
	}
	if len(sis) > 0 {
		return gs.cache.Get(ctx, sis[0].ID())
	}

	gs.locker.Lock(gs.src.Remote)
	defer gs.locker.Unlock(gs.src.Remote)
	gitDir, unmountGitDir, err := gs.mountRemote(ctx, gs.src.Remote, gs.auth)
	if err != nil {
		return nil, err
	}
	defer unmountGitDir()

	doFetch := true
	if isCommitSHA(ref) {
		// skip fetch if commit already exists
		if _, err := gitWithinDir(ctx, gitDir, "", nil, "cat-file", "-e", ref+"^{commit}"); err == nil {
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
		if _, err := gitWithinDir(ctx, gitDir, "", gs.auth, args...); err != nil {
			return nil, errors.Wrapf(err, "failed to fetch remote %s", gs.src.Remote)
		}
	}

	checkoutRef, err := gs.cache.New(ctx, nil, cache.WithRecordType(client.UsageRecordTypeGitCheckout), cache.WithDescription(fmt.Sprintf("git snapshot for %s#%s", gs.src.Remote, ref)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new mutable for %s", gs.src.Remote)
	}

	defer func() {
		if retErr != nil && checkoutRef != nil {
			checkoutRef.Release(context.TODO())
		}
	}()

	mount, err := checkoutRef.Mount(ctx, false)
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

	if gs.src.KeepGitDir {
		checkoutDirGit := filepath.Join(checkoutDir, ".git")
		if err := os.MkdirAll(checkoutDir, 0711); err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, "", nil, "init")
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, "", nil, "remote", "add", "origin", gitDir)
		if err != nil {
			return nil, err
		}
		pullref := ref
		if isCommitSHA(ref) {
			pullref = "refs/buildkit/" + identity.NewID()
			_, err = gitWithinDir(ctx, gitDir, "", gs.auth, "update-ref", pullref, ref)
			if err != nil {
				return nil, err
			}
		} else {
			pullref += ":" + pullref
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, "", gs.auth, "fetch", "-u", "--depth=1", "origin", pullref)
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, checkoutDir, nil, "checkout", "FETCH_HEAD")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", gs.src.Remote)
		}
		gitDir = checkoutDirGit
	} else {
		_, err = gitWithinDir(ctx, gitDir, checkoutDir, nil, "checkout", ref, "--", ".")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", gs.src.Remote)
		}
	}

	_, err = gitWithinDir(ctx, gitDir, checkoutDir, gs.auth, "submodule", "update", "--init", "--recursive", "--depth=1")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update submodules for %s", gs.src.Remote)
	}

	if idmap := mount.IdentityMapping(); idmap != nil {
		u := idmap.RootPair()
		err := filepath.Walk(gitDir, func(p string, f os.FileInfo, err error) error {
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

	si, _ := gs.md.Get(snap.ID())
	v, err := metadata.NewValue(snapshotKey)
	v.Index = snapshotKey
	if err != nil {
		return nil, err
	}
	if err := si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, "git-snapshot", v)
	}); err != nil {
		return nil, err
	}

	return snap, nil
}

func isCommitSHA(str string) bool {
	return validHex.MatchString(str)
}

func gitWithinDir(ctx context.Context, gitDir, workDir string, auth []string, args ...string) (*bytes.Buffer, error) {
	a := append([]string{"--git-dir", gitDir}, auth...)
	if workDir != "" {
		a = append(a, "--work-tree", workDir)
	}
	return git(ctx, workDir, append(a, args...)...)
}

func git(ctx context.Context, dir string, args ...string) (*bytes.Buffer, error) {
	for {
		stdout, stderr := logs.NewLogStreams(ctx, false)
		defer stdout.Close()
		defer stderr.Close()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir // some commands like submodule require this
		buf := bytes.NewBuffer(nil)
		errbuf := bytes.NewBuffer(nil)
		cmd.Stdin = nil
		cmd.Stdout = io.MultiWriter(stdout, buf)
		cmd.Stderr = io.MultiWriter(stderr, errbuf)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"GIT_TERMINAL_PROMPT=0",
			//	"GIT_TRACE=1",
		}
		// remote git commands spawn helper processes that inherit FDs and don't
		// handle parent death signal so exec.CommandContext can't be used
		err := runProcessGroup(ctx, cmd)
		if err != nil {
			if strings.Contains(errbuf.String(), "--depth") || strings.Contains(errbuf.String(), "shallow") {
				if newArgs := argsNoDepth(args); len(args) > len(newArgs) {
					args = newArgs
					continue
				}
			}
		}
		return buf, err
	}
}

func argsNoDepth(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a != "--depth=1" {
			out = append(out, a)
		}
	}
	return out
}
