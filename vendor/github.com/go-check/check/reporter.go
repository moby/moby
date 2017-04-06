package check

import (
	"fmt"
	"io"
	"sync"
)

// -----------------------------------------------------------------------
// Output writer manages atomic output writing according to settings.

type outputWriter struct {
	m                    sync.Mutex
	writer               io.Writer
	wroteCallProblemLast bool
	Stream               bool
	Verbose              bool
}

func newOutputWriter(writer io.Writer, stream, verbose bool) *outputWriter {
	return &outputWriter{writer: writer, Stream: stream, Verbose: verbose}
}

func (ow *outputWriter) Write(content []byte) (n int, err error) {
	ow.m.Lock()
	n, err = ow.writer.Write(content)
	ow.m.Unlock()
	return
}

func (ow *outputWriter) WriteCallStarted(label string, c *C) {
	if ow.Stream {
		header := renderCallHeader(label, c, "", "\n")
		ow.m.Lock()
		ow.writer.Write([]byte(header))
		ow.m.Unlock()
	}
}

func (ow *outputWriter) WriteCallProblem(label string, c *C) {
	var prefix string
	if !ow.Stream {
		prefix = "\n-----------------------------------" +
			"-----------------------------------\n"
	}
	header := renderCallHeader(label, c, prefix, "\n\n")
	ow.m.Lock()
	ow.wroteCallProblemLast = true
	ow.writer.Write([]byte(header))
	if !ow.Stream {
		c.logb.WriteTo(ow.writer)
	}
	ow.m.Unlock()
}

func (ow *outputWriter) WriteCallSuccess(label string, c *C) {
	if ow.Stream || (ow.Verbose && c.kind == testKd) {
		// TODO Use a buffer here.
		var suffix string
		if c.reason != "" {
			suffix = " (" + c.reason + ")"
		}
		if c.status() == succeededSt {
			suffix += "\t" + c.timerString()
		}
		suffix += "\n"
		if ow.Stream {
			suffix += "\n"
		}
		header := renderCallHeader(label, c, "", suffix)
		ow.m.Lock()
		// Resist temptation of using line as prefix above due to race.
		if !ow.Stream && ow.wroteCallProblemLast {
			header = "\n-----------------------------------" +
				"-----------------------------------\n" +
				header
		}
		ow.wroteCallProblemLast = false
		ow.writer.Write([]byte(header))
		ow.m.Unlock()
	}
}

func renderCallHeader(label string, c *C, prefix, suffix string) string {
	pc := c.method.PC()
	return fmt.Sprintf("%s%s: %s: %s%s", prefix, label, niceFuncPath(pc),
		niceFuncName(pc), suffix)
}
