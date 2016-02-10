package client

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerignore"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/gitutils"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/reference"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
)

type translatorFunc func(reference.NamedTagged) (reference.Canonical, error)

// CmdBuild builds a new image from the source code at a given path.
//
// If '-' is provided instead of a path or URL, Docker will build an image from either a Dockerfile or tar archive read from STDIN.
//
// Usage: docker build [OPTIONS] PATH | URL | -
func (cli *DockerCli) CmdBuild(args ...string) error {
	cmd := Cli.Subcmd("build", []string{"PATH | URL | -"}, Cli.DockerCommands["build"].Description, true)
	flTags := opts.NewListOpts(validateTag)
	cmd.Var(&flTags, []string{"t", "-tag"}, "Name and optionally a tag in the 'name:tag' format")
	suppressOutput := cmd.Bool([]string{"q", "-quiet"}, false, "Suppress the build output and print image ID on success")
	noCache := cmd.Bool([]string{"-no-cache"}, false, "Do not use cache when building the image")
	rm := cmd.Bool([]string{"-rm"}, true, "Remove intermediate containers after a successful build")
	forceRm := cmd.Bool([]string{"-force-rm"}, false, "Always remove intermediate containers")
	pull := cmd.Bool([]string{"-pull"}, false, "Always attempt to pull a newer version of the image")
	dockerfileName := cmd.String([]string{"f", "-file"}, "", "Name of the Dockerfile (Default is 'PATH/Dockerfile')")
	flMemoryString := cmd.String([]string{"m", "-memory"}, "", "Memory limit")
	flMemorySwap := cmd.String([]string{"-memory-swap"}, "", "Swap limit equal to memory plus swap: '-1' to enable unlimited swap")
	flShmSize := cmd.String([]string{"-shm-size"}, "", "Size of /dev/shm, default value is 64MB")
	flCPUShares := cmd.Int64([]string{"#c", "-cpu-shares"}, 0, "CPU shares (relative weight)")
	flCPUPeriod := cmd.Int64([]string{"-cpu-period"}, 0, "Limit the CPU CFS (Completely Fair Scheduler) period")
	flCPUQuota := cmd.Int64([]string{"-cpu-quota"}, 0, "Limit the CPU CFS (Completely Fair Scheduler) quota")
	flCPUSetCpus := cmd.String([]string{"-cpuset-cpus"}, "", "CPUs in which to allow execution (0-3, 0,1)")
	flCPUSetMems := cmd.String([]string{"-cpuset-mems"}, "", "MEMs in which to allow execution (0-3, 0,1)")
	flCgroupParent := cmd.String([]string{"-cgroup-parent"}, "", "Optional parent cgroup for the container")
	flBuildArg := opts.NewListOpts(runconfigopts.ValidateEnv)
	cmd.Var(&flBuildArg, []string{"-build-arg"}, "Set build-time variables")
	isolation := cmd.String([]string{"-isolation"}, "", "Container isolation technology")

	ulimits := make(map[string]*units.Ulimit)
	flUlimits := runconfigopts.NewUlimitOpt(&ulimits)
	cmd.Var(flUlimits, []string{"-ulimit"}, "Ulimit options")

	cmd.Require(flag.Exact, 1)

	// For trusted pull on "FROM <image>" instruction.
	addTrustedFlags(cmd, true)

	cmd.ParseFlags(args, true)

	var (
		ctx io.ReadCloser
		err error
	)

	specifiedContext := cmd.Arg(0)

	var (
		contextDir    string
		tempDir       string
		relDockerfile string
		progBuff      io.Writer
		buildBuff     io.Writer
	)

	progBuff = cli.out
	buildBuff = cli.out
	if *suppressOutput {
		progBuff = bytes.NewBuffer(nil)
		buildBuff = bytes.NewBuffer(nil)
	}

	switch {
	case specifiedContext == "-":
		ctx, relDockerfile, err = getContextFromReader(cli.in, *dockerfileName)
	case urlutil.IsGitURL(specifiedContext):
		tempDir, relDockerfile, err = getContextFromGitURL(specifiedContext, *dockerfileName)
	case urlutil.IsURL(specifiedContext):
		ctx, relDockerfile, err = getContextFromURL(progBuff, specifiedContext, *dockerfileName)
	default:
		contextDir, relDockerfile, err = getContextFromLocalDir(specifiedContext, *dockerfileName)
	}

	if err != nil {
		if *suppressOutput && urlutil.IsURL(specifiedContext) {
			fmt.Fprintln(cli.err, progBuff)
		}
		return fmt.Errorf("unable to prepare context: %s", err)
	}

	if tempDir != "" {
		defer os.RemoveAll(tempDir)
		contextDir = tempDir
	}

	if ctx == nil {
		// And canonicalize dockerfile name to a platform-independent one
		relDockerfile, err = archive.CanonicalTarNameForPath(relDockerfile)
		if err != nil {
			return fmt.Errorf("cannot canonicalize dockerfile path %s: %v", relDockerfile, err)
		}

		f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		var excludes []string
		if err == nil {
			excludes, err = dockerignore.ReadAll(f)
			if err != nil {
				return err
			}
		}

		if err := builder.ValidateContextDirectory(contextDir, excludes); err != nil {
			return fmt.Errorf("Error checking context: '%s'.", err)
		}

		// If .dockerignore mentions .dockerignore or the Dockerfile
		// then make sure we send both files over to the daemon
		// because Dockerfile is, obviously, needed no matter what, and
		// .dockerignore is needed to know if either one needs to be
		// removed. The daemon will remove them for us, if needed, after it
		// parses the Dockerfile. Ignore errors here, as they will have been
		// caught by validateContextDirectory above.
		var includes = []string{"."}
		keepThem1, _ := fileutils.Matches(".dockerignore", excludes)
		keepThem2, _ := fileutils.Matches(relDockerfile, excludes)
		if keepThem1 || keepThem2 {
			includes = append(includes, ".dockerignore", relDockerfile)
		}

		ctx, err = archive.TarWithOptions(contextDir, &archive.TarOptions{
			Compression:     archive.Uncompressed,
			ExcludePatterns: excludes,
			IncludeFiles:    includes,
		})
		if err != nil {
			return err
		}
	}

	var resolvedTags []*resolvedTag
	if isTrusted() {
		// Wrap the tar archive to replace the Dockerfile entry with the rewritten
		// Dockerfile which uses trusted pulls.
		ctx = replaceDockerfileTarWrapper(ctx, relDockerfile, cli.trustedReference, &resolvedTags)
	}

	// Setup an upload progress bar
	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(progBuff, true)

	var body io.Reader = progress.NewProgressReader(ctx, progressOutput, 0, "", "Sending build context to Docker daemon")

	var memory int64
	if *flMemoryString != "" {
		parsedMemory, err := units.RAMInBytes(*flMemoryString)
		if err != nil {
			return err
		}
		memory = parsedMemory
	}

	var memorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			parsedMemorySwap, err := units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return err
			}
			memorySwap = parsedMemorySwap
		}
	}

	var shmSize int64
	if *flShmSize != "" {
		shmSize, err = units.RAMInBytes(*flShmSize)
		if err != nil {
			return err
		}
	}

	options := types.ImageBuildOptions{
		Context:        body,
		Memory:         memory,
		MemorySwap:     memorySwap,
		Tags:           flTags.GetAll(),
		SuppressOutput: *suppressOutput,
		NoCache:        *noCache,
		Remove:         *rm,
		ForceRemove:    *forceRm,
		PullParent:     *pull,
		Isolation:      container.Isolation(*isolation),
		CPUSetCPUs:     *flCPUSetCpus,
		CPUSetMems:     *flCPUSetMems,
		CPUShares:      *flCPUShares,
		CPUQuota:       *flCPUQuota,
		CPUPeriod:      *flCPUPeriod,
		CgroupParent:   *flCgroupParent,
		Dockerfile:     relDockerfile,
		ShmSize:        shmSize,
		Ulimits:        flUlimits.GetList(),
		BuildArgs:      runconfigopts.ConvertKVStringsToMap(flBuildArg.GetAll()),
		AuthConfigs:    cli.configFile.AuthConfigs,
	}

	response, err := cli.client.ImageBuild(context.Background(), options)
	if err != nil {
		return err
	}

	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, cli.outFd, cli.isTerminalOut, nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			if *suppressOutput {
				fmt.Fprintf(cli.err, "%s%s", progBuff, buildBuff)
			}
			return Cli.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
		}
	}

	// Windows: show error message about modified file permissions if the
	// daemon isn't running Windows.
	if response.OSType != "windows" && runtime.GOOS == "windows" {
		fmt.Fprintln(cli.err, `SECURITY WARNING: You are building a Docker image from Windows against a non-Windows Docker host. All files and directories added to build context will have '-rwxr-xr-x' permissions. It is recommended to double check and reset permissions for sensitive files and directories.`)
	}

	// Everything worked so if -q was provided the output from the daemon
	// should be just the image ID and we'll print that to stdout.
	if *suppressOutput {
		fmt.Fprintf(cli.out, "%s", buildBuff)
	}

	if isTrusted() {
		// Since the build was successful, now we must tag any of the resolved
		// images from the above Dockerfile rewrite.
		for _, resolved := range resolvedTags {
			if err := cli.tagTrusted(resolved.digestRef, resolved.tagRef); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateTag checks if the given image name can be resolved.
func validateTag(rawRepo string) (string, error) {
	_, err := reference.ParseNamed(rawRepo)
	if err != nil {
		return "", err
	}

	return rawRepo, nil
}

// isUNC returns true if the path is UNC (one starting \\). It always returns
// false on Linux.
func isUNC(path string) bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(path, `\\`)
}

// getDockerfileRelPath uses the given context directory for a `docker build`
// and returns the absolute path to the context directory, the relative path of
// the dockerfile in that context directory, and a non-nil error on success.
func getDockerfileRelPath(givenContextDir, givenDockerfile string) (absContextDir, relDockerfile string, err error) {
	if absContextDir, err = filepath.Abs(givenContextDir); err != nil {
		return "", "", fmt.Errorf("unable to get absolute context directory: %v", err)
	}

	// The context dir might be a symbolic link, so follow it to the actual
	// target directory.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	if !isUNC(absContextDir) {
		absContextDir, err = filepath.EvalSymlinks(absContextDir)
		if err != nil {
			return "", "", fmt.Errorf("unable to evaluate symlinks in context path: %v", err)
		}
	}

	stat, err := os.Lstat(absContextDir)
	if err != nil {
		return "", "", fmt.Errorf("unable to stat context directory %q: %v", absContextDir, err)
	}

	if !stat.IsDir() {
		return "", "", fmt.Errorf("context must be a directory: %s", absContextDir)
	}

	absDockerfile := givenDockerfile
	if absDockerfile == "" {
		// No -f/--file was specified so use the default relative to the
		// context directory.
		absDockerfile = filepath.Join(absContextDir, api.DefaultDockerfileName)

		// Just to be nice ;-) look for 'dockerfile' too but only
		// use it if we found it, otherwise ignore this check
		if _, err = os.Lstat(absDockerfile); os.IsNotExist(err) {
			altPath := filepath.Join(absContextDir, strings.ToLower(api.DefaultDockerfileName))
			if _, err = os.Lstat(altPath); err == nil {
				absDockerfile = altPath
			}
		}
	}

	// If not already an absolute path, the Dockerfile path should be joined to
	// the base directory.
	if !filepath.IsAbs(absDockerfile) {
		absDockerfile = filepath.Join(absContextDir, absDockerfile)
	}

	// Evaluate symlinks in the path to the Dockerfile too.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	if !isUNC(absDockerfile) {
		absDockerfile, err = filepath.EvalSymlinks(absDockerfile)
		if err != nil {
			return "", "", fmt.Errorf("unable to evaluate symlinks in Dockerfile path: %v", err)
		}
	}

	if _, err := os.Lstat(absDockerfile); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("Cannot locate Dockerfile: %q", absDockerfile)
		}
		return "", "", fmt.Errorf("unable to stat Dockerfile: %v", err)
	}

	if relDockerfile, err = filepath.Rel(absContextDir, absDockerfile); err != nil {
		return "", "", fmt.Errorf("unable to get relative Dockerfile path: %v", err)
	}

	if strings.HasPrefix(relDockerfile, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("The Dockerfile (%s) must be within the build context (%s)", givenDockerfile, givenContextDir)
	}

	return absContextDir, relDockerfile, nil
}

// writeToFile copies from the given reader and writes it to a file with the
// given filename.
func writeToFile(r io.Reader, filename string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("unable to write file: %v", err)
	}

	return nil
}

// getContextFromReader will read the contents of the given reader as either a
// Dockerfile or tar archive. Returns a tar archive used as a context and a
// path to the Dockerfile inside the tar.
func getContextFromReader(r io.ReadCloser, dockerfileName string) (out io.ReadCloser, relDockerfile string, err error) {
	buf := bufio.NewReader(r)

	magic, err := buf.Peek(archive.HeaderSize)
	if err != nil && err != io.EOF {
		return nil, "", fmt.Errorf("failed to peek context header from STDIN: %v", err)
	}

	if archive.IsArchive(magic) {
		return ioutils.NewReadCloserWrapper(buf, func() error { return r.Close() }), dockerfileName, nil
	}

	// Input should be read as a Dockerfile.
	tmpDir, err := ioutil.TempDir("", "docker-build-context-")
	if err != nil {
		return nil, "", fmt.Errorf("unbale to create temporary context directory: %v", err)
	}

	f, err := os.Create(filepath.Join(tmpDir, api.DefaultDockerfileName))
	if err != nil {
		return nil, "", err
	}
	_, err = io.Copy(f, buf)
	if err != nil {
		f.Close()
		return nil, "", err
	}

	if err := f.Close(); err != nil {
		return nil, "", err
	}
	if err := r.Close(); err != nil {
		return nil, "", err
	}

	tar, err := archive.Tar(tmpDir, archive.Uncompressed)
	if err != nil {
		return nil, "", err
	}

	return ioutils.NewReadCloserWrapper(tar, func() error {
		err := tar.Close()
		os.RemoveAll(tmpDir)
		return err
	}), api.DefaultDockerfileName, nil

}

// getContextFromGitURL uses a Git URL as context for a `docker build`. The
// git repo is cloned into a temporary directory used as the context directory.
// Returns the absolute path to the temporary context directory, the relative
// path of the dockerfile in that context directory, and a non-nil error on
// success.
func getContextFromGitURL(gitURL, dockerfileName string) (absContextDir, relDockerfile string, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", fmt.Errorf("unable to find 'git': %v", err)
	}
	if absContextDir, err = gitutils.Clone(gitURL); err != nil {
		return "", "", fmt.Errorf("unable to 'git clone' to temporary context directory: %v", err)
	}

	return getDockerfileRelPath(absContextDir, dockerfileName)
}

// getContextFromURL uses a remote URL as context for a `docker build`. The
// remote resource is downloaded as either a Dockerfile or a tar archive.
// Returns the tar archive used for the context and a path of the
// dockerfile inside the tar.
func getContextFromURL(out io.Writer, remoteURL, dockerfileName string) (io.ReadCloser, string, error) {
	response, err := httputils.Download(remoteURL)
	if err != nil {
		return nil, "", fmt.Errorf("unable to download remote context %s: %v", remoteURL, err)
	}
	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(out, true)

	// Pass the response body through a progress reader.
	progReader := progress.NewProgressReader(response.Body, progressOutput, response.ContentLength, "", fmt.Sprintf("Downloading build context from remote url: %s", remoteURL))

	return getContextFromReader(ioutils.NewReadCloserWrapper(progReader, func() error { return response.Body.Close() }), dockerfileName)
}

// getContextFromLocalDir uses the given local directory as context for a
// `docker build`. Returns the absolute path to the local context directory,
// the relative path of the dockerfile in that context directory, and a non-nil
// error on success.
func getContextFromLocalDir(localDir, dockerfileName string) (absContextDir, relDockerfile string, err error) {
	// When using a local context directory, when the Dockerfile is specified
	// with the `-f/--file` option then it is considered relative to the
	// current directory and not the context directory.
	if dockerfileName != "" {
		if dockerfileName, err = filepath.Abs(dockerfileName); err != nil {
			return "", "", fmt.Errorf("unable to get absolute path to Dockerfile: %v", err)
		}
	}

	return getDockerfileRelPath(localDir, dockerfileName)
}

var dockerfileFromLinePattern = regexp.MustCompile(`(?i)^[\s]*FROM[ \f\r\t\v]+(?P<image>[^ \f\r\t\v\n#]+)`)

// resolvedTag records the repository, tag, and resolved digest reference
// from a Dockerfile rewrite.
type resolvedTag struct {
	digestRef reference.Canonical
	tagRef    reference.NamedTagged
}

// rewriteDockerfileFrom rewrites the given Dockerfile by resolving images in
// "FROM <image>" instructions to a digest reference. `translator` is a
// function that takes a repository name and tag reference and returns a
// trusted digest reference.
func rewriteDockerfileFrom(dockerfile io.Reader, translator translatorFunc) (newDockerfile []byte, resolvedTags []*resolvedTag, err error) {
	scanner := bufio.NewScanner(dockerfile)
	buf := bytes.NewBuffer(nil)

	// Scan the lines of the Dockerfile, looking for a "FROM" line.
	for scanner.Scan() {
		line := scanner.Text()

		matches := dockerfileFromLinePattern.FindStringSubmatch(line)
		if matches != nil && matches[1] != api.NoBaseImageSpecifier {
			// Replace the line with a resolved "FROM repo@digest"
			ref, err := reference.ParseNamed(matches[1])
			if err != nil {
				return nil, nil, err
			}
			ref = reference.WithDefaultTag(ref)
			if ref, ok := ref.(reference.NamedTagged); ok && isTrusted() {
				trustedRef, err := translator(ref)
				if err != nil {
					return nil, nil, err
				}

				line = dockerfileFromLinePattern.ReplaceAllLiteralString(line, fmt.Sprintf("FROM %s", trustedRef.String()))
				resolvedTags = append(resolvedTags, &resolvedTag{
					digestRef: trustedRef,
					tagRef:    ref,
				})
			}
		}

		_, err := fmt.Fprintln(buf, line)
		if err != nil {
			return nil, nil, err
		}
	}

	return buf.Bytes(), resolvedTags, scanner.Err()
}

// replaceDockerfileTarWrapper wraps the given input tar archive stream and
// replaces the entry with the given Dockerfile name with the contents of the
// new Dockerfile. Returns a new tar archive stream with the replaced
// Dockerfile.
func replaceDockerfileTarWrapper(inputTarStream io.ReadCloser, dockerfileName string, translator translatorFunc, resolvedTags *[]*resolvedTag) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		tarReader := tar.NewReader(inputTarStream)
		tarWriter := tar.NewWriter(pipeWriter)

		defer inputTarStream.Close()

		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				// Signals end of archive.
				tarWriter.Close()
				pipeWriter.Close()
				return
			}
			if err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			var content io.Reader = tarReader
			if hdr.Name == dockerfileName {
				// This entry is the Dockerfile. Since the tar archive was
				// generated from a directory on the local filesystem, the
				// Dockerfile will only appear once in the archive.
				var newDockerfile []byte
				newDockerfile, *resolvedTags, err = rewriteDockerfileFrom(content, translator)
				if err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				hdr.Size = int64(len(newDockerfile))
				content = bytes.NewBuffer(newDockerfile)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			if _, err := io.Copy(tarWriter, content); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}
	}()

	return pipeReader
}
