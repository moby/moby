package dockerfile

// internals for handling commands. Covers many areas and a lot of
// non-contiguous functionality. Please read the comments.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/pkg/errors"
)

func (b *Builder) commit(comment string) error {
	if b.disableCommit {
		return nil
	}
	if !b.hasFromImage() {
		return errors.New("Please provide a source image with `from` prior to commit")
	}
	// TODO: why is this set here?
	b.runConfig.Image = b.image

	runConfigWithCommentCmd := copyRunConfig(b.runConfig, withCmdComment(comment))
	hit, err := b.probeCache(b.image, runConfigWithCommentCmd)
	if err != nil || hit {
		return err
	}
	id, err := b.create(runConfigWithCommentCmd)
	if err != nil {
		return err
	}

	return b.commitContainer(id, b.runConfig)
}

func (b *Builder) commitContainer(id string, runConfig *container.Config) error {
	if b.disableCommit {
		return nil
	}

	commitCfg := &backend.ContainerCommitConfig{
		ContainerCommitConfig: types.ContainerCommitConfig{
			Author: b.maintainer,
			Pause:  true,
			Config: runConfig,
		},
	}

	// Commit the container
	imageID, err := b.docker.Commit(id, commitCfg)
	if err != nil {
		return err
	}

	// TODO: this function should return imageID and runConfig instead of setting
	// then on the builder
	b.image = imageID
	b.imageContexts.update(imageID, runConfig)
	return nil
}

type copyInfo struct {
	root       string
	path       string
	hash       string
	decompress bool
}

func (b *Builder) runContextCommand(args []string, allowRemote bool, allowLocalDecompression bool, cmdName string, imageSource *imageMount) error {
	if len(args) < 2 {
		return fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	// Work in daemon-specific filepath semantics
	dest := filepath.FromSlash(args[len(args)-1]) // last one is always the dest

	b.runConfig.Image = b.image

	var infos []copyInfo

	// Loop through each src file and calculate the info we need to
	// do the copy (e.g. hash value if cached).  Don't actually do
	// the copy until we've looked at all src files
	var err error
	for _, orig := range args[0 : len(args)-1] {
		if urlutil.IsURL(orig) {
			if !allowRemote {
				return fmt.Errorf("Source can't be a URL for %s", cmdName)
			}
			remote, path, err := b.download(orig)
			if err != nil {
				return err
			}
			defer os.RemoveAll(remote.Root())
			h, err := remote.Hash(path)
			if err != nil {
				return err
			}
			infos = append(infos, copyInfo{
				root: remote.Root(),
				path: path,
				hash: h,
			})
			continue
		}
		// not a URL
		subInfos, err := b.calcCopyInfo(cmdName, orig, allowLocalDecompression, true, imageSource)
		if err != nil {
			return err
		}

		infos = append(infos, subInfos...)
	}

	if len(infos) == 0 {
		return errors.New("No source files were specified")
	}
	if len(infos) > 1 && !strings.HasSuffix(dest, string(os.PathSeparator)) {
		return fmt.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}

	// For backwards compat, if there's just one info then use it as the
	// cache look-up string, otherwise hash 'em all into one
	var srcHash string

	if len(infos) == 1 {
		info := infos[0]
		srcHash = info.hash
	} else {
		var hashs []string
		var origs []string
		for _, info := range infos {
			origs = append(origs, info.path)
			hashs = append(hashs, info.hash)
		}
		hasher := sha256.New()
		hasher.Write([]byte(strings.Join(hashs, ",")))
		srcHash = "multi:" + hex.EncodeToString(hasher.Sum(nil))
	}

	cmd := b.runConfig.Cmd
	// TODO: should this have been using origPaths instead of srcHash in the comment?
	b.runConfig.Cmd = strslice.StrSlice(append(getShell(b.runConfig), fmt.Sprintf("#(nop) %s %s in %s ", cmdName, srcHash, dest)))
	defer func(cmd strslice.StrSlice) { b.runConfig.Cmd = cmd }(cmd)

	// TODO: this should pass a copy of runConfig
	if hit, err := b.probeCache(b.image, b.runConfig); err != nil || hit {
		return err
	}

	container, err := b.docker.ContainerCreate(types.ContainerCreateConfig{
		Config: b.runConfig,
		// Set a log config to override any default value set on the daemon
		HostConfig: &container.HostConfig{LogConfig: defaultLogConfig},
	})
	if err != nil {
		return err
	}
	b.tmpContainers[container.ID] = struct{}{}

	// Twiddle the destination when it's a relative path - meaning, make it
	// relative to the WORKINGDIR
	if dest, err = normaliseDest(cmdName, b.runConfig.WorkingDir, dest); err != nil {
		return err
	}

	for _, info := range infos {
		if err := b.docker.CopyOnBuild(container.ID, dest, info.root, info.path, info.decompress); err != nil {
			return err
		}
	}

	return b.commitContainer(container.ID, copyRunConfig(b.runConfig, withCmd(cmd)))
}

type runConfigModifier func(*container.Config)

func copyRunConfig(runConfig *container.Config, modifiers ...runConfigModifier) *container.Config {
	copy := *runConfig
	for _, modifier := range modifiers {
		modifier(&copy)
	}
	return &copy
}

func withCmd(cmd []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = cmd
	}
}

func withCmdComment(comment string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Cmd = append(getShell(runConfig), "#(nop) ", comment)
	}
}

func withEnv(env []string) runConfigModifier {
	return func(runConfig *container.Config) {
		runConfig.Env = env
	}
}

// getShell is a helper function which gets the right shell for prefixing the
// shell-form of RUN, ENTRYPOINT and CMD instructions
func getShell(c *container.Config) []string {
	if 0 == len(c.Shell) {
		return append([]string{}, defaultShell[:]...)
	}
	return append([]string{}, c.Shell[:]...)
}

func (b *Builder) download(srcURL string) (remote builder.Source, p string, err error) {
	// get filename from URL
	u, err := url.Parse(srcURL)
	if err != nil {
		return
	}
	path := filepath.FromSlash(u.Path) // Ensure in platform semantics
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		path = path[:len(path)-1]
	}
	parts := strings.Split(path, string(os.PathSeparator))
	filename := parts[len(parts)-1]
	if filename == "" {
		err = fmt.Errorf("cannot determine filename from url: %s", u)
		return
	}

	// Initiate the download
	resp, err := httputils.Download(srcURL)
	if err != nil {
		return
	}

	// Prepare file in a tmp dir
	tmpDir, err := ioutils.TempDir("", "docker-remote")
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir)
		}
	}()
	tmpFileName := filepath.Join(tmpDir, filename)
	tmpFile, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return
	}

	stdoutFormatter := b.Stdout.(*streamformatter.StdoutFormatter)
	progressOutput := stdoutFormatter.StreamFormatter.NewProgressOutput(stdoutFormatter.Writer, true)
	progressReader := progress.NewProgressReader(resp.Body, progressOutput, resp.ContentLength, "", "Downloading")
	// Download and dump result to tmp file
	// TODO: add filehash directly
	if _, err = io.Copy(tmpFile, progressReader); err != nil {
		tmpFile.Close()
		return
	}
	fmt.Fprintln(b.Stdout)

	// Set the mtime to the Last-Modified header value if present
	// Otherwise just remove atime and mtime
	mTime := time.Time{}

	lastMod := resp.Header.Get("Last-Modified")
	if lastMod != "" {
		// If we can't parse it then just let it default to 'zero'
		// otherwise use the parsed time value
		if parsedMTime, err := http.ParseTime(lastMod); err == nil {
			mTime = parsedMTime
		}
	}

	tmpFile.Close()

	if err = system.Chtimes(tmpFileName, mTime, mTime); err != nil {
		return
	}

	lc, err := remotecontext.NewLazyContext(tmpDir)
	if err != nil {
		return
	}

	return lc, filename, nil
}

var windowsBlacklist = map[string]bool{
	"c:\\":        true,
	"c:\\windows": true,
}

func (b *Builder) calcCopyInfo(cmdName, origPath string, allowLocalDecompression, allowWildcards bool, imageSource *imageMount) ([]copyInfo, error) {

	// Work in daemon-specific OS filepath semantics
	origPath = filepath.FromSlash(origPath)
	// validate windows paths from other images
	if imageSource != nil && runtime.GOOS == "windows" {
		p := strings.ToLower(filepath.Clean(origPath))
		if !filepath.IsAbs(p) {
			if filepath.VolumeName(p) != "" {
				if p[len(p)-2:] == ":." { // case where clean returns weird c:. paths
					p = p[:len(p)-1]
				}
				p += "\\"
			} else {
				p = filepath.Join("c:\\", p)
			}
		}
		if _, blacklisted := windowsBlacklist[p]; blacklisted {
			return nil, errors.New("copy from c:\\ or c:\\windows is not allowed on windows")
		}
	}

	if origPath != "" && origPath[0] == os.PathSeparator && len(origPath) > 1 {
		origPath = origPath[1:]
	}
	origPath = strings.TrimPrefix(origPath, "."+string(os.PathSeparator))

	source := b.source
	var err error
	if imageSource != nil {
		source, err = imageSource.context()
		if err != nil {
			return nil, err
		}
	}

	if source == nil {
		return nil, errors.Errorf("No context given. Impossible to use %s", cmdName)
	}

	// Deal with wildcards
	if allowWildcards && containsWildcards(origPath) {
		var copyInfos []copyInfo
		if err := filepath.Walk(source.Root(), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := remotecontext.Rel(source.Root(), path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			if match, _ := filepath.Match(origPath, rel); !match {
				return nil
			}

			// Note we set allowWildcards to false in case the name has
			// a * in it
			subInfos, err := b.calcCopyInfo(cmdName, rel, allowLocalDecompression, false, imageSource)
			if err != nil {
				return err
			}
			copyInfos = append(copyInfos, subInfos...)
			return nil
		}); err != nil {
			return nil, err
		}
		return copyInfos, nil
	}

	// Must be a dir or a file
	hash, err := source.Hash(origPath)
	if err != nil {
		return nil, err
	}

	fi, err := remotecontext.StatAt(source, origPath)
	if err != nil {
		return nil, err
	}

	// TODO: remove, handle dirs in Hash()
	copyInfos := []copyInfo{{root: source.Root(), path: origPath, hash: hash, decompress: allowLocalDecompression}}

	if imageSource != nil {
		// fast-cache based on imageID
		if h, ok := b.imageContexts.getCache(imageSource.id, origPath); ok {
			copyInfos[0].hash = h.(string)
			return copyInfos, nil
		}
	}

	// Deal with the single file case
	if !fi.IsDir() {
		copyInfos[0].hash = "file:" + copyInfos[0].hash
		return copyInfos, nil
	}

	fp, err := remotecontext.FullPath(source, origPath)
	if err != nil {
		return nil, err
	}
	// Must be a dir
	var subfiles []string
	err = filepath.Walk(fp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := remotecontext.Rel(source.Root(), path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hash, err := source.Hash(rel)
		if err != nil {
			return nil
		}
		// we already checked handleHash above
		subfiles = append(subfiles, hash)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(subfiles)
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(subfiles, ",")))
	copyInfos[0].hash = "dir:" + hex.EncodeToString(hasher.Sum(nil))
	if imageSource != nil {
		b.imageContexts.setCache(imageSource.id, origPath, copyInfos[0].hash)
	}

	return copyInfos, nil
}

func (b *Builder) processImageFrom(img builder.Image) error {
	if img != nil {
		b.image = img.ImageID()

		if img.RunConfig() != nil {
			b.runConfig = img.RunConfig()
		}
	}

	// Check to see if we have a default PATH, note that windows won't
	// have one as it's set by HCS
	if system.DefaultPathEnv != "" {
		if _, ok := b.runConfigEnvMapping()["PATH"]; !ok {
			b.runConfig.Env = append(b.runConfig.Env,
				"PATH="+system.DefaultPathEnv)
		}
	}

	if img == nil {
		// Typically this means they used "FROM scratch"
		return nil
	}

	// Process ONBUILD triggers if they exist
	if nTriggers := len(b.runConfig.OnBuild); nTriggers != 0 {
		word := "trigger"
		if nTriggers > 1 {
			word = "triggers"
		}
		fmt.Fprintf(b.Stderr, "# Executing %d build %s...\n", nTriggers, word)
	}

	// Copy the ONBUILD triggers, and remove them from the config, since the config will be committed.
	onBuildTriggers := b.runConfig.OnBuild
	b.runConfig.OnBuild = []string{}

	// Reset stdin settings as all build actions run without stdin
	b.runConfig.OpenStdin = false
	b.runConfig.StdinOnce = false

	// parse the ONBUILD triggers by invoking the parser
	for _, step := range onBuildTriggers {
		result, err := parser.Parse(strings.NewReader(step))
		if err != nil {
			return err
		}

		for _, n := range result.AST.Children {
			if err := checkDispatch(n); err != nil {
				return err
			}

			upperCasedCmd := strings.ToUpper(n.Value)
			switch upperCasedCmd {
			case "ONBUILD":
				return errors.New("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
			case "MAINTAINER", "FROM":
				return errors.Errorf("%s isn't allowed as an ONBUILD trigger", upperCasedCmd)
			}
		}

		if err := dispatchFromDockerfile(b, result); err != nil {
			return err
		}
	}
	return nil
}

// probeCache checks if cache match can be found for current build instruction.
// If an image is found, probeCache returns `(true, nil)`.
// If no image is found, it returns `(false, nil)`.
// If there is any error, it returns `(false, err)`.
func (b *Builder) probeCache(imageID string, runConfig *container.Config) (bool, error) {
	c := b.imageCache
	if c == nil || b.options.NoCache || b.cacheBusted {
		return false, nil
	}
	cache, err := c.GetCache(imageID, runConfig)
	if err != nil {
		return false, err
	}
	if len(cache) == 0 {
		logrus.Debugf("[BUILDER] Cache miss: %s", runConfig.Cmd)
		b.cacheBusted = true
		return false, nil
	}

	fmt.Fprint(b.Stdout, " ---> Using cache\n")
	logrus.Debugf("[BUILDER] Use cached version: %s", runConfig.Cmd)
	b.image = string(cache)
	b.imageContexts.update(b.image, runConfig)

	return true, nil
}

func (b *Builder) create(runConfig *container.Config) (string, error) {
	if !b.hasFromImage() {
		return "", errors.New("Please provide a source image with `from` prior to run")
	}
	resources := container.Resources{
		CgroupParent: b.options.CgroupParent,
		CPUShares:    b.options.CPUShares,
		CPUPeriod:    b.options.CPUPeriod,
		CPUQuota:     b.options.CPUQuota,
		CpusetCpus:   b.options.CPUSetCPUs,
		CpusetMems:   b.options.CPUSetMems,
		Memory:       b.options.Memory,
		MemorySwap:   b.options.MemorySwap,
		Ulimits:      b.options.Ulimits,
	}

	// TODO: why not embed a hostconfig in builder?
	hostConfig := &container.HostConfig{
		SecurityOpt: b.options.SecurityOpt,
		Isolation:   b.options.Isolation,
		ShmSize:     b.options.ShmSize,
		Resources:   resources,
		NetworkMode: container.NetworkMode(b.options.NetworkMode),
		// Set a log config to override any default value set on the daemon
		LogConfig:  defaultLogConfig,
		ExtraHosts: b.options.ExtraHosts,
	}

	// Create the container
	c, err := b.docker.ContainerCreate(types.ContainerCreateConfig{
		Config:     runConfig,
		HostConfig: hostConfig,
	})
	if err != nil {
		return "", err
	}
	for _, warning := range c.Warnings {
		fmt.Fprintf(b.Stdout, " ---> [Warning] %s\n", warning)
	}

	b.tmpContainers[c.ID] = struct{}{}
	fmt.Fprintf(b.Stdout, " ---> Running in %s\n", stringid.TruncateID(c.ID))

	// override the entry point that may have been picked up from the base image
	if err := b.docker.ContainerUpdateCmdOnBuild(c.ID, runConfig.Cmd); err != nil {
		return "", err
	}

	return c.ID, nil
}

var errCancelled = errors.New("build cancelled")

func (b *Builder) run(cID string) (err error) {
	attached := make(chan struct{})
	errCh := make(chan error)
	go func() {
		errCh <- b.docker.ContainerAttachRaw(cID, nil, b.Stdout, b.Stderr, true, attached)
	}()

	select {
	case err := <-errCh:
		return err
	case <-attached:
	}

	finished := make(chan struct{})
	cancelErrCh := make(chan error, 1)
	go func() {
		select {
		case <-b.clientCtx.Done():
			logrus.Debugln("Build cancelled, killing and removing container:", cID)
			b.docker.ContainerKill(cID, 0)
			b.removeContainer(cID)
			cancelErrCh <- errCancelled
		case <-finished:
			cancelErrCh <- nil
		}
	}()

	if err := b.docker.ContainerStart(cID, nil, "", ""); err != nil {
		close(finished)
		if cancelErr := <-cancelErrCh; cancelErr != nil {
			logrus.Debugf("Build cancelled (%v) and got an error from ContainerStart: %v",
				cancelErr, err)
		}
		return err
	}

	// Block on reading output from container, stop on err or chan closed
	if err := <-errCh; err != nil {
		close(finished)
		if cancelErr := <-cancelErrCh; cancelErr != nil {
			logrus.Debugf("Build cancelled (%v) and got an error from errCh: %v",
				cancelErr, err)
		}
		return err
	}

	if ret, _ := b.docker.ContainerWait(cID, -1); ret != 0 {
		close(finished)
		if cancelErr := <-cancelErrCh; cancelErr != nil {
			logrus.Debugf("Build cancelled (%v) and got a non-zero code from ContainerWait: %d",
				cancelErr, ret)
		}
		// TODO: change error type, because jsonmessage.JSONError assumes HTTP
		return &jsonmessage.JSONError{
			Message: fmt.Sprintf("The command '%s' returned a non-zero code: %d", strings.Join(b.runConfig.Cmd, " "), ret),
			Code:    ret,
		}
	}
	close(finished)
	return <-cancelErrCh
}

func (b *Builder) removeContainer(c string) error {
	rmConfig := &types.ContainerRmConfig{
		ForceRemove:  true,
		RemoveVolume: true,
	}
	if err := b.docker.ContainerRm(c, rmConfig); err != nil {
		fmt.Fprintf(b.Stdout, "Error removing intermediate container %s: %v\n", stringid.TruncateID(c), err)
		return err
	}
	return nil
}

func (b *Builder) clearTmp() {
	for c := range b.tmpContainers {
		if err := b.removeContainer(c); err != nil {
			return
		}
		delete(b.tmpContainers, c)
		fmt.Fprintf(b.Stdout, "Removing intermediate container %s\n", stringid.TruncateID(c))
	}
}
