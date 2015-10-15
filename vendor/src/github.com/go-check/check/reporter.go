package check

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type outputWriter interface {
	Write(content []byte) (n int, err error)
	WriteCallStarted(label string, c *C)

	WriteCallSuccess(label string, c *C)
	WriteCallSkipped(label string, c *C)

	WriteCallError(label string, c *C)
	WriteCallFailure(label string, c *C)
	StreamEnabled() bool

	PrintReport(*Result) error
}

/*************** Plain writer *****************/

type plainWriter struct {
	outputWriter
	m                    sync.Mutex
	writer               io.Writer
	wroteCallProblemLast bool
	stream               bool
	verbose              bool
}

func newPlainWriter(writer io.Writer, verbose, stream bool) *plainWriter {
	return &plainWriter{writer: writer, stream: stream, verbose: verbose}
}

func (w *plainWriter) StreamEnabled() bool { return w.stream }

func (w *plainWriter) PrintReport(r *Result) error {
	fmt.Fprintf(w.writer, "%s\n", r.String())
	return nil
}

func (w *plainWriter) Write(content []byte) (n int, err error) {
	w.m.Lock()
	n, err = w.writer.Write(content)
	w.m.Unlock()
	return
}

func (w *plainWriter) WriteCallStarted(label string, c *C) {
	if w.stream {
		header := renderCallHeader(label, c, "", "\n")
		w.m.Lock()
		w.writer.Write([]byte(header))
		w.m.Unlock()
	}
}

func (w *plainWriter) WriteCallSkipped(label string, c *C) {
	w.writeSuccess(label, c)
}

func (w *plainWriter) WriteCallFailure(label string, c *C) {
	w.writeProblem(label, c)
}

func (w *plainWriter) WriteCallError(label string, c *C) {
	w.writeProblem(label, c)
}

func (w *plainWriter) WriteCallSuccess(label string, c *C) {
	w.writeSuccess(label, c)
}

func (w *plainWriter) writeProblem(label string, c *C) {
	var prefix string
	if !w.stream {
		prefix = "\n-----------------------------------" +
			"-----------------------------------\n"
	}
	header := renderCallHeader(label, c, prefix, "\n\n")
	w.m.Lock()
	w.wroteCallProblemLast = true
	w.writer.Write([]byte(header))
	if !w.stream {
		c.logb.WriteTo(w.writer)
	}
	w.m.Unlock()
}

func (w *plainWriter) writeSuccess(label string, c *C) {
	if w.stream || (w.verbose && c.kind == testKd) {
		// TODO Use a buffer here.
		var suffix string
		if c.reason != "" {
			suffix = " (" + c.reason + ")"
		}
		if c.status() == succeededSt {
			suffix += "\t" + c.timerString()
		}
		suffix += "\n"
		if w.stream {
			suffix += "\n"
		}
		header := renderCallHeader(label, c, "", suffix)
		w.m.Lock()
		// Resist temptation of using line as prefix above due to race.
		if !w.stream && w.wroteCallProblemLast {
			header = "\n-----------------------------------" +
				"-----------------------------------\n" +
				header
		}
		w.wroteCallProblemLast = false
		w.writer.Write([]byte(header))
		w.m.Unlock()
	}
}

func renderCallHeader(label string, c *C, prefix, suffix string) string {
	pc := c.method.PC()
	return fmt.Sprintf("%s%s: %s: %s%s", prefix, label, niceFuncPath(pc),
		niceFuncName(pc), suffix)
}

/*************** xUnit writer *****************/
type xunitReport struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []xunitSuite `xml:"testsuite,omitempty"`
}

type xunitSuite struct {
	Package   string    `xml:"package,attr,omitempty"`
	Name      string    `xml:"name,attr,omitempty"`
	Classname string    `xml:"classname,attr,omitempty"`
	Time      float64   `xml:"time,attr"`
	Timestamp time.Time `xml:"timestamp,attr"`

	Tests    uint64 `xml:"tests,attr"`
	Failures uint64 `xml:"failures,attr"`
	Errors   uint64 `xml:"errors,attr"`
	Skipped  uint64 `xml:"skipped,attr"`

	// TODO: according specs suite also contains Properties node
	// but reporter has no use for it for now
	Testcases []xunitTestcase `xml:"testcase,omitempty"`

	// TODO: specs define also nodes "properties", "system-out" and "system-err"
	// but reporter has no use for them for now

	m sync.Mutex
}

func (s *xunitSuite) TestFail(tc xunitTestcase, message, value string) {
	tc.Failure = &xunitTestcaseResult{
		Message: message,
		Value:   value,
		Type:    "go.failure",
	}

	s.m.Lock()
	s.Failures++
	s.addTestCase(tc)
	s.m.Unlock()
}
func (s *xunitSuite) TestError(tc xunitTestcase, message, value string) {
	tc.Error = &xunitTestcaseResult{
		Message: message,
		Value:   value,
		Type:    "go.error",
	}

	s.m.Lock()
	s.Errors++
	s.addTestCase(tc)
	s.m.Unlock()
}

func (s *xunitSuite) TestSkip(tc xunitTestcase) {
	tc.Skipped = true

	s.m.Lock()
	s.Skipped++
	s.addTestCase(tc)
	s.m.Unlock()
}

func (s *xunitSuite) TestSuccess(tc xunitTestcase) {
	s.m.Lock()
	s.addTestCase(tc)
	s.m.Unlock()
}

func (s *xunitSuite) addTestCase(tc xunitTestcase) {
	s.Tests++
	s.Testcases = append(s.Testcases, tc)
	s.Time = time.Since(s.Timestamp).Seconds()
}

type xunitTestcase struct {
	Name      string  `xml:"name,attr,omitempty"`
	Classname string  `xml:"classname,attr,omitempty"`
	Time      float64 `xml:"time,attr"`

	File string `xml:"file,attr,omitempty"`
	Line int    `xml:"line,attr,omitempty"`

	Failure *xunitTestcaseResult `xml:"failure,omitempty"`
	Error   *xunitTestcaseResult `xml:"error,omitempty"`
	Skipped bool                 `xml:"skipped,omitempty"`
}

type xunitTestcaseResult struct {
	Message string `xml:"message,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	Value   string `xml:",innerxml"`
}

type xunitWriter struct {
	outputWriter
	m      sync.Mutex
	writer io.Writer
	stream bool
	suites map[string]*xunitSuite

	systemOut io.Writer
}

// creates new writer for xUnit reports
// "writer" here is used for logging purpose
func newXunitWriter(writer io.Writer, stream bool) *xunitWriter {
	return &xunitWriter{
		writer: writer,
		stream: stream,
		suites: make(map[string]*xunitSuite),
	}
}

func (w *xunitWriter) PrintReport(_ *Result) error {
	report := xunitReport{}
	report.Suites = make([]xunitSuite, 0, len(w.suites))
	for k := range w.suites {
		report.Suites = append(report.Suites, *w.suites[k])
	}

	rep, err := xml.MarshalIndent(report, "", "    ")
	if err != nil {
		return err
	}
	fmt.Fprintf(w.writer, "%s", string(rep))
	return nil
}

func (w *xunitWriter) Write(content []byte) (n int, err error) {
	if w.writer == nil {
		return
	}
	w.m.Lock()
	n, err = w.writer.Write(content)
	w.m.Unlock()
	return
}

func (w *xunitWriter) WriteCallStarted(label string, c *C) {
	w.getSuite(c) // init suite if not yet existing
}

func (w *xunitWriter) WriteCallSkipped(label string, c *C) {
	res := w.newTestcase(c)
	if !isAutogenerated(res.File) {
		w.getSuite(c).TestSkip(res)
	}
}

func (w *xunitWriter) WriteCallFailure(label string, c *C) {
	res := w.newTestcase(c)
	if !isAutogenerated(res.File) {
		message := strings.TrimSpace(c.logb.String())
		w.getSuite(c).TestFail(res, label, message)
	}
}

func (w *xunitWriter) WriteCallError(label string, c *C) {
	res := w.newTestcase(c)
	if !isAutogenerated(res.File) {
		message := strings.TrimSpace(c.logb.String())
		w.getSuite(c).TestError(res, label, message)
	}
}

func (w *xunitWriter) WriteCallSuccess(label string, c *C) {
	res := w.newTestcase(c)
	if !isAutogenerated(res.File) {
		w.getSuite(c).TestSuccess(res)
	}
}

func (w *xunitWriter) StreamEnabled() bool { return w.stream }

func (w *xunitWriter) getSuite(c *C) (suite *xunitSuite) {
	var ok bool
	suiteName := c.method.suiteName()
	w.m.Lock()
	if suite, ok = w.suites[suiteName]; !ok {
		suite = &xunitSuite{
			Name:      suiteName,
			Package:   getFuncPackage(c.method.PC()),
			Timestamp: c.startTime,
		}
		w.suites[suiteName] = suite
	}
	w.m.Unlock()

	return
}

func (w *xunitWriter) newTestcase(c *C) xunitTestcase {
	file, line := getFuncPosition(c.method.PC())
	return xunitTestcase{
		Name:      c.testName,
		Classname: c.method.suiteName(),
		File:      file,
		Line:      line,
		Time:      time.Since(c.startTime).Seconds(),
	}
}

func isAutogenerated(filename string) bool {
	return filename == "<autogenerated>"
}

/************ multiplexer writer ***************/

type multiplexerWriter struct {
	writers []outputWriter
}

func newMultiplexerWriter(plainWriter, xunitWriter io.Writer, verbose, stream bool) outputWriter {
	return &multiplexerWriter{
		writers: []outputWriter{
			newPlainWriter(plainWriter, verbose, stream),
			newXunitWriter(xunitWriter, stream),
		},
	}
}

func (w *multiplexerWriter) Write(content []byte) (n int, err error) {
	for _, writer := range w.writers {
		if _, err := writer.Write(content); err != nil {
			return 0, err
		}
	}
	return len(content), nil
}

func (w *multiplexerWriter) WriteCallStarted(label string, c *C) {
	for _, writer := range w.writers {
		writer.WriteCallStarted(label, c)
	}
}

func (w *multiplexerWriter) WriteCallSuccess(label string, c *C) {
	for _, writer := range w.writers {
		writer.WriteCallSuccess(label, c)
	}
}

func (w *multiplexerWriter) WriteCallSkipped(label string, c *C) {
	for _, writer := range w.writers {
		writer.WriteCallSkipped(label, c)
	}
}

func (w *multiplexerWriter) WriteCallError(label string, c *C) {
	for _, writer := range w.writers {
		writer.WriteCallError(label, c)
	}
}

func (w *multiplexerWriter) WriteCallFailure(label string, c *C) {
	for _, writer := range w.writers {
		writer.WriteCallFailure(label, c)
	}
}

func (w *multiplexerWriter) StreamEnabled() bool {
	return w.writers[0].StreamEnabled()
}

func (w *multiplexerWriter) PrintReport(r *Result) error {
	for _, writer := range w.writers {
		if err := writer.PrintReport(r); err != nil {
			return err
		}
	}
	return nil
}
