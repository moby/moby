package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/utils"
)

func (b *BuildFile) findEnvKey(key string) int {
	for k, envVar := range b.config.Env {
		envParts := strings.SplitN(envVar, "=", 2)
		if key == envParts[0] {
			return k
		}
	}
	return -1
}

func (b *BuildFile) replaceEnvMatches(value string) (string, error) {
	exp, err := regexp.Compile("(\\\\\\\\+|[^\\\\]|\\b|\\A)\\$({?)([[:alnum:]_]+)(}?)")
	if err != nil {
		return value, err
	}
	matches := exp.FindAllString(value, -1)
	for _, match := range matches {
		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		for _, envVar := range b.config.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			envKey := envParts[0]
			envValue := envParts[1]

			if envKey == matchKey {
				value = strings.Replace(value, match, envValue, -1)
				break
			}
		}
	}
	return value, nil
}

func (b *BuildFile) buildCmdFromJson(args string) []string {
	var cmd []string
	if err := json.Unmarshal([]byte(args), &cmd); err != nil {
		utils.Debugf("Error unmarshalling: %s, setting to /bin/sh -c", err)
		cmd = []string{"/bin/sh", "-c", args}
	}
	return cmd
}

func (b *BuildFile) runContextCommand(args string, allowRemote bool, allowDecompression bool, cmdName string) error {
	if b.context == nil {
		return fmt.Errorf("No context given. Impossible to use %s", cmdName)
	}
	tmp := strings.SplitN(args, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("Invalid %s format", cmdName)
	}

	orig, err := b.replaceEnvMatches(strings.Trim(tmp[0], " \t"))
	if err != nil {
		return err
	}

	dest, err := b.replaceEnvMatches(strings.Trim(tmp[1], " \t"))
	if err != nil {
		return err
	}

	cmd := b.config.Cmd
	b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) %s %s in %s", cmdName, orig, dest)}
	defer func(cmd []string) { b.config.Cmd = cmd }(cmd)
	b.config.Image = b.image

	var (
		origPath   = orig
		destPath   = dest
		remoteHash string
		isRemote   bool
		decompress = true
	)

	isRemote = utils.IsURL(orig)
	if isRemote && !allowRemote {
		return fmt.Errorf("Source can't be an URL for %s", cmdName)
	} else if utils.IsURL(orig) {
		// Initiate the download
		resp, err := utils.Download(orig)
		if err != nil {
			return err
		}

		// Create a tmp dir
		tmpDirName, err := ioutil.TempDir(b.contextPath, "docker-remote")
		if err != nil {
			return err
		}

		// Create a tmp file within our tmp dir
		tmpFileName := path.Join(tmpDirName, "tmp")
		tmpFile, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDirName)

		// Download and dump result to tmp file
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return err
		}
		tmpFile.Close()

		// Remove the mtime of the newly created tmp file
		if err := system.UtimesNano(tmpFileName, make([]syscall.Timespec, 2)); err != nil {
			return err
		}

		origPath = path.Join(filepath.Base(tmpDirName), filepath.Base(tmpFileName))

		// Process the checksum
		r, err := archive.Tar(tmpFileName, archive.Uncompressed)
		if err != nil {
			return err
		}
		tarSum := &utils.TarSum{Reader: r, DisableCompression: true}
		if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
			return err
		}
		remoteHash = tarSum.Sum(nil)
		r.Close()

		// If the destination is a directory, figure out the filename.
		if strings.HasSuffix(dest, "/") {
			u, err := url.Parse(orig)
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
			destPath = dest + filename
		}
	}

	if err := b.checkPathForAddition(origPath); err != nil {
		return err
	}

	// Hash path and check the cache
	if b.utilizeCache {
		var (
			hash string
			sums = b.context.GetSums()
		)

		if remoteHash != "" {
			hash = remoteHash
		} else if fi, err := os.Stat(path.Join(b.contextPath, origPath)); err != nil {
			return err
		} else if fi.IsDir() {
			var subfiles []string
			for file, sum := range sums {
				absFile := path.Join(b.contextPath, file)
				absOrigPath := path.Join(b.contextPath, origPath)
				if strings.HasPrefix(absFile, absOrigPath) {
					subfiles = append(subfiles, sum)
				}
			}
			sort.Strings(subfiles)
			hasher := sha256.New()
			hasher.Write([]byte(strings.Join(subfiles, ",")))
			hash = "dir:" + hex.EncodeToString(hasher.Sum(nil))
		} else {
			if origPath[0] == '/' && len(origPath) > 1 {
				origPath = origPath[1:]
			}
			origPath = strings.TrimPrefix(origPath, "./")
			if h, ok := sums[origPath]; ok {
				hash = "file:" + h
			}
		}
		b.config.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("#(nop) %s %s in %s", cmdName, hash, dest)}
		hit, err := b.probeCache()
		if err != nil {
			return err
		}
		// If we do not have a hash, never use the cache
		if hit && hash != "" {
			return nil
		}
	}

	// Create the container
	container, _, err := b.daemon.Create(b.config, "")
	if err != nil {
		return err
	}
	b.tmpContainers[container.ID] = struct{}{}

	if err := container.Mount(); err != nil {
		return err
	}
	defer container.Unmount()

	if !allowDecompression || isRemote {
		decompress = false
	}
	if err := b.addContext(container, origPath, destPath, decompress); err != nil {
		return err
	}

	if err := b.commit(container.ID, cmd, fmt.Sprintf("%s %s in %s", cmdName, orig, dest)); err != nil {
		return err
	}
	return nil
}

func (b *BuildFile) create() (*daemon.Container, error) {
	if b.image == "" {
		return nil, fmt.Errorf("Please provide a source image with `from` prior to run")
	}
	b.config.Image = b.image

	// Create the container
	c, _, err := b.daemon.Create(b.config, "")
	if err != nil {
		return nil, err
	}
	b.tmpContainers[c.ID] = struct{}{}
	fmt.Fprintf(b.outStream, " ---> Running in %s\n", utils.TruncateID(c.ID))

	// override the entry point that may have been picked up from the base image
	c.Path = b.config.Cmd[0]
	c.Args = b.config.Cmd[1:]

	return c, nil
}

func (b *BuildFile) run(c *daemon.Container) error {
	var errCh chan error
	if b.verbose {
		errCh = utils.Go(func() error {
			return <-b.daemon.Attach(c, nil, nil, b.outStream, b.errStream)
		})
	}

	//start the container
	if err := c.Start(); err != nil {
		return err
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			return err
		}
	}

	// Wait for it to finish
	if ret, _ := c.State.WaitStop(-1 * time.Second); ret != 0 {
		err := &utils.JSONError{
			Message: fmt.Sprintf("The command %v returned a non-zero code: %d", b.config.Cmd, ret),
			Code:    ret,
		}
		return err
	}

	return nil
}

// Commit the container <id> with the autorun command <autoCmd>
func (b *BuildFile) commit(id string, autoCmd []string, comment string) error {
	if b.image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to commit")
	}
	b.config.Image = b.image
	if id == "" {
		cmd := b.config.Cmd
		b.config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + comment}
		defer func(cmd []string) { b.config.Cmd = cmd }(cmd)

		hit, err := b.probeCache()
		if err != nil {
			return err
		}
		if hit {
			return nil
		}

		container, warnings, err := b.daemon.Create(b.config, "")
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			fmt.Fprintf(b.outStream, " ---> [Warning] %s\n", warning)
		}
		b.tmpContainers[container.ID] = struct{}{}
		fmt.Fprintf(b.outStream, " ---> Running in %s\n", utils.TruncateID(container.ID))
		id = container.ID

		if err := container.Mount(); err != nil {
			return err
		}
		defer container.Unmount()
	}
	container := b.daemon.Get(id)
	if container == nil {
		return fmt.Errorf("An error occured while creating the container")
	}

	// Note: Actually copy the struct
	autoConfig := *b.config
	autoConfig.Cmd = autoCmd
	// Commit the container
	image, err := b.daemon.Commit(container, "", "", "", b.maintainer, true, &autoConfig)
	if err != nil {
		return err
	}
	b.tmpImages[image.ID] = struct{}{}
	b.image = image.ID
	return nil
}

// probeCache checks to see if image-caching is enabled (`b.utilizeCache`)
// and if so attempts to look up the current `b.image` and `b.config` pair
// in the current server `b.daemon`. If an image is found, probeCache returns
// `(true, nil)`. If no image is found, it returns `(false, nil)`. If there
// is any error, it returns `(false, err)`.
func (b *BuildFile) probeCache() (bool, error) {
	if b.utilizeCache {
		if cache, err := b.daemon.ImageGetCached(b.image, b.config); err != nil {
			return false, err
		} else if cache != nil {
			fmt.Fprintf(b.outStream, " ---> Using cache\n")
			utils.Debugf("[BUILDER] Use cached version")
			b.image = cache.ID
			return true, nil
		} else {
			utils.Debugf("[BUILDER] Cache miss")
		}
	}
	return false, nil
}
