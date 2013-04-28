package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"time"
)

var DOCKER_PATH string = path.Join(os.Getenv("DOCKERPATH"), "docker")

func runDaemon() (*exec.Cmd, error) {
	os.Remove("/var/run/docker.pid")
	cmd := exec.Command(DOCKER_PATH, "-d")
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		io.Copy(os.Stdout, outPipe)
	}()
	go func() {
		io.Copy(os.Stderr, errPipe)
	}()
	return cmd, nil
}

func crashTest() error {
	if err := exec.Command("/bin/bash", "-c", "while true; do true; done").Start(); err != nil {
		return err
	}

	var endpoint string
	if ep := os.Getenv("TEST_ENDPOINT"); ep == "" {
		endpoint = "192.168.56.1:7979"
	} else {
		endpoint = ep
	}
	conn, _ := net.Dial("tcp", endpoint)

	restartCount := 0
	totalTestCount := 1
	for {
		daemon, err := runDaemon()
		if err != nil {
			return err
		}
		if conn != nil {
			fmt.Fprintf(conn, "RESTART: %d\n", restartCount)
		}
		restartCount++
		//		time.Sleep(5000 * time.Millisecond)
		var stop bool
		go func() error {
			stop = false
			for i := 0; i < 100 && !stop; i++ {
				func() error {
					if conn != nil {
						fmt.Fprintf(conn, "TEST: %d\n", totalTestCount)
					}
					totalTestCount++
					cmd := exec.Command(DOCKER_PATH, "run", "base", "echo", "hello", "world")
					log.Printf("%d", i)
					outPipe, err := cmd.StdoutPipe()
					if err != nil {
						return err
					}
					inPipe, err := cmd.StdinPipe()
					if err != nil {
						return err
					}
					if err := cmd.Start(); err != nil {
						return err
					}
					go func() {
						io.Copy(os.Stdout, outPipe)
					}()
					// Expecting error, do not check
					inPipe.Write([]byte("hello world!!!!!\n"))
					go inPipe.Write([]byte("hello world!!!!!\n"))
					go inPipe.Write([]byte("hello world!!!!!\n"))
					inPipe.Close()

					if err := cmd.Wait(); err != nil {
						return err
					}
					outPipe.Close()
					return nil
				}()
			}
			return nil
		}()
		time.Sleep(20 * time.Second)
		stop = true
		if err := daemon.Process.Kill(); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := crashTest(); err != nil {
		log.Println(err)
	}
}
