package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"index/suffixarray"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	var err error
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
	sf           *StreamFormatter
	newLine      bool
}

func (r *progressReader) Read(p []byte) (n int, err error) {
	read, err := io.ReadCloser(r.reader).Read(p)
	r.readProgress += read
	updateEvery := 1024 * 512 //512kB
	if r.readTotal > 0 {
		// Update progress for every 1% read if 1% < 512kB
		if increment := int(0.01 * float64(r.readTotal)); increment < updateEvery {
			updateEvery = increment
		}
	}
	if r.readProgress-r.lastUpdate > updateEvery || err != nil {
		if r.readTotal > 0 {
			fmt.Fprintf(r.output, r.template, HumanSize(int64(r.readProgress)), HumanSize(int64(r.readTotal)), fmt.Sprintf("%.0f%%", float64(r.readProgress)/float64(r.readTotal)*100))
		} else {
			fmt.Fprintf(r.output, r.template, r.readProgress, "?", "n/a")
		}
		r.lastUpdate = r.readProgress
	}
	// Send newline when complete
	if r.newLine && err != nil {
		r.output.Write(r.sf.FormatStatus("", ""))
	}
	return read, err
}
func (r *progressReader) Close() error {
	return io.ReadCloser(r.reader).Close()
}
func ProgressReader(r io.ReadCloser, size int, output io.Writer, tpl []byte, sf *StreamFormatter, newline bool) *progressReader {
	return &progressReader{
		reader:    r,
		output:    NewWriteFlusher(output),
		readTotal: size,
		template:  string(tpl),
		sf:        sf,
		newLine:   newline,
	}
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
	return fmt.Sprintf("%f years", d.Hours()/24/365)
}

// HumanSize returns a human-readable approximation of a size
// using SI standard (eg. "44kB", "17MB")
func HumanSize(size int64) string {
	i := 0
	var sizef float64
	sizef = float64(size)
	units := []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	for sizef >= 1000.0 {
		sizef = sizef / 1000.0
		i++
	}
	return fmt.Sprintf("%.4g %s", sizef, units[i])
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
	writers map[StreamWriter]bool
}

type StreamWriter struct {
	wc     io.WriteCloser
	stream string
}

func (w *WriteBroadcaster) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	sw := StreamWriter{wc: writer, stream: stream}
	w.writers[sw] = true
	w.Unlock()
}

type JSONLog struct {
	Log     string    `json:"log,omitempty"`
	Stream  string    `json:"stream,omitempty"`
	Created time.Time `json:"time"`
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()
	w.buf.Write(p)
	for sw := range w.writers {
		lp := p
		if sw.stream != "" {
			lp = nil
			for {
				line, err := w.buf.ReadString('\n')
				if err != nil {
					w.buf.Write([]byte(line))
					break
				}
				b, err := json.Marshal(&JSONLog{Log: line, Stream: sw.stream, Created: time.Now()})
				if err != nil {
					// On error, evict the writer
					delete(w.writers, sw)
					continue
				}
				lp = append(lp, b...)
			}
		}
		if n, err := sw.wc.Write(lp); err != nil || n != len(lp) {
			// On error, evict the writer
			delete(w.writers, sw)
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriters() error {
	w.Lock()
	defer w.Unlock()
	for sw := range w.writers {
		sw.wc.Close()
	}
	w.writers = make(map[StreamWriter]bool)
	return nil
}

func NewWriteBroadcaster() *WriteBroadcaster {
	return &WriteBroadcaster{writers: make(map[StreamWriter]bool), buf: bytes.NewBuffer(nil)}
}

func GetTotalUsedFds() int {
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
	flavor := ""
	if len(k.Flavor) > 0 {
		flavor = fmt.Sprintf("-%s", k.Flavor)
	}
	return fmt.Sprintf("%d.%d.%d%s", k.Kernel, k.Major, k.Minor, flavor)
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
	output, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return "", err
	}

	// /proc/mounts has 6 fields per line, one mount per line, e.g.
	// cgroup /sys/fs/cgroup/devices cgroup rw,relatime,devices 0 0
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Split(line, " ")
		if len(parts) == 6 && parts[2] == "cgroup" {
			for _, opt := range strings.Split(parts[3], ",") {
				if opt == cgroupType {
					return parts[1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("cgroup mountpoint not found for %s", cgroupType)
}

func GetKernelVersion() (*KernelVersionInfo, error) {
	var (
		flavor               string
		kernel, major, minor int
		err                  error
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

	tmp := strings.SplitN(string(release), "-", 2)
	tmp2 := strings.SplitN(tmp[0], ".", 3)

	if len(tmp2) > 0 {
		kernel, err = strconv.Atoi(tmp2[0])
		if err != nil {
			return nil, err
		}
	}

	if len(tmp2) > 1 {
		major, err = strconv.Atoi(tmp2[1])
		if err != nil {
			return nil, err
		}
	}

	if len(tmp2) > 2 {
		// Removes "+" because git kernels might set it
		minorUnparsed := strings.Trim(tmp2[2], "+")
		minor, err = strconv.Atoi(minorUnparsed)
		if err != nil {
			return nil, err
		}
	}

	if len(tmp) == 2 {
		flavor = tmp[1]
	} else {
		flavor = ""
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

func NewWriteFlusher(w io.Writer) *WriteFlusher {
	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	} else {
		flusher = &NopFlusher{}
	}
	return &WriteFlusher{w: w, flusher: flusher}
}

type JSONError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type JSONMessage struct {
	Status       string     `json:"status,omitempty"`
	Progress     string     `json:"progress,omitempty"`
	ErrorMessage string     `json:"error,omitempty"` //deprecated
	ID           string     `json:"id,omitempty"`
	From         string     `json:"from,omitempty"`
	Time         int64      `json:"time,omitempty"`
	Error        *JSONError `json:"errorDetail,omitempty"`
}

func (e *JSONError) Error() string {
	return e.Message
}

func NewHTTPRequestError(msg string, res *http.Response) error {
	return &JSONError{
		Message: msg,
		Code:    res.StatusCode,
	}
}

func (jm *JSONMessage) Display(out io.Writer) error {
	if jm.Error != nil {
		if jm.Error.Code == 401 {
			return fmt.Errorf("Authentication is required.")
		}
		return jm.Error
	}
	fmt.Fprintf(out, "%c[2K\r", 27)
	if jm.Time != 0 {
		fmt.Fprintf(out, "[%s] ", time.Unix(jm.Time, 0))
	}
	if jm.ID != "" {
		fmt.Fprintf(out, "%s: ", jm.ID)
	}
	if jm.From != "" {
		fmt.Fprintf(out, "(from %s) ", jm.From)
	}
	if jm.Progress != "" {
		fmt.Fprintf(out, "%s %s\r", jm.Status, jm.Progress)
	} else {
		fmt.Fprintf(out, "%s\r\n", jm.Status)
	}
	return nil
}

func DisplayJSONMessagesStream(in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	ids := make(map[string]int)
	diff := 0
	for {
		jm := JSONMessage{}
		if err := dec.Decode(&jm); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if jm.Progress != "" && jm.ID != "" {
			line, ok := ids[jm.ID]
			if !ok {
				line = len(ids)
				ids[jm.ID] = line
				fmt.Fprintf(out, "\n")
				diff = 0
			} else {
				diff = len(ids) - line
			}
			fmt.Fprintf(out, "%c[%dA", 27, diff)
		}
		err := jm.Display(out)
		if jm.ID != "" {
			fmt.Fprintf(out, "%c[%dB", 27, diff)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type StreamFormatter struct {
	json bool
	used bool
}

func NewStreamFormatter(json bool) *StreamFormatter {
	return &StreamFormatter{json, false}
}

func (sf *StreamFormatter) FormatStatus(id, format string, a ...interface{}) []byte {
	sf.used = true
	str := fmt.Sprintf(format, a...)
	if sf.json {
		b, err := json.Marshal(&JSONMessage{ID: id, Status: str})
		if err != nil {
			return sf.FormatError(err)
		}
		return b
	}
	return []byte(str + "\r\n")
}

func (sf *StreamFormatter) FormatError(err error) []byte {
	sf.used = true
	if sf.json {
		jsonError, ok := err.(*JSONError)
		if !ok {
			jsonError = &JSONError{Message: err.Error()}
		}
		if b, err := json.Marshal(&JSONMessage{Error: jsonError, ErrorMessage: err.Error()}); err == nil {
			return b
		}
		return []byte("{\"error\":\"format error\"}")
	}
	return []byte("Error: " + err.Error() + "\r\n")
}

func (sf *StreamFormatter) FormatProgress(id, action, progress string) []byte {
	sf.used = true
	if sf.json {
		b, err := json.Marshal(&JSONMessage{Status: action, Progress: progress, ID: id})
		if err != nil {
			return nil
		}
		return b
	}
	return []byte(action + " " + progress + "\r")
}

func (sf *StreamFormatter) Used() bool {
	return sf.used
}

func IsURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

func IsGIT(str string) bool {
	return strings.HasPrefix(str, "git://") || strings.HasPrefix(str, "github.com/")
}

// GetResolvConf opens and read the content of /etc/resolv.conf.
// It returns it as byte slice.
func GetResolvConf() ([]byte, error) {
	resolv, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		Debugf("Error openning resolv.conf: %s", err)
		return nil, err
	}
	return resolv, nil
}

// CheckLocalDns looks into the /etc/resolv.conf,
// it returns true if there is a local nameserver or if there is no nameserver.
func CheckLocalDns(resolvConf []byte) bool {
	var parsedResolvConf = StripComments(resolvConf, []byte("#"))
	if !bytes.Contains(parsedResolvConf, []byte("nameserver")) {
		return true
	}
	for _, ip := range [][]byte{
		[]byte("127.0.0.1"),
		[]byte("127.0.1.1"),
	} {
		if bytes.Contains(parsedResolvConf, ip) {
			return true
		}
	}
	return false
}

// StripComments parses input into lines and strips away comments.
func StripComments(input []byte, commentMarker []byte) []byte {
	lines := bytes.Split(input, []byte("\n"))
	var output []byte
	for _, currentLine := range lines {
		var commentIndex = bytes.Index(currentLine, commentMarker)
		if ( commentIndex == -1 ) {
			output = append(output, currentLine...)
		} else {
			output = append(output, currentLine[:commentIndex]...)
		}
		output = append(output, []byte("\n")...)
	}
	return output
}

func ParseHost(host string, port int, addr string) string {
	if strings.HasPrefix(addr, "unix://") {
		return addr
	}
	if strings.HasPrefix(addr, "tcp://") {
		addr = strings.TrimPrefix(addr, "tcp://")
	}
	if strings.Contains(addr, ":") {
		hostParts := strings.Split(addr, ":")
		if len(hostParts) != 2 {
			log.Fatal("Invalid bind address format.")
			os.Exit(-1)
		}
		if hostParts[0] != "" {
			host = hostParts[0]
		}
		if p, err := strconv.Atoi(hostParts[1]); err == nil {
			port = p
		}
	} else {
		host = addr
	}
	return fmt.Sprintf("tcp://%s:%d", host, port)
}

func GetReleaseVersion() string {
	resp, err := http.Get("http://get.docker.io/latest")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.ContentLength > 24 || resp.StatusCode != 200 {
		return ""
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
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

// UserLookup check if the given username or uid is present in /etc/passwd
// and returns the user struct.
// If the username is not found, an error is returned.
func UserLookup(uid string) (*user.User, error) {
	file, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(file), "\n") {
		data := strings.Split(line, ":")
		if len(data) > 5 && (data[0] == uid || data[2] == uid) {
			return &user.User{
				Uid:      data[2],
				Gid:      data[3],
				Username: data[0],
				Name:     data[4],
				HomeDir:  data[5],
			}, nil
		}
	}
	return nil, fmt.Errorf("User not found in /etc/passwd")
}
