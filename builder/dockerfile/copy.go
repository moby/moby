package dockerfile

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/remotecontext"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/pkg/errors"
)

type pathCache interface {
	Load(key interface{}) (value interface{}, ok bool)
	Store(key, value interface{})
}

// copyInfo is a data object which stores the metadata about each source file in
// a copyInstruction
type copyInfo struct {
	root string
	path string
	hash string
}

func newCopyInfoFromSource(source builder.Source, path string, hash string) copyInfo {
	return copyInfo{root: source.Root(), path: path, hash: hash}
}

func newCopyInfos(copyInfos ...copyInfo) []copyInfo {
	return copyInfos
}

// copyInstruction is a fully parsed COPY or ADD command that is passed to
// Builder.performCopy to copy files into the image filesystem
type copyInstruction struct {
	cmdName                 string
	infos                   []copyInfo
	dest                    string
	allowLocalDecompression bool
}

// copier reads a raw COPY or ADD command, fetches remote sources using a downloader,
// and creates a copyInstruction
type copier struct {
	imageSource *imageMount
	source      builder.Source
	pathCache   pathCache
	download    sourceDownloader
	tmpPaths    []string
}

func copierFromDispatchRequest(req dispatchRequest, download sourceDownloader, imageSource *imageMount) copier {
	return copier{
		source:      req.source,
		pathCache:   req.builder.pathCache,
		download:    download,
		imageSource: imageSource,
	}
}

func (o *copier) createCopyInstruction(args []string, cmdName string) (copyInstruction, error) {
	inst := copyInstruction{cmdName: cmdName}
	last := len(args) - 1

	// Work in daemon-specific filepath semantics
	inst.dest = filepath.FromSlash(args[last])

	infos, err := o.getCopyInfosForSourcePaths(args[0:last])
	if err != nil {
		return inst, errors.Wrapf(err, "%s failed", cmdName)
	}
	if len(infos) > 1 && !strings.HasSuffix(inst.dest, string(os.PathSeparator)) {
		return inst, errors.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}
	inst.infos = infos
	return inst, nil
}

// getCopyInfosForSourcePaths iterates over the source files and calculate the info
// needed to copy (e.g. hash value if cached)
func (o *copier) getCopyInfosForSourcePaths(sources []string) ([]copyInfo, error) {
	var infos []copyInfo
	for _, orig := range sources {
		subinfos, err := o.getCopyInfoForSourcePath(orig)
		if err != nil {
			return nil, err
		}
		infos = append(infos, subinfos...)
	}

	if len(infos) == 0 {
		return nil, errors.New("no source files were specified")
	}
	return infos, nil
}

func (o *copier) getCopyInfoForSourcePath(orig string) ([]copyInfo, error) {
	if !urlutil.IsURL(orig) {
		return o.calcCopyInfo(orig, true)
	}
	remote, path, err := o.download(orig)
	if err != nil {
		return nil, err
	}
	o.tmpPaths = append(o.tmpPaths, remote.Root())

	hash, err := remote.Hash(path)
	return newCopyInfos(newCopyInfoFromSource(remote, path, hash)), err
}

// Cleanup removes any temporary directories created as part of downloading
// remote files.
func (o *copier) Cleanup() {
	for _, path := range o.tmpPaths {
		os.RemoveAll(path)
	}
	o.tmpPaths = []string{}
}

// TODO: allowWildcards can probably be removed by refactoring this function further.
func (o *copier) calcCopyInfo(origPath string, allowWildcards bool) ([]copyInfo, error) {
	imageSource := o.imageSource
	if err := validateCopySourcePath(imageSource, origPath); err != nil {
		return nil, err
	}

	// Work in daemon-specific OS filepath semantics
	origPath = filepath.FromSlash(origPath)
	origPath = strings.TrimPrefix(origPath, string(os.PathSeparator))
	origPath = strings.TrimPrefix(origPath, "."+string(os.PathSeparator))

	// TODO: do this when creating copier. Requires validateCopySourcePath
	// (and other below) to be aware of the difference sources. Why is it only
	// done on image Source?
	if imageSource != nil {
		var err error
		o.source, err = imageSource.Source()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to copy")
		}
	}

	if o.source == nil {
		return nil, errors.Errorf("missing build context")
	}

	// Deal with wildcards
	if allowWildcards && containsWildcards(origPath) {
		return o.copyWithWildcards(origPath)
	}

	if imageSource != nil && imageSource.ImageID() != "" {
		// return a cached copy if one exists
		if h, ok := o.pathCache.Load(imageSource.ImageID() + origPath); ok {
			return newCopyInfos(newCopyInfoFromSource(o.source, origPath, h.(string))), nil
		}
	}

	// Deal with the single file case
	copyInfo, err := copyInfoForFile(o.source, origPath)
	switch {
	case err != nil:
		return nil, err
	case copyInfo.hash != "":
		o.storeInPathCache(imageSource, origPath, copyInfo.hash)
		return newCopyInfos(copyInfo), err
	}

	// TODO: remove, handle dirs in Hash()
	subfiles, err := walkSource(o.source, origPath)
	if err != nil {
		return nil, err
	}

	hash := hashStringSlice("dir", subfiles)
	o.storeInPathCache(imageSource, origPath, hash)
	return newCopyInfos(newCopyInfoFromSource(o.source, origPath, hash)), nil
}

func (o *copier) storeInPathCache(im *imageMount, path string, hash string) {
	if im != nil {
		o.pathCache.Store(im.ImageID()+path, hash)
	}
}

func (o *copier) copyWithWildcards(origPath string) ([]copyInfo, error) {
	var copyInfos []copyInfo
	if err := filepath.Walk(o.source.Root(), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := remotecontext.Rel(o.source.Root(), path)
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
		subInfos, err := o.calcCopyInfo(rel, false)
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

func copyInfoForFile(source builder.Source, path string) (copyInfo, error) {
	fi, err := remotecontext.StatAt(source, path)
	if err != nil {
		return copyInfo{}, err
	}

	if fi.IsDir() {
		return copyInfo{}, nil
	}
	hash, err := source.Hash(path)
	if err != nil {
		return copyInfo{}, err
	}
	return newCopyInfoFromSource(source, path, "file:"+hash), nil
}

// TODO: dedupe with copyWithWildcards()
func walkSource(source builder.Source, origPath string) ([]string, error) {
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
	return subfiles, nil
}

type sourceDownloader func(string) (builder.Source, string, error)

func newRemoteSourceDownloader(output, stdout io.Writer) sourceDownloader {
	return func(url string) (builder.Source, string, error) {
		return downloadSource(output, stdout, url)
	}
}

func errOnSourceDownload(_ string) (builder.Source, string, error) {
	return nil, "", errors.New("source can't be a URL for COPY")
}

func downloadSource(output io.Writer, stdout io.Writer, srcURL string) (remote builder.Source, p string, err error) {
	// get filename from URL
	u, err := url.Parse(srcURL)
	if err != nil {
		return
	}
	filename := filepath.Base(filepath.FromSlash(u.Path)) // Ensure in platform semantics
	if filename == "" {
		err = errors.Errorf("cannot determine filename from url: %s", u)
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

	progressOutput := streamformatter.NewJSONProgressOutput(output, true)
	progressReader := progress.NewProgressReader(resp.Body, progressOutput, resp.ContentLength, "", "Downloading")
	// Download and dump result to tmp file
	// TODO: add filehash directly
	if _, err = io.Copy(tmpFile, progressReader); err != nil {
		tmpFile.Close()
		return
	}
	// TODO: how important is this random blank line to the output?
	fmt.Fprintln(stdout)

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
	return lc, filename, err
}
