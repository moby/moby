package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
)

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func proc1env() []string {
	raw, err := ioutil.ReadFile("/proc/1/environ")
	if err != nil {
		panic(err)
	}

	return strings.Split(string(raw), "\000")
}

func main() {
	ssh.Handle(func(s ssh.Session) {
		cmd := exec.Command("bash")
		ptyReq, winCh, isPty := s.Pty()
		if isPty {
			cmd.Env = append(cmd.Env,
				append(proc1env(),
					fmt.Sprintf("TERM=%s", ptyReq.Term),
					"DOCKER_MOBY_DISABLE_HACK=1")...)
			f, err := pty.Start(cmd)
			if err != nil {
				panic(err)
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				io.Copy(f, s) // stdin
			}()
			io.Copy(s, f) // stdout
		} else {
			io.WriteString(s, "No PTY requested.\n")
			s.Exit(1)
		}
	})

	log.Println("starting ssh server on port 2824...")
	log.Fatal(ssh.ListenAndServe(":2824", nil))
}
