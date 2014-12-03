package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/ioutils"
)

type KeyValuePair struct {
	Key   string
	Value string
}

// Request a given URL and return an io.Reader
func Download(url string) (resp *http.Response, err error) {
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Got HTTP status code >= 400: %s", resp.Status)
	}
	return resp, nil
}

func Trunc(s string, maxlen int) string {
	if len(s) <= maxlen {
		return s
	}
	return s[:maxlen]
}

// Figure out the absolute path of our own binary (if it's still around).
func SelfPath() string {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		if execErr, ok := err.(*exec.Error); ok && os.IsNotExist(execErr.Err) {
			return ""
		}
		panic(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		panic(err)
	}
	return path
}

func dockerInitSha1(target string) string {
	f, err := os.Open(target)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

func isValidDockerInitPath(target string, selfPath string) bool { // target and selfPath should be absolute (InitPath and SelfPath already do this)
	if target == "" {
		return false
	}
	if dockerversion.IAMSTATIC {
		if selfPath == "" {
			return false
		}
		if target == selfPath {
			return true
		}
		targetFileInfo, err := os.Lstat(target)
		if err != nil {
			return false
		}
		selfPathFileInfo, err := os.Lstat(selfPath)
		if err != nil {
			return false
		}
		return os.SameFile(targetFileInfo, selfPathFileInfo)
	}
	return dockerversion.INITSHA1 != "" && dockerInitSha1(target) == dockerversion.INITSHA1
}

// Figure out the path of our dockerinit (which may be SelfPath())
func DockerInitPath(localCopy string) string {
	selfPath := SelfPath()
	if isValidDockerInitPath(selfPath, selfPath) {
		// if we're valid, don't bother checking anything else
		return selfPath
	}
	var possibleInits = []string{
		localCopy,
		dockerversion.INITPATH,
		filepath.Join(filepath.Dir(selfPath), "dockerinit"),

		// FHS 3.0 Draft: "/usr/libexec includes internal binaries that are not intended to be executed directly by users or shell scripts. Applications may use a single subdirectory under /usr/libexec."
		// http://www.linuxbase.org/betaspecs/fhs/fhs.html#usrlibexec
		"/usr/libexec/docker/dockerinit",
		"/usr/local/libexec/docker/dockerinit",

		// FHS 2.3: "/usr/lib includes object files, libraries, and internal binaries that are not intended to be executed directly by users or shell scripts."
		// http://refspecs.linuxfoundation.org/FHS_2.3/fhs-2.3.html#USRLIBLIBRARIESFORPROGRAMMINGANDPA
		"/usr/lib/docker/dockerinit",
		"/usr/local/lib/docker/dockerinit",
	}
	for _, dockerInit := range possibleInits {
		if dockerInit == "" {
			continue
		}
		path, err := exec.LookPath(dockerInit)
		if err == nil {
			path, err = filepath.Abs(path)
			if err != nil {
				// LookPath already validated that this file exists and is executable (following symlinks), so how could Abs fail?
				panic(err)
			}
			if isValidDockerInitPath(path, selfPath) {
				return path
			}
		}
	}
	return ""
}

func GetTotalUsedFds() int {
	if fds, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", os.Getpid())); err != nil {
		log.Errorf("Error opening /proc/%d/fd: %s", os.Getpid(), err)
	} else {
		return len(fds)
	}
	return -1
}

// TruncateID returns a shorthand version of a string identifier for convenience.
// A collision with other shorthands is very unlikely, but possible.
// In case of a collision a lookup with TruncIndex.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length Id.
func TruncateID(id string) string {
	shortLen := 12
	if len(id) < shortLen {
		shortLen = len(id)
	}
	return id[:shortLen]
}

// GenerateRandomID returns an unique id
func GenerateRandomID() string {
	for {
		id := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, id); err != nil {
			panic(err) // This shouldn't happen
		}
		value := hex.EncodeToString(id)
		// if we try to parse the truncated for as an int and we don't have
		// an error then the value is all numberic and causes issues when
		// used as a hostname. ref #3869
		if _, err := strconv.ParseInt(TruncateID(value), 10, 64); err == nil {
			continue
		}
		return value
	}
}

func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("Id can't be empty")
	}
	if strings.Contains(id, ":") {
		return fmt.Errorf("Invalid character in id: ':'")
	}
	return nil
}

// Code c/c from io.Copy() modified to handle escape sequence
func CopyEscapable(dst io.Writer, src io.ReadCloser) (written int64, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			// ---- Docker addition
			// char 16 is C-p
			if nr == 1 && buf[0] == 16 {
				nr, er = src.Read(buf)
				// char 17 is C-q
				if nr == 1 && buf[0] == 17 {
					if err := src.Close(); err != nil {
						return 0, err
					}
					return 0, nil
				}
			}
			// ---- End of docker
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

func HashData(src io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

type WriteFlusher struct {
	sync.Mutex
	w       io.Writer
	flusher http.Flusher
}

func (wf *WriteFlusher) Write(b []byte) (n int, err error) {
	wf.Lock()
	defer wf.Unlock()
	n, err = wf.w.Write(b)
	wf.flusher.Flush()
	return n, err
}

// Flush the stream immediately.
func (wf *WriteFlusher) Flush() {
	wf.Lock()
	defer wf.Unlock()
	wf.flusher.Flush()
}

func NewWriteFlusher(w io.Writer) *WriteFlusher {
	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	} else {
		flusher = &ioutils.NopFlusher{}
	}
	return &WriteFlusher{w: w, flusher: flusher}
}

func NewHTTPRequestError(msg string, res *http.Response) error {
	return &JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}

var localHostRx = regexp.MustCompile(`(?m)^nameserver 127[^\n]+\n*`)

// RemoveLocalDns looks into the /etc/resolv.conf,
// and removes any local nameserver entries.
func RemoveLocalDns(resolvConf []byte) []byte {
	return localHostRx.ReplaceAll(resolvConf, []byte{})
}

// An StatusError reports an unsuccessful exit by a command.
type StatusError struct {
	Status     string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("Status: %s, Code: %d", e.Status, e.StatusCode)
}

func quote(word string, buf *bytes.Buffer) {
	// Bail out early for "simple" strings
	if word != "" && !strings.ContainsAny(word, "\\'\"`${[|&;<>()~*?! \t\n") {
		buf.WriteString(word)
		return
	}

	buf.WriteString("'")

	for i := 0; i < len(word); i++ {
		b := word[i]
		if b == '\'' {
			// Replace literal ' with a close ', a \', and a open '
			buf.WriteString("'\\''")
		} else {
			buf.WriteByte(b)
		}
	}

	buf.WriteString("'")
}

// Take a list of strings and escape them so they will be handled right
// when passed as arguments to an program via a shell
func ShellQuoteArguments(args []string) string {
	var buf bytes.Buffer
	for i, arg := range args {
		if i != 0 {
			buf.WriteByte(' ')
		}
		quote(arg, &buf)
	}
	return buf.String()
}

var globalTestID string

// TestDirectory creates a new temporary directory and returns its path.
// The contents of directory at path `templateDir` is copied into the
// new directory.
func TestDirectory(templateDir string) (dir string, err error) {
	if globalTestID == "" {
		globalTestID = RandomString()[:4]
	}
	prefix := fmt.Sprintf("docker-test%s-%s-", globalTestID, GetCallerName(2))
	if prefix == "" {
		prefix = "docker-test-"
	}
	dir, err = ioutil.TempDir("", prefix)
	if err = os.Remove(dir); err != nil {
		return
	}
	if templateDir != "" {
		if err = archive.CopyWithTar(templateDir, dir); err != nil {
			return
		}
	}
	return
}

// GetCallerName introspects the call stack and returns the name of the
// function `depth` levels down in the stack.
func GetCallerName(depth int) string {
	// Use the caller function name as a prefix.
	// This helps trace temp directories back to their test.
	pc, _, _, _ := runtime.Caller(depth + 1)
	callerLongName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(callerLongName, ".")
	callerShortName := parts[len(parts)-1]
	return callerShortName
}

func CopyFile(src, dst string) (int64, error) {
	if src == dst {
		return 0, nil
	}
	sf, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer sf.Close()
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	df, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer df.Close()
	return io.Copy(df, sf)
}

// ReplaceOrAppendValues returns the defaults with the overrides either
// replaced by env key or appended to the list
func ReplaceOrAppendEnvValues(defaults, overrides []string) []string {
	cache := make(map[string]int, len(defaults))
	for i, e := range defaults {
		parts := strings.SplitN(e, "=", 2)
		cache[parts[0]] = i
	}
	for _, value := range overrides {
		parts := strings.SplitN(value, "=", 2)
		if i, exists := cache[parts[0]]; exists {
			defaults[i] = value
		} else {
			defaults = append(defaults, value)
		}
	}
	return defaults
}

// ReadSymlinkedDirectory returns the target directory of a symlink.
// The target of the symbolic link may not be a file.
func ReadSymlinkedDirectory(path string) (string, error) {
	var realPath string
	var err error
	if realPath, err = filepath.Abs(path); err != nil {
		return "", fmt.Errorf("unable to get absolute path for %s: %s", path, err)
	}
	if realPath, err = filepath.EvalSymlinks(realPath); err != nil {
		return "", fmt.Errorf("failed to canonicalise path for %s: %s", path, err)
	}
	realPathInfo, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat target '%s' of '%s': %s", realPath, path, err)
	}
	if !realPathInfo.Mode().IsDir() {
		return "", fmt.Errorf("canonical path points to a file '%s'", realPath)
	}
	return realPath, nil
}

// ValidateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read
// symlinks which point to non-existing files don't trigger an error
func ValidateContextDirectory(srcPath string, excludes []string) error {
	return filepath.Walk(filepath.Join(srcPath, "."), func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(srcPath, filePath); err != nil {
			return err
		} else if skip, err := fileutils.Matches(relFilePath, excludes); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hanging on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return fmt.Errorf("no permission to read from '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}

func StringsContainsNoCase(slice []string, s string) bool {
	for _, ss := range slice {
		if strings.ToLower(s) == strings.ToLower(ss) {
			return true
		}
	}
	return false
}
