package builder

// internals for handling commands. Covers many areas and a lot of
// non-contiguous functionality. Please read the comments.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/builder/parser"
	"github.com/docker/docker/daemon"
	imagepkg "github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

func (b *Builder) readContext(context io.Reader) error {
	tmpdirPath, err := ioutil.TempDir("", "docker-build")
	if err != nil {
		return err
	}

	decompressedStream, err := archive.DecompressStream(context)
	if err != nil {
		return err
	}

	if b.context, err = tarsum.NewTarSum(decompressedStream, true, tarsum.Version0); err != nil {
		return err
	}

	if err := chrootarchive.Untar(b.context, tmpdirPath, nil); err != nil {
		return err
	}

	b.contextPath = tmpdirPath
	return nil
}

func (b *Builder) commit(id string, autoCmd []string, comment string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to commit")
	}
	b.Config.Image = b.image
	if id == "" {
		cmd := b.Config.Cmd
		b.Config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + comment}
		defer func(cmd []string) { b.Config.Cmd = cmd }(cmd)

		hit, err := b.probeCache()
		if err != nil {
			return err
		}
		if hit {
			return nil
		}

		container, err := b.create()
		if err != nil {
			return err
		}
		id = container.ID

		if err := container.Mount(); err != nil {
			return err
		}
		defer container.Unmount()
	}
	container := b.Daemon.Get(id)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	// Note: Actually copy the struct
	autoConfig := *b.Config
	autoConfig.Cmd = autoCmd

	// Commit the container
	image, err := b.Daemon.Commit(container, "", "", "", b.maintainer, true, &autoConfig)
	if err != nil {
		return err
	}
	b.image = image.ID
	return nil
}

type copyInfo struct {
	origPath   string
	destPath   string
	hash       string
	decompress bool
	tmpDir     string
}

func (b *Builder) runContextCommand(args []string, allowRemote bool, allowDecompression bool, cmdName string) error {
	if b.context == nil {
		return fmt.Errorf("No context given. Impossible to use %s", cmdName)
	}

	if len(args) < 2 {
		return fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	dest := args[len(args)-1] // last one is always the dest

	copyInfos := []*copyInfo{}

	b.Config.Image = b.image

	defer func() {
		for _, ci := range copyInfos {
			if ci.tmpDir != "" {
				os.RemoveAll(ci.tmpDir)
			}
		}
	}()

	// Loop through each src file and calculate the info we need to
	// do the copy (e.g. hash value if cached).  Don't actually do
	// the copy until we've looked at all src files
	for _, orig := range args[0 : len(args)-1] {
		err := calcCopyInfo(b, cmdName, &copyInfos, orig, dest, allowRemote, allowDecompression)
		if err != nil {
			return err
		}
	}

	if len(copyInfos) == 0 {
		return fmt.Errorf("No source files were specified")
	}

	if len(copyInfos) > 1 && !strings.HasSuffix(dest, "/") {
		return fmt.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}

	// For backwards compat, if there's just one CI then use it as the
	// cache look-up string, otherwise hash 'em all into one
	var srcHash string
	var origPaths string

	if len(copyInfos) == 1 {
		srcHash = copyInfos[0].hash
		origPaths = copyInfos[0].origPath
	} else {
		var hashs []string
		var origs []string
		for _, ci := range copyInfos {
			hashs = append(hashs, ci.hash)
			origs = append(origs, ci.origPath)
		}
		hasher := sha256.New()
		hasher.Write([]byte(strings.Join(hashs, ",")))
		srcHash = "multi:" + hex.EncodeToString(hasher.Sum(nil))
		origPaths = strings.Join(origs, " ")
	}

	cmd := b.Config.Cmd
	b.Config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) %s %s in %s", cmdName, srcHash, dest)}
	defer func(cmd []string) { b.Config.Cmd = cmd }(cmd)

	hit, err := b.probeCache()
	if err != nil {
		return err
	}
	// If we do not have at least one hash, never use the cache
	if hit && b.UtilizeCache {
		return nil
	}

	container, _, err := b.Daemon.Create(b.Config, nil, "")
	if err != nil {
		return err
	}
	b.TmpContainers[container.ID] = struct{}{}

	if err := container.Mount(); err != nil {
		return err
	}
	defer container.Unmount()

	for _, ci := range copyInfos {
		if err := b.addContext(container, ci.origPath, ci.destPath, ci.decompress); err != nil {
			return err
		}
	}

	if err := b.commit(container.ID, cmd, fmt.Sprintf("%s %s in %s", cmdName, origPaths, dest)); err != nil {
		return err
	}
	return nil
}

func calcCopyInfo(b *Builder, cmdName string, cInfos *[]*copyInfo, origPath string, destPath string, allowRemote bool, allowDecompression bool) error {

	if origPath != "" && origPath[0] == '/' && len(origPath) > 1 {
		origPath = origPath[1:]
	}
	origPath = strings.TrimPrefix(origPath, "./")

	// In the remote/URL case, download it and gen its hashcode
	if urlutil.IsURL(origPath) {
		if !allowRemote {
			return fmt.Errorf("Source can't be a URL for %s", cmdName)
		}

		ci := copyInfo{}
		ci.origPath = origPath
		ci.hash = origPath // default to this but can change
		ci.destPath = destPath
		ci.decompress = false
		*cInfos = append(*cInfos, &ci)

		// Initiate the download
		resp, err := utils.Download(ci.origPath)
		if err != nil {
			return err
		}

		// Create a tmp dir
		tmpDirName, err := ioutil.TempDir(b.contextPath, "docker-remote")
		if err != nil {
			return err
		}
		ci.tmpDir = tmpDirName

		// Create a tmp file within our tmp dir
		tmpFileName := path.Join(tmpDirName, "tmp")
		tmpFile, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}

		// Download and dump result to tmp file
		if _, err := io.Copy(tmpFile, utils.ProgressReader(resp.Body, int(resp.ContentLength), b.OutOld, b.StreamFormatter, true, "", "Downloading")); err != nil {
			tmpFile.Close()
			return err
		}
		fmt.Fprintf(b.OutStream, "\n")
		tmpFile.Close()

		// Set the mtime to the Last-Modified header value if present
		// Otherwise just remove atime and mtime
		times := make([]syscall.Timespec, 2)

		lastMod := resp.Header.Get("Last-Modified")
		if lastMod != "" {
			mTime, err := http.ParseTime(lastMod)
			// If we can't parse it then just let it default to 'zero'
			// otherwise use the parsed time value
			if err == nil {
				times[1] = syscall.NsecToTimespec(mTime.UnixNano())
			}
		}

		if err := system.UtimesNano(tmpFileName, times); err != nil {
			return err
		}

		ci.origPath = path.Join(filepath.Base(tmpDirName), filepath.Base(tmpFileName))

		// If the destination is a directory, figure out the filename.
		if strings.HasSuffix(ci.destPath, "/") {
			u, err := url.Parse(origPath)
			if err != nil {
				return err
			}
			path := u.Path
			if strings.HasSuffix(path, "/") {
				path = path[:len(path)-1]
			}
			parts := strings.Split(path, "/")
			filename := parts[len(parts)-1]
			if filename == "" {
				return fmt.Errorf("cannot determine filename from url: %s", u)
			}
			ci.destPath = ci.destPath + filename
		}

		// Calc the checksum, only if we're using the cache
		if b.UtilizeCache {
			r, err := archive.Tar(tmpFileName, archive.Uncompressed)
			if err != nil {
				return err
			}
			tarSum, err := tarsum.NewTarSum(r, true, tarsum.Version0)
			if err != nil {
				return err
			}
			if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
				return err
			}
			ci.hash = tarSum.Sum(nil)
			r.Close()
		}

		return nil
	}

	// Deal with wildcards
	if ContainsWildcards(origPath) {
		for _, fileInfo := range b.context.GetSums() {
			if fileInfo.Name() == "" {
				continue
			}
			match, _ := path.Match(origPath, fileInfo.Name())
			if !match {
				continue
			}

			calcCopyInfo(b, cmdName, cInfos, fileInfo.Name(), destPath, allowRemote, allowDecompression)
		}
		return nil
	}

	// Must be a dir or a file

	if err := b.checkPathForAddition(origPath); err != nil {
		return err
	}
	fi, _ := os.Stat(path.Join(b.contextPath, origPath))

	ci := copyInfo{}
	ci.origPath = origPath
	ci.hash = origPath
	ci.destPath = destPath
	ci.decompress = allowDecompression
	*cInfos = append(*cInfos, &ci)

	// If not using cache don't need to do anything else.
	// If we are using a cache then calc the hash for the src file/dir
	if !b.UtilizeCache {
		return nil
	}

	// Deal with the single file case
	if !fi.IsDir() {
		// This will match first file in sums of the archive
		fis := b.context.GetSums().GetFile(ci.origPath)
		if fis != nil {
			ci.hash = "file:" + fis.Sum()
		}
		return nil
	}

	// Must be a dir
	var subfiles []string
	absOrigPath := path.Join(b.contextPath, ci.origPath)

	// Add a trailing / to make sure we only pick up nested files under
	// the dir and not sibling files of the dir that just happen to
	// start with the same chars
	if !strings.HasSuffix(absOrigPath, "/") {
		absOrigPath += "/"
	}

	// Need path w/o / too to find matching dir w/o trailing /
	absOrigPathNoSlash := absOrigPath[:len(absOrigPath)-1]

	for _, fileInfo := range b.context.GetSums() {
		absFile := path.Join(b.contextPath, fileInfo.Name())
		if strings.HasPrefix(absFile, absOrigPath) || absFile == absOrigPathNoSlash {
			subfiles = append(subfiles, fileInfo.Sum())
		}
	}
	sort.Strings(subfiles)
	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(subfiles, ",")))
	ci.hash = "dir:" + hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func ContainsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func (b *Builder) pullImage(name string) (*imagepkg.Image, error) {
	remote, tag := parsers.ParseRepositoryTag(name)
	if tag == "" {
		tag = "latest"
	}
	pullRegistryAuth := b.AuthConfig
	if len(b.AuthConfigFile.Configs) > 0 {
		// The request came with a full auth config file, we prefer to use that
		endpoint, _, err := registry.ResolveRepositoryName(remote)
		if err != nil {
			return nil, err
		}
		resolvedAuth := b.AuthConfigFile.ResolveAuthConfig(endpoint)
		pullRegistryAuth = &resolvedAuth
	}
	job := b.Engine.Job("pull", remote, tag)
	job.SetenvBool("json", b.StreamFormatter.Json())
	job.SetenvBool("parallel", true)
	job.SetenvJson("authConfig", pullRegistryAuth)
	job.Stdout.Add(b.OutOld)
	if err := job.Run(); err != nil {
		return nil, err
	}
	image, err := b.Daemon.Repositories().LookupImage(name)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func (b *Builder) processImageFrom(img *imagepkg.Image) error {
	b.image = img.ID

	if img.Config != nil {
		b.Config = img.Config
	}

	if len(b.Config.Env) == 0 {
		b.Config.Env = append(b.Config.Env, "PATH="+daemon.DefaultPathEnv)
	}

	// Process ONBUILD triggers if they exist
	if nTriggers := len(b.Config.OnBuild); nTriggers != 0 {
		fmt.Fprintf(b.ErrStream, "# Executing %d build triggers\n", nTriggers)
	}

	// Copy the ONBUILD triggers, and remove them from the config, since the config will be commited.
	onBuildTriggers := b.Config.OnBuild
	b.Config.OnBuild = []string{}

	// parse the ONBUILD triggers by invoking the parser
	for stepN, step := range onBuildTriggers {
		ast, err := parser.Parse(strings.NewReader(step))
		if err != nil {
			return err
		}

		for i, n := range ast.Children {
			switch strings.ToUpper(n.Value) {
			case "ONBUILD":
				return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
			case "MAINTAINER", "FROM":
				return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", n.Value)
			}

			fmt.Fprintf(b.OutStream, "Trigger %d, %s\n", stepN, step)

			if err := b.dispatch(i, n); err != nil {
				return err
			}
		}
	}

	return nil
}

// probeCache checks to see if image-caching is enabled (`b.UtilizeCache`)
// and if so attempts to look up the current `b.image` and `b.Config` pair
// in the current server `b.Daemon`. If an image is found, probeCache returns
// `(true, nil)`. If no image is found, it returns `(false, nil)`. If there
// is any error, it returns `(false, err)`.
func (b *Builder) probeCache() (bool, error) {
	if b.UtilizeCache {
		if cache, err := b.Daemon.ImageGetCached(b.image, b.Config); err != nil {
			return false, err
		} else if cache != nil {
			fmt.Fprintf(b.OutStream, " ---> Using cache\n")
			log.Debugf("[BUILDER] Use cached version")
			b.image = cache.ID
			return true, nil
		} else {
			log.Debugf("[BUILDER] Cache miss")
		}
	}
	return false, nil
}

func (b *Builder) create() (*daemon.Container, error) {
	if b.image == "" {
		return nil, fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.Config.Image = b.image

	config := *b.Config

	// Create the container
	c, warnings, err := b.Daemon.Create(b.Config, nil, "")
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		fmt.Fprintf(b.OutStream, " ---> [Warning] %s\n", warning)
	}

	b.TmpContainers[c.ID] = struct{}{}
	fmt.Fprintf(b.OutStream, " ---> Running in %s\n", utils.TruncateID(c.ID))

	// override the entry point that may have been picked up from the base image
	c.Path = config.Cmd[0]
	c.Args = config.Cmd[1:]

	return c, nil
}

func (b *Builder) run(c *daemon.Container) error {
	//start the container
	if err := c.Start(); err != nil {
		return err
	}

	if b.Verbose {
		logsJob := b.Engine.Job("logs", c.ID)
		logsJob.Setenv("follow", "1")
		logsJob.Setenv("stdout", "1")
		logsJob.Setenv("stderr", "1")
		logsJob.Stdout.Add(b.OutStream)
		logsJob.Stderr.Set(b.ErrStream)
		if err := logsJob.Run(); err != nil {
			return err
		}
	}

	// Wait for it to finish
	if ret, _ := c.WaitStop(-1 * time.Second); ret != 0 {
		err := &utils.JSONError{
			Message: fmt.Sprintf("The command %v returned a non-zero code: %d", b.Config.Cmd, ret),
			Code:    ret,
		}
		return err
	}

	return nil
}

func (b *Builder) checkPathForAddition(orig string) error {
	origPath := path.Join(b.contextPath, orig)
	origPath, err := filepath.EvalSymlinks(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	if !strings.HasPrefix(origPath, b.contextPath) {
		return fmt.Errorf("Forbidden path outside the build context: %s (%s)", orig, origPath)
	}
	if _, err := os.Stat(origPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	return nil
}

func (b *Builder) addContext(container *daemon.Container, orig, dest string, decompress bool) error {
	var (
		err        error
		destExists = true
		origPath   = path.Join(b.contextPath, orig)
		destPath   = path.Join(container.RootfsPath(), dest)
	)

	if destPath != container.RootfsPath() {
		destPath, err = symlink.FollowSymlinkInScope(destPath, container.RootfsPath())
		if err != nil {
			return err
		}
	}

	// Preserve the trailing '/'
	if strings.HasSuffix(dest, "/") || dest == "." {
		destPath = destPath + "/"
	}

	destStat, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		destExists = false
	}

	fi, err := os.Stat(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}

	if fi.IsDir() {
		return copyAsDirectory(origPath, destPath, destExists)
	}

	// If we are adding a remote file (or we've been told not to decompress), do not try to untar it
	if decompress {
		// First try to unpack the source as an archive
		// to support the untar feature we need to clean up the path a little bit
		// because tar is very forgiving.  First we need to strip off the archive's
		// filename from the path but this is only added if it does not end in / .
		tarDest := destPath
		if strings.HasSuffix(tarDest, "/") {
			tarDest = filepath.Dir(destPath)
		}

		// try to successfully untar the orig
		if err := chrootarchive.UntarPath(origPath, tarDest); err == nil {
			return nil
		} else if err != io.EOF {
			log.Debugf("Couldn't untar %s to %s: %s", origPath, tarDest, err)
		}
	}

	if err := os.MkdirAll(path.Dir(destPath), 0755); err != nil {
		return err
	}
	if err := chrootarchive.CopyWithTar(origPath, destPath); err != nil {
		return err
	}

	resPath := destPath
	if destExists && destStat.IsDir() {
		resPath = path.Join(destPath, path.Base(origPath))
	}

	return fixPermissions(origPath, resPath, 0, 0, destExists)
}

func copyAsDirectory(source, destination string, destExisted bool) error {
	if err := chrootarchive.CopyWithTar(source, destination); err != nil {
		return err
	}
	return fixPermissions(source, destination, 0, 0, destExisted)
}

func fixPermissions(source, destination string, uid, gid int, destExisted bool) error {
	// If the destination didn't already exist, or the destination isn't a
	// directory, then we should Lchown the destination. Otherwise, we shouldn't
	// Lchown the destination.
	destStat, err := os.Stat(destination)
	if err != nil {
		// This should *never* be reached, because the destination must've already
		// been created while untar-ing the context.
		return err
	}
	doChownDestination := !destExisted || !destStat.IsDir()

	// We Walk on the source rather than on the destination because we don't
	// want to change permissions on things we haven't created or modified.
	return filepath.Walk(source, func(fullpath string, info os.FileInfo, err error) error {
		// Do not alter the walk root iff. it existed before, as it doesn't fall under
		// the domain of "things we should chown".
		if !doChownDestination && (source == fullpath) {
			return nil
		}

		// Path is prefixed by source: substitute with destination instead.
		cleaned, err := filepath.Rel(source, fullpath)
		if err != nil {
			return err
		}

		fullpath = path.Join(destination, cleaned)
		return os.Lchown(fullpath, uid, gid)
	})
}

func (b *Builder) clearTmp() {
	for c := range b.TmpContainers {
		tmp := b.Daemon.Get(c)
		if err := b.Daemon.Destroy(tmp); err != nil {
			fmt.Fprintf(b.OutStream, "Error removing intermediate container %s: %s\n", utils.TruncateID(c), err.Error())
			return
		}
		b.Daemon.DeleteVolumes(tmp.VolumePaths())
		delete(b.TmpContainers, c)
		fmt.Fprintf(b.OutStream, "Removing intermediate container %s\n", utils.TruncateID(c))
	}
}
