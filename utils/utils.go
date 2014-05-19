package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"index/suffixarray"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dotcloud/docker/dockerversion"
)

type KeyValuePair struct {
	Key   string
	Value string
}

// A common interface to access the Fatal method of
// both testing.B and testing.T.
type Fataler interface {
	Fatal(args ...interface{})
}

// Go is a basic promise implementation: it wraps calls a function in a goroutine,
// and returns a channel which will later return the function's return value.
func Go(f func() error) chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- f()
	}()
	return ch
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

func logf(level string, format string, a ...interface{}) {
	// Retrieve the stack infos
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "<unknown>"
		line = -1
	} else {
		file = file[strings.LastIndex(file, "/")+1:]
	}

	fmt.Fprintf(os.Stderr, fmt.Sprintf("[%s] %s:%d %s\n", level, file, line, format), a...)
}

// Debug function, if the debug flag is set, then display. Do nothing otherwise
// If Docker is in damon mode, also send the debug info on the socket
func Debugf(format string, a ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		logf("debug", format, a...)
	}
}

func Errorf(format string, a ...interface{}) {
	logf("error", format, a...)
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

type NopWriter struct{}

func (*NopWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}

type bufReader struct {
	sync.Mutex
	buf    *bytes.Buffer
	reader io.Reader
	err    error
	wait   sync.Cond
}

func NewBufReader(r io.Reader) *bufReader {
	reader := &bufReader{
		buf:    &bytes.Buffer{},
		reader: r,
	}
	reader.wait.L = &reader.Mutex
	go reader.drain()
	return reader
}

func (r *bufReader) drain() {
	buf := make([]byte, 1024)
	for {
		n, err := r.reader.Read(buf)
		r.Lock()
		if err != nil {
			r.err = err
		} else {
			r.buf.Write(buf[0:n])
		}
		r.wait.Signal()
		r.Unlock()
		if err != nil {
			break
		}
	}
}

func (r *bufReader) Read(p []byte) (n int, err error) {
	r.Lock()
	defer r.Unlock()
	for {
		n, err = r.buf.Read(p)
		if n > 0 {
			return n, err
		}
		if r.err != nil {
			return 0, r.err
		}
		r.wait.Wait()
	}
}

func (r *bufReader) Close() error {
	closer, ok := r.reader.(io.ReadCloser)
	if !ok {
		return nil
	}
	return closer.Close()
}

type WriteBroadcaster struct {
	sync.Mutex
	buf     *bytes.Buffer
	streams map[string](map[io.WriteCloser]struct{})
}

func (w *WriteBroadcaster) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	if _, ok := w.streams[stream]; !ok {
		w.streams[stream] = make(map[io.WriteCloser]struct{})
	}
	w.streams[stream][writer] = struct{}{}
	w.Unlock()
}

type JSONLog struct {
	Log     string    `json:"log,omitempty"`
	Stream  string    `json:"stream,omitempty"`
	Created time.Time `json:"time"`
}

func (jl *JSONLog) Format(format string) (string, error) {
	if format == "" {
		return jl.Log, nil
	}
	if format == "json" {
		m, err := json.Marshal(jl)
		return string(m), err
	}
	return fmt.Sprintf("[%s] %s", jl.Created.Format(format), jl.Log), nil
}

func WriteLog(src io.Reader, dst io.WriteCloser, format string) error {
	dec := json.NewDecoder(src)
	for {
		l := &JSONLog{}

		if err := dec.Decode(l); err == io.EOF {
			return nil
		} else if err != nil {
			Errorf("Error streaming logs: %s", err)
			return err
		}
		line, err := l.Format(format)
		if err != nil {
			return err
		}
		fmt.Fprintf(dst, "%s", line)
	}
}

type LogFormatter struct {
	wc         io.WriteCloser
	timeFormat string
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	created := time.Now().UTC()
	w.Lock()
	defer w.Unlock()
	if writers, ok := w.streams[""]; ok {
		for sw := range writers {
			if n, err := sw.Write(p); err != nil || n != len(p) {
				// On error, evict the writer
				delete(writers, sw)
			}
		}
	}
	w.buf.Write(p)
	lines := []string{}
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.Write([]byte(line))
			break
		}
		lines = append(lines, line)
	}

	if len(lines) != 0 {
		for stream, writers := range w.streams {
			if stream == "" {
				continue
			}
			var lp []byte
			for _, line := range lines {
				b, err := json.Marshal(&JSONLog{Log: line, Stream: stream, Created: created})
				if err != nil {
					Errorf("Error making JSON log line: %s", err)
				}
				lp = append(lp, b...)
				lp = append(lp, '\n')
			}
			for sw := range writers {
				if _, err := sw.Write(lp); err != nil {
					delete(writers, sw)
				}
			}
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriters() error {
	w.Lock()
	defer w.Unlock()
	for _, writers := range w.streams {
		for w := range writers {
			w.Close()
		}
	}
	w.streams = make(map[string](map[io.WriteCloser]struct{}))
	return nil
}

func NewWriteBroadcaster() *WriteBroadcaster {
	return &WriteBroadcaster{
		streams: make(map[string](map[io.WriteCloser]struct{})),
		buf:     bytes.NewBuffer(nil),
	}
}

func GetTotalUsedFds() int {
	if fds, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", os.Getpid())); err != nil {
		Errorf("Error opening /proc/%d/fd: %s", os.Getpid(), err)
	} else {
		return len(fds)
	}
	return -1
}

// TruncIndex allows the retrieval of string identifiers by any of their unique prefixes.
// This is used to retrieve image and container IDs by more convenient shorthand prefixes.
type TruncIndex struct {
	sync.RWMutex
	index *suffixarray.Index
	ids   map[string]bool
	bytes []byte
}

func NewTruncIndex(ids []string) (idx *TruncIndex) {
	idx = &TruncIndex{
		ids:   make(map[string]bool),
		bytes: []byte{' '},
	}
	for _, id := range ids {
		idx.ids[id] = true
		idx.bytes = append(idx.bytes, []byte(id+" ")...)
	}
	idx.index = suffixarray.New(idx.bytes)
	return
}

func (idx *TruncIndex) addId(id string) error {
	if strings.Contains(id, " ") {
		return fmt.Errorf("Illegal character: ' '")
	}
	if _, exists := idx.ids[id]; exists {
		return fmt.Errorf("Id already exists: %s", id)
	}
	idx.ids[id] = true
	idx.bytes = append(idx.bytes, []byte(id+" ")...)
	return nil
}

func (idx *TruncIndex) Add(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if err := idx.addId(id); err != nil {
		return err
	}
	idx.index = suffixarray.New(idx.bytes)
	return nil
}

func (idx *TruncIndex) AddWithoutSuffixarrayUpdate(id string) error {
	idx.Lock()
	defer idx.Unlock()
	return idx.addId(id)
}

func (idx *TruncIndex) UpdateSuffixarray() {
	idx.Lock()
	defer idx.Unlock()
	idx.index = suffixarray.New(idx.bytes)
}

func (idx *TruncIndex) Delete(id string) error {
	idx.Lock()
	defer idx.Unlock()
	if _, exists := idx.ids[id]; !exists {
		return fmt.Errorf("No such id: %s", id)
	}
	before, after, err := idx.lookup(id)
	if err != nil {
		return err
	}
	delete(idx.ids, id)
	idx.bytes = append(idx.bytes[:before], idx.bytes[after:]...)
	idx.index = suffixarray.New(idx.bytes)
	return nil
}

func (idx *TruncIndex) lookup(s string) (int, int, error) {
	offsets := idx.index.Lookup([]byte(" "+s), -1)
	//log.Printf("lookup(%s): %v (index bytes: '%s')\n", s, offsets, idx.index.Bytes())
	if offsets == nil || len(offsets) == 0 || len(offsets) > 1 {
		return -1, -1, fmt.Errorf("No such id: %s", s)
	}
	offsetBefore := offsets[0] + 1
	offsetAfter := offsetBefore + strings.Index(string(idx.bytes[offsetBefore:]), " ")
	return offsetBefore, offsetAfter, nil
}

func (idx *TruncIndex) Get(s string) (string, error) {
	idx.RLock()
	defer idx.RUnlock()
	before, after, err := idx.lookup(s)
	//log.Printf("Get(%s) bytes=|%s| before=|%d| after=|%d|\n", s, idx.bytes, before, after)
	if err != nil {
		return "", err
	}
	return string(idx.bytes[before:after]), err
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
		if _, err := strconv.Atoi(TruncateID(value)); err == nil {
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

type KernelVersionInfo struct {
	Kernel int
	Major  int
	Minor  int
	Flavor string
}

func (k *KernelVersionInfo) String() string {
	return fmt.Sprintf("%d.%d.%d%s", k.Kernel, k.Major, k.Minor, k.Flavor)
}

// Compare two KernelVersionInfo struct.
// Returns -1 if a < b, 0 if a == b, 1 it a > b
func CompareKernelVersion(a, b *KernelVersionInfo) int {
	if a.Kernel < b.Kernel {
		return -1
	} else if a.Kernel > b.Kernel {
		return 1
	}

	if a.Major < b.Major {
		return -1
	} else if a.Major > b.Major {
		return 1
	}

	if a.Minor < b.Minor {
		return -1
	} else if a.Minor > b.Minor {
		return 1
	}

	return 0
}

func GetKernelVersion() (*KernelVersionInfo, error) {
	var (
		err error
	)

	uts, err := uname()
	if err != nil {
		return nil, err
	}

	release := make([]byte, len(uts.Release))

	i := 0
	for _, c := range uts.Release {
		release[i] = byte(c)
		i++
	}

	// Remove the \x00 from the release for Atoi to parse correctly
	release = release[:bytes.IndexByte(release, 0)]

	return ParseRelease(string(release))
}

func ParseRelease(release string) (*KernelVersionInfo, error) {
	var (
		kernel, major, minor, parsed int
		flavor, partial              string
	)

	// Ignore error from Sscanf to allow an empty flavor.  Instead, just
	// make sure we got all the version numbers.
	parsed, _ = fmt.Sscanf(release, "%d.%d%s", &kernel, &major, &partial)
	if parsed < 2 {
		return nil, errors.New("Can't parse kernel version " + release)
	}

	// sometimes we have 3.12.25-gentoo, but sometimes we just have 3.12-1-amd64
	parsed, _ = fmt.Sscanf(partial, ".%d%s", &minor, &flavor)
	if parsed < 1 {
		flavor = partial
	}

	return &KernelVersionInfo{
		Kernel: kernel,
		Major:  major,
		Minor:  minor,
		Flavor: flavor,
	}, nil
}

// FIXME: this is deprecated by CopyWithTar in archive.go
func CopyDirectory(source, dest string) error {
	if output, err := exec.Command("cp", "-ra", source, dest).CombinedOutput(); err != nil {
		return fmt.Errorf("Error copy: %s (%s)", err, output)
	}
	return nil
}

type NopFlusher struct{}

func (f *NopFlusher) Flush() {}

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
		flusher = &NopFlusher{}
	}
	return &WriteFlusher{w: w, flusher: flusher}
}

func NewHTTPRequestError(msg string, res *http.Response) error {
	return &JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}

func IsURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

func IsGIT(str string) bool {
	return strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "github.com/") || strings.HasPrefix(str, "git@github.com:") || (strings.HasSuffix(str, ".git") && IsURL(str))
}

// CheckLocalDns looks into the /etc/resolv.conf,
// it returns true if there is a local nameserver or if there is no nameserver.
func CheckLocalDns(resolvConf []byte) bool {
	for _, line := range GetLines(resolvConf, []byte("#")) {
		if !bytes.Contains(line, []byte("nameserver")) {
			continue
		}
		for _, ip := range [][]byte{
			[]byte("127.0.0.1"),
			[]byte("127.0.1.1"),
		} {
			if bytes.Contains(line, ip) {
				return true
			}
		}
		return false
	}
	return true
}

// GetLines parses input into lines and strips away comments.
func GetLines(input []byte, commentMarker []byte) [][]byte {
	lines := bytes.Split(input, []byte("\n"))
	var output [][]byte
	for _, currentLine := range lines {
		var commentIndex = bytes.Index(currentLine, commentMarker)
		if commentIndex == -1 {
			output = append(output, currentLine)
		} else {
			output = append(output, currentLine[:commentIndex])
		}
	}
	return output
}

// FIXME: Change this not to receive default value as parameter
func ParseHost(defaultHost string, defaultUnix, addr string) (string, error) {
	var (
		proto string
		host  string
		port  int
	)
	addr = strings.TrimSpace(addr)
	switch {
	case addr == "tcp://":
		return "", fmt.Errorf("Invalid bind address format: %s", addr)
	case strings.HasPrefix(addr, "unix://"):
		proto = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
		if addr == "" {
			addr = defaultUnix
		}
	case strings.HasPrefix(addr, "tcp://"):
		proto = "tcp"
		addr = strings.TrimPrefix(addr, "tcp://")
	case strings.HasPrefix(addr, "fd://"):
		return addr, nil
	case addr == "":
		proto = "unix"
		addr = defaultUnix
	default:
		if strings.Contains(addr, "://") {
			return "", fmt.Errorf("Invalid bind address protocol: %s", addr)
		}
		proto = "tcp"
	}

	if proto != "unix" && strings.Contains(addr, ":") {
		hostParts := strings.Split(addr, ":")
		if len(hostParts) != 2 {
			return "", fmt.Errorf("Invalid bind address format: %s", addr)
		}
		if hostParts[0] != "" {
			host = hostParts[0]
		} else {
			host = defaultHost
		}

		if p, err := strconv.Atoi(hostParts[1]); err == nil && p != 0 {
			port = p
		} else {
			return "", fmt.Errorf("Invalid bind address format: %s", addr)
		}

	} else if proto == "tcp" && !strings.Contains(addr, ":") {
		return "", fmt.Errorf("Invalid bind address format: %s", addr)
	} else {
		host = addr
	}
	if proto == "unix" {
		return fmt.Sprintf("%s://%s", proto, host), nil
	}
	return fmt.Sprintf("%s://%s:%d", proto, host, port), nil
}

// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
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

func PartParser(template, data string) (map[string]string, error) {
	// ip:public:private
	var (
		templateParts = strings.Split(template, ":")
		parts         = strings.Split(data, ":")
		out           = make(map[string]string, len(templateParts))
	)
	if len(parts) != len(templateParts) {
		return nil, fmt.Errorf("Invalid format to parse.  %s should match template %s", data, template)
	}

	for i, t := range templateParts {
		value := ""
		if len(parts) > i {
			value = parts[i]
		}
		out[t] = value
	}
	return out, nil
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
		if err = CopyDirectory(templateDir, dir); err != nil {
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

type readCloserWrapper struct {
	io.Reader
	closer func() error
}

func (r *readCloserWrapper) Close() error {
	return r.closer()
}

func NewReadCloserWrapper(r io.Reader, closer func() error) io.ReadCloser {
	return &readCloserWrapper{
		Reader: r,
		closer: closer,
	}
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

func ParseKeyValueOpt(opt string) (string, string, error) {
	parts := strings.SplitN(opt, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Unable to parse key/value option: %s", opt)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// TreeSize walks a directory tree and returns its total size in bytes.
func TreeSize(dir string) (size int64, err error) {
	data := make(map[uint64]struct{})
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, e error) error {
		// Ignore directory sizes
		if fileInfo == nil {
			return nil
		}

		s := fileInfo.Size()
		if fileInfo.IsDir() || s == 0 {
			return nil
		}

		// Check inode to handle hard links correctly
		inode := fileInfo.Sys().(*syscall.Stat_t).Ino
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		if _, exists := data[uint64(inode)]; exists {
			return nil
		}
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		data[uint64(inode)] = struct{}{}

		size += s

		return nil
	})
	return
}

// ValidateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read
// symlinks which point to non-existing files don't trigger an error
func ValidateContextDirectory(srcPath string) error {
	var finalError error

	filepath.Walk(filepath.Join(srcPath, "."), func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		_, err = filepath.Rel(srcPath, filePath)
		if err != nil && os.IsPermission(err) {
			return nil
		}

		if _, err := os.Stat(filePath); err != nil && os.IsPermission(err) {
			finalError = fmt.Errorf("can't stat '%s'", filePath)
			return err
		}
		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		lstat, _ := os.Lstat(filePath)
		if lstat.Mode()&os.ModeSymlink == os.ModeSymlink {
			return err
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				finalError = fmt.Errorf("no permission to read from '%s'", filePath)
				return err
			} else {
				currentFile.Close()
			}
		}
		return nil
	})
	return finalError
}
