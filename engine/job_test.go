package engine

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func TestJobOK(t *testing.T) {
	eng := New()
	eng.Register("return_ok", func(job *Job) error { return nil })
	err := eng.Job("return_ok").Run()
	if err != nil {
		t.Fatalf("Expected: err=%v\nReceived: err=%v", nil, err)
	}
}

func TestJobErr(t *testing.T) {
	eng := New()
	eng.Register("return_err", func(job *Job) error { return errors.New("return_err") })
	err := eng.Job("return_err").Run()
	if err == nil {
		t.Fatalf("When a job returns error, Run() should return an error")
	}
}

func TestJobStdoutString(t *testing.T) {
	eng := New()
	// FIXME: test multiple combinations of output and status
	eng.Register("say_something_in_stdout", func(job *Job) error {
		job.Printf("Hello world\n")
		return nil
	})

	job := eng.Job("say_something_in_stdout")
	var outputBuffer = bytes.NewBuffer(nil)
	job.Stdout.Add(outputBuffer)
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}
	fmt.Println(outputBuffer)
	var output = Tail(outputBuffer, 1)
	if expectedOutput := "Hello world"; output != expectedOutput {
		t.Fatalf("Stdout last line:\nExpected: %v\nReceived: %v", expectedOutput, output)
	}
}
