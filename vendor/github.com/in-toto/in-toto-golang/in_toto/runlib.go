package in_toto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/shibumi/go-pathspec"
)

// ErrSymCycle signals a detected symlink cycle in our RecordArtifacts() function.
var ErrSymCycle = errors.New("symlink cycle detected")

// ErrUnsupportedHashAlgorithm signals a missing hash mapping in getHashMapping
var ErrUnsupportedHashAlgorithm = errors.New("unsupported hash algorithm detected")

var ErrEmptyCommandArgs = errors.New("the command args are empty")

// visitedSymlinks is a hashset that contains all paths that we have visited.
var visitedSymlinks Set

/*
RecordArtifact reads and hashes the contents of the file at the passed path
using sha256 and returns a map in the following format:

	{
		"<path>": {
			"sha256": <hex representation of hash>
		}
	}

If reading the file fails, the first return value is nil and the second return
value is the error.
NOTE: For cross-platform consistency Windows-style line separators (CRLF) are
normalized to Unix-style line separators (LF) before hashing file contents.
*/
func RecordArtifact(path string, hashAlgorithms []string, lineNormalization bool) (map[string]interface{}, error) {
	supportedHashMappings := getHashMapping()
	// Read file from passed path
	contents, err := os.ReadFile(path)
	hashedContentsMap := make(map[string]interface{})
	if err != nil {
		return nil, err
	}

	if lineNormalization {
		// "Normalize" file contents. We convert all line separators to '\n'
		// for keeping operating system independence
		contents = bytes.ReplaceAll(contents, []byte("\r\n"), []byte("\n"))
		contents = bytes.ReplaceAll(contents, []byte("\r"), []byte("\n"))
	}

	// Create a map of all the hashes present in the hash_func list
	for _, element := range hashAlgorithms {
		if _, ok := supportedHashMappings[element]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedHashAlgorithm, element)
		}
		h := supportedHashMappings[element]
		result := fmt.Sprintf("%x", hashToHex(h(), contents))
		hashedContentsMap[element] = result
	}

	// Return it in a format that is conformant with link metadata artifacts
	return hashedContentsMap, nil
}

/*
RecordArtifacts is a wrapper around recordArtifacts.
RecordArtifacts initializes a set for storing visited symlinks,
calls recordArtifacts and deletes the set if no longer needed.
recordArtifacts walks through the passed slice of paths, traversing
subdirectories, and calls RecordArtifact for each file. It returns a map in
the following format:

	{
		"<path>": {
			"sha256": <hex representation of hash>
		},
		"<path>": {
		"sha256": <hex representation of hash>
		},
		...
	}

If recording an artifact fails the first return value is nil and the second
return value is the error.
*/
func RecordArtifacts(paths []string, hashAlgorithms []string, gitignorePatterns []string, lStripPaths []string, lineNormalization bool, followSymlinkDirs bool) (evalArtifacts map[string]interface{}, err error) {
	// Make sure to initialize a fresh hashset for every RecordArtifacts call
	visitedSymlinks = NewSet()
	evalArtifacts, err = recordArtifacts(paths, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
	// pass result and error through
	return evalArtifacts, err
}

/*
recordArtifacts walks through the passed slice of paths, traversing
subdirectories, and calls RecordArtifact for each file. It returns a map in
the following format:

	{
		"<path>": {
			"sha256": <hex representation of hash>
		},
		"<path>": {
		"sha256": <hex representation of hash>
		},
		...
	}

If recording an artifact fails the first return value is nil and the second
return value is the error.
*/
func recordArtifacts(paths []string, hashAlgorithms []string, gitignorePatterns []string, lStripPaths []string, lineNormalization bool, followSymlinkDirs bool) (map[string]interface{}, error) {
	artifacts := make(map[string]interface{})
	for _, path := range paths {
		err := filepath.Walk(path,
			func(path string, info os.FileInfo, err error) error {
				// Abort if Walk function has a problem,
				// e.g. path does not exist
				if err != nil {
					return err
				}
				// We need to call pathspec.GitIgnore inside of our filepath.Walk, because otherwise
				// we will not catch all paths. Just imagine a path like "." and a pattern like "*.pub".
				// If we would call pathspec outside of the filepath.Walk this would not match.
				ignore, err := pathspec.GitIgnore(gitignorePatterns, path)
				if err != nil {
					return err
				}
				if ignore {
					return nil
				}
				// Don't hash directories
				if info.IsDir() {
					return nil
				}

				// check for symlink and evaluate the last element in a symlink
				// chain via filepath.EvalSymlinks. We use EvalSymlinks here,
				// because with os.Readlink() we would just read the next
				// element in a possible symlink chain. This would mean more
				// iterations. infoMode()&os.ModeSymlink uses the file
				// type bitmask to check for a symlink.
				if info.Mode()&os.ModeSymlink == os.ModeSymlink {
					// return with error if we detect a symlink cycle
					if ok := visitedSymlinks.Has(path); ok {
						// this error will get passed through
						// to RecordArtifacts()
						return ErrSymCycle
					}
					evalSym, err := filepath.EvalSymlinks(path)
					if err != nil {
						return err
					}
					info, err := os.Stat(evalSym)
					if err != nil {
						return err
					}
					targetIsDir := false
					if info.IsDir() {
						if !followSymlinkDirs {
							// We don't follow symlinked directories
							return nil
						}
						targetIsDir = true
					}
					// add symlink to visitedSymlinks set
					// this way, we know which link we have visited already
					// if we visit a symlink twice, we have detected a symlink cycle
					visitedSymlinks.Add(path)
					// We recursively call recordArtifacts() to follow
					// the new path.
					evalArtifacts, evalErr := recordArtifacts([]string{evalSym}, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
					if evalErr != nil {
						return evalErr
					}
					for key, value := range evalArtifacts {
						if targetIsDir {
							symlinkPath := filepath.Join(path, strings.TrimPrefix(key, evalSym))
							artifacts[symlinkPath] = value
						} else {
							artifacts[path] = value
						}
					}
					return nil
				}
				artifact, err := RecordArtifact(path, hashAlgorithms, lineNormalization)
				// Abort if artifact can't be recorded, e.g.
				// due to file permissions
				if err != nil {
					return err
				}

				for _, strip := range lStripPaths {
					if strings.HasPrefix(path, strip) {
						path = strings.TrimPrefix(path, strip)
						break
					}
				}
				// Check if path is unique
				if _, exists := artifacts[path]; exists {
					return fmt.Errorf("left stripping has resulted in non unique dictionary key: %s", path)
				}
				artifacts[path] = artifact
				return nil
			})

		if err != nil {
			return nil, err
		}
	}

	return artifacts, nil
}

/*
waitErrToExitCode converts an error returned by Cmd.wait() to an exit code.  It
returns -1 if no exit code can be inferred.
*/
func waitErrToExitCode(err error) int {
	// If there's no exit code, we return -1
	retVal := -1

	// See https://stackoverflow.com/questions/10385551/get-exit-code-go
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				retVal = status.ExitStatus()
			}
		}
	} else {
		retVal = 0
	}

	return retVal
}

/*
RunCommand executes the passed command in a subprocess.  The first element of
cmdArgs is used as executable and the rest as command arguments.  It captures
and returns stdout, stderr and exit code.  The format of the returned map is:

	{
		"return-value": <exit code>,
		"stdout": "<standard output>",
		"stderr": "<standard error>"
	}

If the command cannot be executed or no pipes for stdout or stderr can be
created the first return value is nil and the second return value is the error.
NOTE: Since stdout and stderr are captured, they cannot be seen during the
command execution.
*/
func RunCommand(cmdArgs []string, runDir string) (map[string]interface{}, error) {
	if len(cmdArgs) == 0 {
		return nil, ErrEmptyCommandArgs
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	if runDir != "" {
		cmd.Dir = runDir
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// TODO: duplicate stdout, stderr
	stdout, _ := io.ReadAll(stdoutPipe)
	stderr, _ := io.ReadAll(stderrPipe)

	retVal := waitErrToExitCode(cmd.Wait())

	return map[string]interface{}{
		"return-value": float64(retVal),
		"stdout":       string(stdout),
		"stderr":       string(stderr),
	}, nil
}

/*
InTotoRun executes commands, e.g. for software supply chain steps or
inspections of an in-toto layout, and creates and returns corresponding link
metadata.  Link metadata contains recorded products at the passed productPaths
and materials at the passed materialPaths.  The returned link is wrapped in a
Metablock object.  If command execution or artifact recording fails the first
return value is an empty Metablock and the second return value is the error.
*/
func InTotoRun(name string, runDir string, materialPaths []string, productPaths []string, cmdArgs []string, key Key, hashAlgorithms []string, gitignorePatterns []string, lStripPaths []string, lineNormalization bool, followSymlinkDirs bool, useDSSE bool) (Metadata, error) {
	materials, err := RecordArtifacts(materialPaths, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
	if err != nil {
		return nil, err
	}

	// make sure that we only run RunCommand if cmdArgs is not nil or empty
	byProducts := map[string]interface{}{}
	if len(cmdArgs) != 0 {
		byProducts, err = RunCommand(cmdArgs, runDir)
		if err != nil {
			return nil, err
		}
	}

	products, err := RecordArtifacts(productPaths, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
	if err != nil {
		return nil, err
	}

	link := Link{
		Type:        "link",
		Name:        name,
		Materials:   materials,
		Products:    products,
		ByProducts:  byProducts,
		Command:     cmdArgs,
		Environment: map[string]interface{}{},
	}

	if useDSSE {
		env := &Envelope{}
		if err := env.SetPayload(link); err != nil {
			return nil, err
		}

		if !reflect.ValueOf(key).IsZero() {
			if err := env.Sign(key); err != nil {
				return nil, err
			}
		}

		return env, nil
	}

	linkMb := &Metablock{Signed: link, Signatures: []Signature{}}
	if !reflect.ValueOf(key).IsZero() {
		if err := linkMb.Sign(key); err != nil {
			return nil, err
		}
	}

	return linkMb, nil
}

/*
InTotoRecordStart begins the creation of a link metablock file in two steps,
in order to provide evidence for supply chain steps that cannot be carries out
by a single command.  InTotoRecordStart collects the hashes of the materials
before any commands are run, signs the unfinished link, and returns the link.
*/
func InTotoRecordStart(name string, materialPaths []string, key Key, hashAlgorithms, gitignorePatterns []string, lStripPaths []string, lineNormalization bool, followSymlinkDirs bool, useDSSE bool) (Metadata, error) {
	materials, err := RecordArtifacts(materialPaths, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
	if err != nil {
		return nil, err
	}

	link := Link{
		Type:        "link",
		Name:        name,
		Materials:   materials,
		Products:    map[string]interface{}{},
		ByProducts:  map[string]interface{}{},
		Command:     []string{},
		Environment: map[string]interface{}{},
	}

	if useDSSE {
		env := &Envelope{}
		if err := env.SetPayload(link); err != nil {
			return nil, err
		}

		if !reflect.ValueOf(key).IsZero() {
			if err := env.Sign(key); err != nil {
				return nil, err
			}
		}

		return env, nil
	}

	linkMb := &Metablock{Signed: link, Signatures: []Signature{}}
	linkMb.Signatures = []Signature{}
	if !reflect.ValueOf(key).IsZero() {
		if err := linkMb.Sign(key); err != nil {
			return nil, err
		}
	}

	return linkMb, nil
}

/*
InTotoRecordStop ends the creation of a metatadata link file created by
InTotoRecordStart. InTotoRecordStop takes in a signed unfinished link metablock
created by InTotoRecordStart and records the hashes of any products creted by
commands run between InTotoRecordStart and InTotoRecordStop.  The resultant
finished link metablock is then signed by the provided key and returned.
*/
func InTotoRecordStop(prelimLinkEnv Metadata, productPaths []string, key Key, hashAlgorithms, gitignorePatterns []string, lStripPaths []string, lineNormalization bool, followSymlinkDirs bool, useDSSE bool) (Metadata, error) {
	if err := prelimLinkEnv.VerifySignature(key); err != nil {
		return nil, err
	}

	link, ok := prelimLinkEnv.GetPayload().(Link)
	if !ok {
		return nil, errors.New("invalid metadata block")
	}

	products, err := RecordArtifacts(productPaths, hashAlgorithms, gitignorePatterns, lStripPaths, lineNormalization, followSymlinkDirs)
	if err != nil {
		return nil, err
	}

	link.Products = products

	if useDSSE {
		env := &Envelope{}
		if err := env.SetPayload(link); err != nil {
			return nil, err
		}

		if !reflect.ValueOf(key).IsZero() {
			if err := env.Sign(key); err != nil {
				return nil, err
			}
		}

		return env, nil
	}

	linkMb := &Metablock{Signed: link, Signatures: []Signature{}}
	if !reflect.ValueOf(key).IsZero() {
		if err := linkMb.Sign(key); err != nil {
			return linkMb, err
		}
	}

	return linkMb, nil
}
