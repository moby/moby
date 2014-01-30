package main

import (
	"bufio"
	"fmt"
	"io"
	"github.com/dotcloud/docker/pkg/beam"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	defer wg.Wait()
	runCommand := beam.Func(RunCommand)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		wg.Add(1)
		go func(cmd []byte) {
			defer wg.Done()
			local, remote := beam.Pipe()
			defer local.Close()
			// Send the command
			if err := runCommand.Send(cmd, remote); err != nil {
				remote.Close()
				die("runcommand: %s", err)
			}
			// Communicate with the command
			for {
				data, st, err := local.Receive()
				if err != nil {
					return
				}
				if st != nil {
					wg.Add(1)
					go func(data []byte, st beam.Stream) {
						defer wg.Done()
						if st == nil {
							return
						}
						defer st.Close()
						if name := string(data); name == "stdout" {
							io.Copy(os.Stdout, beam.NewReader(st))
						} else if name == "stderr" {
							io.Copy(os.Stdout, beam.NewReader(st))
						}
					}(data, st)
				}
			}
		}(input.Bytes())
	}
}

func RunCommand(header []byte, stream beam.Stream) (err error) {
	parts := strings.Split(string(header), " ")
	// Setup stdout
	stdoutRemote, stdout := io.Pipe()
	defer stdout.Close()
	if err := stream.Send([]byte("stdout"), beam.WrapIO(stdoutRemote, 0)); err != nil {
		return err
	}
	// Setup stderr
	stderrRemote, stderr := io.Pipe()
	defer stderr.Close()
	if err := stream.Send([]byte("stderr"), beam.WrapIO(stderrRemote, 0)); err != nil {
		return err
	}
	if parts[0] == "echo" {
		fmt.Fprintf(stdout, "%s\n", strings.Join(parts[1:], " "))
	} else if parts[0] == "die" {
		os.Exit(1)
	} else if parts[0] == "sleep" {
		time.Sleep(1 * time.Second)
	} else {
		fmt.Fprintf(stderr, "%s: command not found\n", parts[0])
	}
	return nil
}

func die(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, f, args...)
	os.Exit(1)
}
