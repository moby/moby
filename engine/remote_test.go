package engine

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
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

	timeout(t, func() {
		senderSide(eng)
	})
}

func timeout(t *testing.T, f func()) {
	onTimeout := time.After(100 * time.Millisecond)
	onDone := make(chan bool)
	go func() {
		f()
		close(onDone)
	}()
	select {
	case <-onTimeout:
		t.Fatalf("timeout")
	case <-onDone:
	}
}
