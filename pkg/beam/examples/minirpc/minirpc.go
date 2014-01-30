package main

import (
	"bufio"
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	runCommand := beam.Func(RunCommand)
	input := bufio.NewScanner(os.Stdin)
	var wg sync.WaitGroup
	defer wg.Wait()
	for input.Scan() {
		wg.Add(1)
		go func(cmd []byte) {
			if err := runCommand.Send(cmd, beam.DevNull); err != nil {
				die("runcommand: %s", err)
			}
			wg.Done()
		}(input.Bytes())
	}
}

func RunCommand(header []byte, stream beam.Stream) (err error) {
	fmt.Printf("RunCommand(%s)\n", header)
	defer fmt.Printf("RunCommand(%s) = %#v\n", header, err)
	parts := strings.Split(string(header), " ")
	// Setup stdout
	stdoutRemote, stdoutLocal := beam.Pipe()
	defer stdoutLocal.Close()
	fmt.Printf("Sending stdout\n")
	if err := stream.Send([]byte("stdout"), stdoutRemote); err != nil {
		return err
	}
	fmt.Printf("Sending stdout -> Done\n")
	stdout := beam.NewWriter(stdoutLocal)
	// Setup stderr
	stderrRemote, stderrLocal := beam.Pipe()
	defer stderrLocal.Close()
	fmt.Printf("Sending stderr\n")
	if err := stream.Send([]byte("stderr"), stderrRemote); err != nil {
		return err
	}
	fmt.Printf("Sending stderr: done\n")
	stderr := beam.NewWriter(stderrLocal)
	if parts[0] == "echo" {
		fmt.Printf("Writing to stdout\n")
		fmt.Fprintf(stdout, "%s\n", strings.Join(parts[1:], " "))
		fmt.Printf("Writing to stdout: done\n")
	} else if parts[0] == "die" {
		fmt.Fprintf(stderr, "%s\n", strings.Join(parts[1:], " "))
		os.Exit(1)
	} else if parts[0] == "sleep" {
		time.Sleep(5 * time.Second)
	} else {
		fmt.Fprintf(stderr, "%s: command not found", parts[0])
	}
	return nil
}

func die(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, f, args...)
	os.Exit(1)
}
