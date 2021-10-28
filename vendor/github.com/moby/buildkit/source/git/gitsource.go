package git

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/moby/buildkit/source"
	srctypes "github.com/moby/buildkit/source/types"
	"github.com/moby/buildkit/util/bklog"
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

func (gs *gitSource) ID() string {
	return srctypes.GitScheme
}

// needs to be called with repo lock
func (gs *gitSource) mountRemote(ctx context.Context, remote string, auth []string, g session.Group) (target string, release func(), retErr error) {
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

	releaseRemoteRef := func() {
		remoteRef.Release(context.TODO())
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

	if initializeRepo {
		if _, err := gitWithinDir(ctx, dir, "", "", "", auth, "init", "--bare"); err != nil {
			return "", nil, errors.Wrapf(err, "failed to init repo at %s", dir)
		}

		if _, err := gitWithinDir(ctx, dir, "", "", "", auth, "remote", "add", "origin", remote); err != nil {
			return "", nil, errors.Wrapf(err, "failed add origin repo at %s", dir)
		}

		// save new remote metadata
		md := cacheRefMetadata{remoteRef}
		if err := md.setGitRemote(remote); err != nil {
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
	if gs.src.Subdir != "" {
		key += ":" + gs.src.Subdir
	}
	return key
}

func (gs *gitSource) Resolve(ctx context.Context, id source.Identifier, sm *session.Manager, _ solver.Vertex) (source.SourceInstance, error) {
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
			gs.auth = []string{"-c", "http." + tokenScope(gs.src.Remote) + ".extraheader=Authorization: " + string(dt)}
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

func (gs *gitSourceHandler) mountKnownHosts(ctx context.Context) (string, func() error, error) {
	if gs.src.KnownSSHHosts == "" {
		return "", nil, errors.Errorf("no configured known hosts forwarded from the client")
	}
	knownHosts, err := ioutil.TempFile("", "")
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

	gitDir, unmountGitDir, err := gs.mountRemote(ctx, remote, gs.auth, g)
	if err != nil {
		return "", "", nil, false, err
	}
	defer unmountGitDir()

	var sock string
	if gs.src.MountSSHSock != "" {
		var unmountSock func() error
		sock, unmountSock, err = gs.mountSSHAuthSock(ctx, gs.src.MountSSHSock, g)
		if err != nil {
			return "", "", nil, false, err
		}
		defer unmountSock()
	}

	var knownHosts string
	if gs.src.KnownSSHHosts != "" {
		var unmountKnownHosts func() error
		knownHosts, unmountKnownHosts, err = gs.mountKnownHosts(ctx)
		if err != nil {
			return "", "", nil, false, err
		}
		defer unmountKnownHosts()
	}

	ref := gs.src.Ref
	if ref == "" {
		ref, err = getDefaultBranch(ctx, gitDir, "", sock, knownHosts, gs.auth, gs.src.Remote)
		if err != nil {
			return "", "", nil, false, err
		}
	}

	// TODO: should we assume that remote tag is immutable? add a timer?

	buf, err := gitWithinDir(ctx, gitDir, "", sock, knownHosts, gs.auth, "ls-remote", "origin", ref)
	if err != nil {
		return "", "", nil, false, errors.Wrapf(err, "failed to fetch remote %s", urlutil.RedactCredentials(remote))
	}
	out := buf.String()
	idx := strings.Index(out, "\t")
	if idx == -1 {
		return "", "", nil, false, errors.Errorf("repository does not contain ref %s, output: %q", ref, string(out))
	}

	sha := string(out[:idx])
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
		return gs.cache.Get(ctx, sis[0].ID())
	}

	gs.locker.Lock(gs.src.Remote)
	defer gs.locker.Unlock(gs.src.Remote)
	gitDir, unmountGitDir, err := gs.mountRemote(ctx, gs.src.Remote, gs.auth, g)
	if err != nil {
		return nil, err
	}
	defer unmountGitDir()

	var sock string
	if gs.src.MountSSHSock != "" {
		var unmountSock func() error
		sock, unmountSock, err = gs.mountSSHAuthSock(ctx, gs.src.MountSSHSock, g)
		if err != nil {
			return nil, err
		}
		defer unmountSock()
	}

	var knownHosts string
	if gs.src.KnownSSHHosts != "" {
		var unmountKnownHosts func() error
		knownHosts, unmountKnownHosts, err = gs.mountKnownHosts(ctx)
		if err != nil {
			return nil, err
		}
		defer unmountKnownHosts()
	}

	ref := gs.src.Ref
	if ref == "" {
		ref, err = getDefaultBranch(ctx, gitDir, "", sock, knownHosts, gs.auth, gs.src.Remote)
		if err != nil {
			return nil, err
		}
	}

	doFetch := true
	if isCommitSHA(ref) {
		// skip fetch if commit already exists
		if _, err := gitWithinDir(ctx, gitDir, "", sock, knownHosts, nil, "cat-file", "-e", ref+"^{commit}"); err == nil {
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
		if _, err := gitWithinDir(ctx, gitDir, "", sock, knownHosts, gs.auth, args...); err != nil {
			return nil, errors.Wrapf(err, "failed to fetch remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
	}

	checkoutRef, err := gs.cache.New(ctx, nil, g, cache.WithRecordType(client.UsageRecordTypeGitCheckout), cache.WithDescription(fmt.Sprintf("git snapshot for %s#%s", gs.src.Remote, ref)))
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
		_, err = gitWithinDir(ctx, checkoutDirGit, "", sock, knownHosts, nil, "init")
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, "", sock, knownHosts, nil, "remote", "add", "origin", gitDir)
		if err != nil {
			return nil, err
		}
		pullref := ref
		if isCommitSHA(ref) {
			pullref = "refs/buildkit/" + identity.NewID()
			_, err = gitWithinDir(ctx, gitDir, "", sock, knownHosts, gs.auth, "update-ref", pullref, ref)
			if err != nil {
				return nil, err
			}
		} else {
			pullref += ":" + pullref
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, "", sock, knownHosts, gs.auth, "fetch", "-u", "--depth=1", "origin", pullref)
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDirGit, checkoutDir, sock, knownHosts, nil, "checkout", "FETCH_HEAD")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", urlutil.RedactCredentials(gs.src.Remote))
		}
		gitDir = checkoutDirGit
	} else {
		cd := checkoutDir
		if subdir != "." {
			cd, err = ioutil.TempDir(cd, "checkout")
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create temporary checkout dir")
			}
		}
		_, err = gitWithinDir(ctx, gitDir, cd, sock, knownHosts, nil, "checkout", ref, "--", ".")
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

	_, err = gitWithinDir(ctx, gitDir, checkoutDir, sock, knownHosts, gs.auth, "submodule", "update", "--init", "--recursive", "--depth=1")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update submodules for %s", urlutil.RedactCredentials(gs.src.Remote))
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

	md := cacheRefMetadata{snap}
	if err := md.setGitSnapshot(snapshotKey); err != nil {
		return nil, err
	}
	return snap, nil
}

func isCommitSHA(str string) bool {
	return validHex.MatchString(str)
}

func gitWithinDir(ctx context.Context, gitDir, workDir, sshAuthSock, knownHosts string, auth []string, args ...string) (*bytes.Buffer, error) {
	a := append([]string{"--git-dir", gitDir}, auth...)
	if workDir != "" {
		a = append(a, "--work-tree", workDir)
	}
	return git(ctx, workDir, sshAuthSock, knownHosts, append(a, args...)...)
}

func getGitSSHCommand(knownHosts string) string {
	gitSSHCommand := "ssh -F /dev/null"
	if knownHosts != "" {
		gitSSHCommand += " -o UserKnownHostsFile=" + knownHosts
	} else {
		gitSSHCommand += " -o StrictHostKeyChecking=no"
	}
	return gitSSHCommand
}

func git(ctx context.Context, dir, sshAuthSock, knownHosts string, args ...string) (*bytes.Buffer, error) {
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
			"GIT_SSH_COMMAND=" + getGitSSHCommand(knownHosts),
			//	"GIT_TRACE=1",
		}
		if sshAuthSock != "" {
			cmd.Env = append(cmd.Env, "SSH_AUTH_SOCK="+sshAuthSock)
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
func getDefaultBranch(ctx context.Context, gitDir, workDir, sshAuthSock, knownHosts string, auth []string, remoteURL string) (string, error) {
	buf, err := gitWithinDir(ctx, gitDir, workDir, sshAuthSock, knownHosts, auth, "ls-remote", "--symref", remoteURL, "HEAD")
	if err != nil {
		return "", errors.Wrapf(err, "error fetching default branch for repository %s", urlutil.RedactCredentials(remoteURL))
	}

	ss := defaultBranch.FindAllStringSubmatch(buf.String(), -1)
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
