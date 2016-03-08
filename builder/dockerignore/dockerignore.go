package dockerignore

import (
	"bufio"
	"fmt"
	"github.com/docker/docker/pkg/fileutils"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// ReadAll reads a .dockerignore file and returns the list of file patterns
// to ignore. Note this will trim whitespace from each line as well
// as use GO's "clean" func to get the shortest/cleanest path for each.
func ReadAll(reader io.ReadCloser) ([]string, error) {
	if reader == nil {
		return nil, nil
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	var excludes []string

	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if pattern == "" {
			continue
		}
		pattern = filepath.Clean(pattern)
		pattern = filepath.ToSlash(pattern)
		excludes = append(excludes, pattern)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Error reading .dockerignore: %v", err)
	}
	return excludes, nil
}

// ReadAllRecursive takes a base directory and a starting directory,
// reads all .dockerignore files in subdirectories and returns the list of file
// patterns to ignore. Note this will trim whitespace from each line as well
// as use GO's "clean" func to get the shortest/cleanest path for each.
func ReadAllRecursive(baseDir string, directory string) ([]string, error) {

	var excludes []string

	directory = filepath.Clean(directory)
	directory = filepath.ToSlash(directory)

	dockerIgnFName := filepath.Join(directory, ".dockerignore")
	reader, err := os.Open(dockerIgnFName)
	// Note that a missing .dockerignore file isn't treated as an error
	if err != nil {
		// If file does not exist, ignore, and continue recursing as needed
		// If file exists, and there is an error reading it, fail.
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		// If reader is nil, it means that something odd happened:
		// lack of permissions, etc.
		if reader == nil {
			return nil, nil
		}

		ignFiles, err := ReadAll(reader)
		if err != nil {
			return nil, err
		}

		relDirectory, err := filepath.Rel(baseDir, directory)
		if err != nil {
			return nil, err
		}

		for i := range ignFiles {
			isException := ignFiles[i][0] == '!'
			if isException && len(ignFiles[i]) > 1 {
				ignFiles[i] = "!" + filepath.Clean(filepath.Join(relDirectory, ignFiles[i][1:len(ignFiles[i])]))
			} else {
				ignFiles[i] = filepath.Clean(filepath.Join(relDirectory, ignFiles[i]))
			}
		}
		excludes = append(excludes, ignFiles...)
	}

	files, _ := ioutil.ReadDir(directory)
	for _, f := range files {
		if f.IsDir() {
			// Calculate subDir path
			subDir := filepath.Clean(filepath.Join(directory, f.Name()))
			// Calculate relative subDir path
			relSubDir, _ := filepath.Rel(baseDir, subDir)
			// Make sure relative subDir path not in exclusions.
			matches, _ := fileutils.Matches(relSubDir, excludes)
			// If not excluded, recurse in
			if !matches {
				subDirIgnFiles, err := ReadAllRecursive(baseDir, subDir)
				if err == nil {
					excludes = append(excludes, subDirIgnFiles...)
				} else {
					return nil, err
				}
			}
		}
	}

	return excludes, nil
}
