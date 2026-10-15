package testmain

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/cilium/ebpf/internal/platform"
)

type testingM interface {
	Run() int
}

// Run m with various debug aids enabled.
//
// The function calls [os.Exit] and does not return.
func Run(m testingM) {
	const traceLogFlag = "trace-log"

	var ts *traceSession
	if platform.IsWindows {
		traceLog := flag.Bool(traceLogFlag, false, "Output a trace of eBPF runtime activity")
		flag.Parse()

		if *traceLog {
			var err error
			ts, err = newTraceSession()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Disabling trace logging:", err)
			}
		}
	}
	defer ts.Close()

	fds = new(sync.Map)
	ret := m.Run()

	for _, f := range flushFrames() {
		onLeakFD(f)
	}

	if foundLeak.Load() {
		ret = 99
	}

	if err := ts.Dump(os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "Error while dumping trace log:", err)
		ret = 99
	}

	if platform.IsWindows && ret != 0 && ts == nil {
		fmt.Fprintf(os.Stderr, "Consider enabling trace logging with -%s\n", traceLogFlag)
	}

	os.Exit(ret)
}
