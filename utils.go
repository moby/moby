package docker

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/rcli"
	"index/suffixarray"
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
	"syscall"
	"time"
)

// Go is a basic promise implementation: it wraps calls a function in a goroutine,
// and returns a channel which will later return the function's return value.
func Go(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

// Request a given URL and return an io.Reader
func Download(url string, stderr io.Writer) (*http.Response, error) {
	var resp *http.Response
	var err error = nil
	if resp, err = http.Get(url); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New("Got HTTP status code >= 400: " + resp.Status)
	}
	return resp, nil
}

// Debug function, if the debug flag is set, then display. Do nothing otherwise
// If Docker is in damon mode, also send the debug info on the socket
func Debugf(format string, a ...interface{}) {
	if os.Getenv("DEBUG") != "" {

		// Retrieve the stack infos
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "<unknown>"
			line = -1
		} else {
			file = file[strings.LastIndex(file, "/")+1:]
		}

		fmt.Fprintf(os.Stderr, fmt.Sprintf("[debug] %s:%d %s\n", file, line, format), a...)
		if rcli.CLIENT_SOCKET != nil {
			fmt.Fprintf(rcli.CLIENT_SOCKET, fmt.Sprintf("[debug] %s:%d %s\n", file, line, format), a...)
		}
	}
}

// Reader with progress bar
type progressReader struct {
	reader       io.ReadCloser // Stream to read from
	output       io.Writer     // Where to send progress bar to
	readTotal    int           // Expected stream length (bytes)
	readProgress int           // How much has been read so far (bytes)
	lastUpdate   int           // How many bytes read at least update
	template     string        // Template to print. Default "%v/%v (%v)"
}

func (r *progressReader) Read(p []byte) (n int, err error) {
	read, err := io.ReadCloser(r.reader).Read(p)
	r.readProgress += read

	updateEvery := 4096
	if r.readTotal > 0 {
		// Only update progress for every 1% read
		if increment := int(0.01 * float64(r.readTotal)); increment > updateEvery {
			updateEvery = increment
		}
	}
	if r.readProgress-r.lastUpdate > updateEvery || err != nil {
		if r.readTotal > 0 {
			fmt.Fprintf(r.output, r.template+"\r", r.readProgress, r.readTotal, fmt.Sprintf("%.0f%%", float64(r.readProgress)/float64(r.readTotal)*100))
		} else {
			fmt.Fprintf(r.output, r.template+"\r", r.readProgress, "?", "n/a")
		}
		r.lastUpdate = r.readProgress
	}
	// Send newline when complete
	if err != nil {
		fmt.Fprintf(r.output, "\n")
	}

	return read, err
}
func (r *progressReader) Close() error {
	return io.ReadCloser(r.reader).Close()
}
func ProgressReader(r io.ReadCloser, size int, output io.Writer, template string) *progressReader {
	if template == "" {
		template = "%v/%v (%v)"
	}
	return &progressReader{r, output, size, 0, 0, template}
}

// HumanDuration returns a human-readable approximation of a duration
// (eg. "About a minute", "4 hours ago", etc.)
func HumanDuration(d time.Duration) string {
	if seconds := int(d.Seconds()); seconds < 1 {
		return "Less than a second"
	} else if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	} else if minutes := int(d.Minutes()); minutes == 1 {
		return "About a minute"
	} else if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	} else if hours := int(d.Hours()); hours == 1 {
		return "About an hour"
	} else if hours < 48 {
		return fmt.Sprintf("%d hours", hours)
	} else if hours < 24*7*2 {
		return fmt.Sprintf("%d days", hours/24)
	} else if hours < 24*30*3 {
		return fmt.Sprintf("%d weeks", hours/24/7)
	} else if hours < 24*365*2 {
		return fmt.Sprintf("%d months", hours/24/30)
	}
	return fmt.Sprintf("%d years", d.Hours()/24/365)
}

func Trunc(s string, maxlen int) string {
	if len(s) <= maxlen {
		return s
	}
	return s[:maxlen]
}

// Figure out the absolute path of our own binary
func SelfPath() string {
	path, err := exec.LookPath(os.Args[0])
	if err != nil {
		panic(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return path
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}

type bufReader struct {
	buf    *bytes.Buffer
	reader io.Reader
	err    error
	l      sync.Mutex
	wait   sync.Cond
}

func newBufReader(r io.Reader) *bufReader {
	reader := &bufReader{
		buf:    &bytes.Buffer{},
		reader: r,
	}
	reader.wait.L = &reader.l
	go reader.drain()
	return reader
}

func (r *bufReader) drain() {
	buf := make([]byte, 1024)
	for {
		n, err := r.reader.Read(buf)
		r.l.Lock()
		if err != nil {
			r.err = err
		} else {
			r.buf.Write(buf[0:n])
		}
		r.wait.Signal()
		r.l.Unlock()
		if err != nil {
			break
		}
	}
}

func (r *bufReader) Read(p []byte) (n int, err error) {
	r.l.Lock()
	defer r.l.Unlock()
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
	panic("unreachable")
}

func (r *bufReader) Close() error {
	closer, ok := r.reader.(io.ReadCloser)
	if !ok {
		return nil
	}
	return closer.Close()
}

type writeBroadcaster struct {
	mu      sync.Mutex
	writers map[io.WriteCloser]struct{}
}

func (w *writeBroadcaster) AddWriter(writer io.WriteCloser) {
	w.mu.Lock()
	w.writers[writer] = struct{}{}
	w.mu.Unlock()
}

// FIXME: Is that function used?
// FIXME: This relies on the concrete writer type used having equality operator
func (w *writeBroadcaster) RemoveWriter(writer io.WriteCloser) {
	w.mu.Lock()
	delete(w.writers, writer)
	w.mu.Unlock()
}

func (w *writeBroadcaster) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for writer := range w.writers {
		if n, err := writer.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			delete(w.writers, writer)
		}
	}
	return len(p), nil
}

func (w *writeBroadcaster) CloseWriters() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for writer := range w.writers {
		writer.Close()
	}
	w.writers = make(map[io.WriteCloser]struct{})
	return nil
}

func newWriteBroadcaster() *writeBroadcaster {
	return &writeBroadcaster{writers: make(map[io.WriteCloser]struct{})}
}

func getTotalUsedFds() int {
	if fds, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", os.Getpid())); err != nil {
		Debugf("Error opening /proc/%d/fd: %s", os.Getpid(), err)
	} else {
		return len(fds)
	}
	return -1
}

// TruncIndex allows the retrieval of string identifiers by any of their unique prefixes.
// This is used to retrieve image and container IDs by more convenient shorthand prefixes.
type TruncIndex struct {
	index *suffixarray.Index
	ids   map[string]bool
	bytes []byte
}

func NewTruncIndex() *TruncIndex {
	return &TruncIndex{
		index: suffixarray.New([]byte{' '}),
		ids:   make(map[string]bool),
		bytes: []byte{' '},
	}
}

func (idx *TruncIndex) Add(id string) error {
	if strings.Contains(id, " ") {
		return fmt.Errorf("Illegal character: ' '")
	}
	if _, exists := idx.ids[id]; exists {
		return fmt.Errorf("Id already exists: %s", id)
	}
	idx.ids[id] = true
	idx.bytes = append(idx.bytes, []byte(id+" ")...)
	idx.index = suffixarray.New(idx.bytes)
	return nil
}

func (idx *TruncIndex) Delete(id string) error {
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
	before, after, err := idx.lookup(s)
	//log.Printf("Get(%s) bytes=|%s| before=|%d| after=|%d|\n", s, idx.bytes, before, after)
	if err != nil {
		return "", err
	}
	return string(idx.bytes[before:after]), err
}

// TruncateId returns a shorthand version of a string identifier for convenience.
// A collision with other shorthands is very unlikely, but possible.
// In case of a collision a lookup with TruncIndex.Get() will fail, and the caller
// will need to use a langer prefix, or the full-length Id.
func TruncateId(id string) string {
	shortLen := 12
	if len(id) < shortLen {
		shortLen = len(id)
	}
	return id[:shortLen]
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
					return 0, io.EOF
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

type KernelVersionInfo struct {
	Kernel int
	Major  int
	Minor  int
	Flavor string
}

// FIXME: this doens't build on Darwin
func GetKernelVersion() (*KernelVersionInfo, error) {
	var uts syscall.Utsname

	if err := syscall.Uname(&uts); err != nil {
		return nil, err
	}

	release := make([]byte, len(uts.Release))

	i := 0
	for _, c := range uts.Release {
		release[i] = byte(c)
		i++
	}

	tmp := strings.SplitN(string(release), "-", 2)
	if len(tmp) != 2 {
		return nil, fmt.Errorf("Unrecognized kernel version")
	}
	tmp2 := strings.SplitN(tmp[0], ".", 3)
	if len(tmp2) != 3 {
		return nil, fmt.Errorf("Unrecognized kernel version")
	}

	kernel, err := strconv.Atoi(tmp2[0])
	if err != nil {
		return nil, err
	}

	major, err := strconv.Atoi(tmp2[1])
	if err != nil {
		return nil, err
	}

	minor, err := strconv.Atoi(tmp2[2])
	if err != nil {
		return nil, err
	}

	flavor := tmp[1]

	return &KernelVersionInfo{
		Kernel: kernel,
		Major:  major,
		Minor:  minor,
		Flavor: flavor,
	}, nil
}

func (k *KernelVersionInfo) String() string {
	return fmt.Sprintf("%d.%d.%d-%s", k.Kernel, k.Major, k.Minor, k.Flavor)
}

// Compare two KernelVersionInfo struct.
// Returns -1 if a < b, = if a == b, 1 it a > b
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

func FindCgroupMountpoint(cgroupType string) (string, error) {
	output, err := exec.Command("mount").CombinedOutput()
	if err != nil {
		return "", err
	}

	reg := regexp.MustCompile(`^cgroup on (.*) type cgroup \(.*` + cgroupType + `[,\)]`)
	for _, line := range strings.Split(string(output), "\n") {
		r := reg.FindStringSubmatch(line)
		if len(r) == 2 {
			return r[1], nil
		}
		fmt.Printf("line: %s (%d)\n", line, len(r))
	}
	return "", fmt.Errorf("cgroup mountpoint not found")
}
