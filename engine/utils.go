package engine

import (
	"testing"
	"runtime"
	"strings"
	"fmt"
	"io/ioutil"
	"github.com/dotcloud/docker/utils"
)

var globalTestID string

func init() {
	Register("dummy", func(job *Job) string { return ""; })
}

func newTestEngine(t *testing.T) *Engine {
	// Use the caller function name as a prefix.
	// This helps trace temp directories back to their test.
	pc, _, _, _ := runtime.Caller(1)
	callerLongName := runtime.FuncForPC(pc).Name()
	parts := strings.Split(callerLongName, ".")
	callerShortName := parts[len(parts)-1]
	if globalTestID == "" {
		globalTestID = utils.RandomString()[:4]
	}
	prefix := fmt.Sprintf("docker-test%s-%s-", globalTestID, callerShortName)
	root, err := ioutil.TempDir("", prefix)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

func mkJob(t *testing.T, name string, args ...string) *Job {
	return newTestEngine(t).Job(name, args...)
}
