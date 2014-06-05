package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/testutils"
	"io"
	"strings"
	"testing"
	"time"
)

func TestHelloWorld(t *testing.T) {
	for i := 0; i < 10; i++ {
		testRemote(t,

			// Sender side
			func(eng *Engine) {
				job := eng.Job("echo", "hello", "world")
				out := &bytes.Buffer{}
				job.Stdout.Add(out)
				job.Run()
				if job.status != StatusOK {
					t.Fatalf("#%v", job.StatusCode())
				}
				lines := bufio.NewScanner(out)
				var i int
				for lines.Scan() {
					if lines.Text() != "hello world" {
						t.Fatalf("%#v", lines.Text())
					}
					i++
				}
				if i != 1000 {
					t.Fatalf("%#v", i)
				}
			},

			// Receiver side
			func(eng *Engine) {
				eng.Register("echo", func(job *Job) Status {
					// Simulate more output with a delay in the middle
					for i := 0; i < 500; i++ {
						fmt.Fprintf(job.Stdout, "%s\n", strings.Join(job.Args, " "))
					}
					time.Sleep(5 * time.Millisecond)
					for i := 0; i < 500; i++ {
						fmt.Fprintf(job.Stdout, "%s\n", strings.Join(job.Args, " "))
					}
					return StatusOK
				})
			},
		)
	}
}

func TestStdin(t *testing.T) {
	testRemote(t,

		func(eng *Engine) {
			job := eng.Job("mirror")
			job.Stdin.Add(strings.NewReader("hello world!\n"))
			out := &bytes.Buffer{}
			job.Stdout.Add(out)
			if err := job.Run(); err != nil {
				t.Fatal(err)
			}
			if out.String() != "hello world!\n" {
				t.Fatalf("%#v", out.String())
			}
		},

		func(eng *Engine) {
			eng.Register("mirror", func(job *Job) Status {
				if _, err := io.Copy(job.Stdout, job.Stdin); err != nil {
					t.Fatal(err)
				}
				return StatusOK
			})
		},
	)
}

func TestEnv(t *testing.T) {
	var (
		foo          string
		answer       int
		shadok_words []string
	)
	testRemote(t,

		func(eng *Engine) {
			job := eng.Job("sendenv")
			job.Env().Set("foo", "bar")
			job.Env().SetInt("answer", 42)
			job.Env().SetList("shadok_words", []string{"ga", "bu", "zo", "meu"})
			if err := job.Run(); err != nil {
				t.Fatal(err)
			}
		},

		func(eng *Engine) {
			eng.Register("sendenv", func(job *Job) Status {
				foo = job.Env().Get("foo")
				answer = job.Env().GetInt("answer")
				shadok_words = job.Env().GetList("shadok_words")
				return StatusOK
			})
		},
	)
	// Check for results here rather than inside the job handler,
	// otherwise the tests may incorrectly pass if the handler is not
	// called.
	if foo != "bar" {
		t.Fatalf("%#v", foo)
	}
	if answer != 42 {
		t.Fatalf("%#v", answer)
	}
	if strings.Join(shadok_words, ", ") != "ga, bu, zo, meu" {
		t.Fatalf("%#v", shadok_words)
	}
}

// Helpers

func testRemote(t *testing.T, senderSide, receiverSide func(*Engine)) {
	sndConn, rcvConn, err := beam.USocketPair()
	if err != nil {
		t.Fatal(err)
	}
	defer sndConn.Close()
	defer rcvConn.Close()
	sender := NewSender(sndConn)
	receiver := NewReceiver(rcvConn)

	// Setup the sender side
	eng := New()
	sender.Install(eng)

	// Setup the receiver side
	receiverSide(receiver.Engine)
	go receiver.Run()

	testutils.Timeout(t, func() {
		senderSide(eng)
	})
}
