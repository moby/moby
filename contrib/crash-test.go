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

var DOCKERPATH = path.Join(os.Getenv("DOCKERPATH"), "docker")

// WARNING: this crashTest will 1) crash your host, 2) remove all containers
func runDaemon() (*exec.Cmd, error) {
	os.Remove("/var/run/docker.pid")
	exec.Command("rm", "-rf", "/var/lib/docker/containers").Run()
	cmd := exec.Command(DOCKERPATH, "-d")
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

	c := make(chan bool)
	var conn io.Writer

	go func() {
		conn, _ = net.Dial("tcp", endpoint)
		c <- false
	}()
	go func() {
		time.Sleep(2 * time.Second)
		c <- true
	}()
	<-c

	restartCount := 0
	totalTestCount := 1
	for {
		daemon, err := runDaemon()
		if err != nil {
			return err
		}
		restartCount++
		//		time.Sleep(5000 * time.Millisecond)
		var stop bool
		go func() error {
			stop = false
			for i := 0; i < 100 && !stop; {
				func() error {
					cmd := exec.Command(DOCKERPATH, "run", "ubuntu", "echo", fmt.Sprintf("%d", totalTestCount))
					i++
					totalTestCount++
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
					if conn != nil {
						go io.Copy(conn, outPipe)
					}

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
}

func main() {
	if err := crashTest(); err != nil {
		log.Println(err)
	}
}
