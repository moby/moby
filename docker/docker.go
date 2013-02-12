package main

import (
	"github.com/dotcloud/docker/rcli"
	"github.com/dotcloud/docker/future"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
	"path"
	"path/filepath"
	"flag"
)


type Termios struct {
	Iflag  uintptr
	Oflag  uintptr
	Cflag  uintptr
	Lflag  uintptr
	Cc     [20]byte
	Ispeed uintptr
	Ospeed uintptr
}


const (
	// Input flags
	inpck  = 0x010
	istrip = 0x020
	icrnl  = 0x100
	ixon   = 0x200

	// Output flags
	opost = 0x1

	// Control flags
	cs8 = 0x300

	// Local flags
	icanon = 0x100
	iexten = 0x400
)

const (
        HUPCL                             = 0x4000 
        ICANON                            = 0x100 
        ICRNL                             = 0x100 
        IEXTEN                            = 0x400
        BRKINT                            = 0x2 
        CFLUSH                            = 0xf 
        CLOCAL                            = 0x8000 
        CREAD                             = 0x800 
        CS5                               = 0x0 
        CS6                               = 0x100 
        CS7                               = 0x200 
        CS8                               = 0x300 
        CSIZE                             = 0x300 
        CSTART                            = 0x11 
        CSTATUS                           = 0x14 
        CSTOP                             = 0x13 
        CSTOPB                            = 0x400 
        CSUSP                             = 0x1a 
        IGNBRK                            = 0x1 
        IGNCR                             = 0x80 
        IGNPAR                            = 0x4 
        IMAXBEL                           = 0x2000 
        INLCR                             = 0x40 
        INPCK                             = 0x10 
        ISIG                              = 0x80 
        ISTRIP                            = 0x20 
        IUTF8                             = 0x4000 
        IXANY                             = 0x800 
        IXOFF                             = 0x400 
        IXON                              = 0x200 
        NOFLSH                            = 0x80000000 
        OCRNL                             = 0x10 
        OFDEL                             = 0x20000 
        OFILL                             = 0x80 
        ONLCR                             = 0x2 
        ONLRET                            = 0x40 
        ONOCR                             = 0x20 
        ONOEOT                            = 0x8 
        OPOST                             = 0x1 
RENB                            = 0x1000 
        PARMRK                            = 0x8 
        PARODD                            = 0x2000 

        TOSTOP                            = 0x400000 
        VDISCARD                          = 0xf 
        VDSUSP                            = 0xb 
        VEOF                              = 0x0 
        VEOL                              = 0x1 
        VEOL2                             = 0x2 
        VERASE                            = 0x3 
        VINTR                             = 0x8 
        VKILL                             = 0x5 
        VLNEXT                            = 0xe 
        VMIN                              = 0x10 
        VQUIT                             = 0x9 
        VREPRINT                          = 0x6 
        VSTART                            = 0xc 
        VSTATUS                           = 0x12 
        VSTOP                             = 0xd 
        VSUSP                             = 0xa 
        VT0                               = 0x0 
        VT1                               = 0x10000 
        VTDLY                             = 0x10000 
        VTIME                             = 0x11 
	ECHO				  = 0x00000008

        PENDIN                            = 0x20000000 
)

type State struct {
       termios Termios
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
        var termios Termios
        _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
        return err == 0
}

// MakeRaw put the terminal connected to the given file descriptor into raw
// mode and returns the previous state of the terminal so that it can be
// restored.
func MakeRaw(fd int) (*State, error) {
        var oldState State
        if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(getTermios), uintptr(unsafe.Pointer(&oldState.termios)), 0, 0, 0); err != 0 {
                return nil, err
        }

        newState := oldState.termios
        newState.Iflag &^= istrip | INLCR | ICRNL | IGNCR | IXON | IXOFF
        newState.Lflag &^= ECHO | ICANON | ISIG
        if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&newState)), 0, 0, 0); err != 0 {
                return nil, err
        }

        return &oldState, nil
}


// Restore restores the terminal connected to the given file descriptor to a
// previous state.
func Restore(fd int, state *State) error {
        _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(setTermios), uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
        return err
}

var oldState *State

func Fatal(err error) {
	if oldState != nil {
		Restore(0, oldState)
	}
	log.Fatal(err)
}


func main() {
	if cmd := path.Base(os.Args[0]); cmd == "docker" {
		fl_shell := flag.Bool("i", false, "Interactive mode")
		flag.Parse()
		if *fl_shell {
			if err := InteractiveMode(); err != nil {
				log.Fatal(err)
			}
		} else {
			SimpleMode(os.Args[1:])
		}
	} else {
		SimpleMode(append([]string{cmd}, os.Args[1:]...))
	}
}

// Run docker in "simple mode": run a single command and return.
func SimpleMode(args []string) {
	var err error
	if IsTerminal(0) && os.Getenv("NORAW") == "" {
		oldState, err = MakeRaw(0)
		if err != nil {
			panic(err)
		}
		defer Restore(0, oldState)
	}
	// FIXME: we want to use unix sockets here, but net.UnixConn doesn't expose
	// CloseWrite(), which we need to cleanly signal that stdin is closed without
	// closing the connection.
	// See http://code.google.com/p/go/issues/detail?id=3345
	conn, err := rcli.Call("tcp", "127.0.0.1:4242", args...)
	if err != nil {
		Fatal(err)
	}
	receive_stdout := future.Go(func() error {
		_, err := io.Copy(os.Stdout, conn)
		return err
	})
	send_stdin := future.Go(func() error {
		_, err := io.Copy(conn, os.Stdin)
		if err := conn.CloseWrite(); err != nil {
			log.Printf("Couldn't send EOF: " + err.Error())
		}
		return err
	})
	if err := <-receive_stdout; err != nil {
		Fatal(err)
	}
	if oldState != nil {
		Restore(0, oldState)
	}
	if !IsTerminal(0) {
		if err := <-send_stdin; err != nil {
			Fatal(err)
		}
	}
}

// Run docker in "interactive mode": run a bash-compatible shell capable of running docker commands.
func InteractiveMode() error {
	// Determine path of current docker binary
	dockerPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return err
	}
	dockerPath, err = filepath.Abs(dockerPath)
	if err != nil {
		return err
	}

	// Create a temp directory
	tmp, err := ioutil.TempDir("", "docker-shell")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// For each command, create an alias in temp directory
	// FIXME: generate this list dynamically with introspection of some sort
	// It might make sense to merge docker and dockerd to keep that introspection
	// within a single binary.
	for _, cmd := range []string{
		"help",
		"run",
		"ps",
		"pull",
		"put",
		"rm",
		"kill",
		"wait",
		"stop",
		"logs",
		"diff",
		"commit",
		"attach",
		"info",
		"tar",
		"web",
		"docker",
	} {
		if err := os.Symlink(dockerPath, path.Join(tmp, cmd)); err != nil {
			return err
		}
	}

	// Run $SHELL with PATH set to temp directory
	rcfile, err := ioutil.TempFile("", "docker-shell-rc")
	if err != nil {
		return err
	}
	io.WriteString(rcfile, "enable -n help\n")
	os.Setenv("PATH", tmp)
	os.Setenv("PS1", "\\h docker> ")
	shell := exec.Command("/bin/bash", "--rcfile", rcfile.Name())
	shell.Stdin = os.Stdin
	shell.Stdout = os.Stdout
	shell.Stderr = os.Stderr
	if err := shell.Run(); err != nil {
		return err
	}
	return nil
}
