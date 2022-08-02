// Package fake implements a journal writer for testing which is decoupled from
// the system's journald.
//
// The systemd project does not have any facilities to support testing of
// journal reader clients (although it has been requested:
// https://github.com/systemd/systemd/issues/14120) so we have to get creative.
// The systemd-journal-remote command reads serialized journal entries in the
// Journal Export Format and writes them to journal files. This format is
// well-documented and straightforward to generate.
package fake // import "github.com/docker/docker/daemon/logger/journald/internal/fake"

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"testing"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/coreos/go-systemd/v22/journal"
	"gotest.tools/v3/assert"

	"github.com/docker/docker/daemon/logger/journald/internal/export"
)

// The systemd-journal-remote command is not conventionally installed on $PATH.
// The manpage from upstream systemd lists the command as
// /usr/lib/systemd/systemd-journal-remote, but Debian installs it to
// /lib/systemd instead.
var cmdPaths = []string{
	"/usr/lib/systemd/systemd-journal-remote",
	"/lib/systemd/systemd-journal-remote",
	"systemd-journal-remote", // Check $PATH anyway, just in case.
}

// ErrCommandNotFound is returned when the systemd-journal-remote command could
// not be located at the well-known paths or $PATH.
var ErrCommandNotFound = errors.New("systemd-journal-remote command not found")

// JournalRemoteCmdPath searches for the systemd-journal-remote command in
// well-known paths and the directories named in the $PATH environment variable.
func JournalRemoteCmdPath() (string, error) {
	for _, p := range cmdPaths {
		if path, err := exec.LookPath(p); err == nil {
			return path, nil
		}
	}
	return "", ErrCommandNotFound
}

// Sender fakes github.com/coreos/go-systemd/v22/journal.Send, writing journal
// entries to an arbitrary journal file without depending on a running journald
// process.
type Sender struct {
	CmdName    string
	OutputPath string

	// Clock for timestamping sent messages.
	Clock clock.Clock
	// Whether to assign the event's realtime timestamp to the time
	// specified by the SYSLOG_TIMESTAMP variable value. This is roughly
	// analogous to journald receiving the event and assigning it a
	// timestamp in zero time after the SYSLOG_TIMESTAMP value was set,
	// which is higly unrealistic in practice.
	AssignEventTimestampFromSyslogTimestamp bool
}

// New constructs a new Sender which will write journal entries to outpath. The
// file name must end in '.journal' and the directory must already exist. The
// journal file will be created if it does not exist. An existing journal file
// will be appended to.
func New(outpath string) (*Sender, error) {
	p, err := JournalRemoteCmdPath()
	if err != nil {
		return nil, err
	}
	sender := &Sender{
		CmdName:    p,
		OutputPath: outpath,
		Clock:      clock.NewClock(),
	}
	return sender, nil
}

// NewT is like New but will skip the test if the systemd-journal-remote command
// is not available.
func NewT(t *testing.T, outpath string) *Sender {
	t.Helper()
	s, err := New(outpath)
	if errors.Is(err, ErrCommandNotFound) {
		t.Skip(err)
	}
	assert.NilError(t, err)
	return s
}

var validVarName = regexp.MustCompile("^[A-Z0-9][A-Z0-9_]*$")

// Send is a drop-in replacement for
// github.com/coreos/go-systemd/v22/journal.Send.
func (s *Sender) Send(message string, priority journal.Priority, vars map[string]string) error {
	var buf bytes.Buffer
	// https://systemd.io/JOURNAL_EXPORT_FORMATS/ says "if you are
	// generating this format you shouldn’t care about these special
	// double-underscore fields," yet systemd-journal-remote treats entries
	// without a __REALTIME_TIMESTAMP as invalid and discards them.
	// Reported upstream: https://github.com/systemd/systemd/issues/22411
	var ts time.Time
	if sts := vars["SYSLOG_TIMESTAMP"]; s.AssignEventTimestampFromSyslogTimestamp && sts != "" {
		var err error
		if ts, err = time.Parse(time.RFC3339Nano, sts); err != nil {
			return fmt.Errorf("fake: error parsing SYSLOG_TIMESTAMP value %q: %w", ts, err)
		}
	} else {
		ts = s.Clock.Now()
	}
	if err := export.WriteField(&buf, "__REALTIME_TIMESTAMP", strconv.FormatInt(ts.UnixMicro(), 10)); err != nil {
		return fmt.Errorf("fake: error writing entry to systemd-journal-remote: %w", err)
	}
	if err := export.WriteField(&buf, "MESSAGE", message); err != nil {
		return fmt.Errorf("fake: error writing entry to systemd-journal-remote: %w", err)
	}
	if err := export.WriteField(&buf, "PRIORITY", strconv.Itoa(int(priority))); err != nil {
		return fmt.Errorf("fake: error writing entry to systemd-journal-remote: %w", err)
	}
	for k, v := range vars {
		if !validVarName.MatchString(k) {
			return fmt.Errorf("fake: invalid journal-entry variable name %q", k)
		}
		if err := export.WriteField(&buf, k, v); err != nil {
			return fmt.Errorf("fake: error writing entry to systemd-journal-remote: %w", err)
		}
	}
	if err := export.WriteEndOfEntry(&buf); err != nil {
		return fmt.Errorf("fake: error writing entry to systemd-journal-remote: %w", err)
	}

	// Invoke the command separately for each entry to ensure that the entry
	// has been flushed to disk when Send returns.
	cmd := exec.Command(s.CmdName, "--output", s.OutputPath, "-")
	cmd.Stdin = &buf
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
