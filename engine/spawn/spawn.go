package spawn

import (
	"fmt"
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/utils"
	"os"
	"os/exec"
)

var initCalled bool

// Init checks if the current process has been created by Spawn.
//
// If no, it returns nil and the original program can continue
// unmodified.
//
// If no, it hijacks the process to run as a child worker controlled
// by its parent over a beam connection, with f exposed as a remote
// service. In this case Init never returns.
//
// The hijacking process takes place as follows:
//	- Open file descriptor 3 as a beam endpoint. If this fails,
//	terminate the current process.
//	- Start a new engine.
//	- Call f.Install on the engine. Any handlers registered
//	will be available for remote invocation by the parent.
//	- Listen for beam messages from the parent and pass them to
//	the handlers.
//	- When the beam endpoint is closed by the parent, terminate
//	the current process.
//
// NOTE: Init must be called at the beginning of the same program
// calling Spawn. This is because Spawn approximates a "fork" by
// re-executing the current binary - where it expects spawn.Init
// to intercept the control flow and execute the worker code.
func Init(f engine.Installer) error {
	initCalled = true
	if os.Getenv("ENGINESPAWN") != "1" {
		return nil
	}
	fmt.Printf("[%d child]\n", os.Getpid())
	// Hijack the process
	childErr := func() error {
		fd3 := os.NewFile(3, "beam-introspect")
		introsp, err := beam.FileConn(fd3)
		if err != nil {
			return fmt.Errorf("beam introspection error: %v", err)
		}
		fd3.Close()
		defer introsp.Close()
		eng := engine.NewReceiver(introsp)
		if err := f.Install(eng.Engine); err != nil {
			return err
		}
		if err := eng.Run(); err != nil {
			return err
		}
		return nil
	}()
	if childErr != nil {
		os.Exit(1)
	}
	os.Exit(0)
	return nil // Never reached
}

// Spawn starts a new Engine in a child process and returns
// a proxy Engine through which it can be controlled.
//
// The commands available on the child engine are determined
// by an earlier call to Init. It is important that Init be
// called at the very beginning of the current program - this
// allows it to be called as a re-execution hook in the child
// process.
//
// Long story short, if you want to expose `myservice` in a child
// process, do this:
//
// func main() {
//     spawn.Init(myservice)
//     [..]
//     child, err := spawn.Spawn()
//     [..]
//     child.Job("dosomething").Run()
// }
func Spawn() (*engine.Engine, error) {
	if !initCalled {
		return nil, fmt.Errorf("spawn.Init must be called at the top of the main() function")
	}
	cmd := exec.Command(utils.SelfPath())
	cmd.Env = append(cmd.Env, "ENGINESPAWN=1")
	local, remote, err := beam.SocketPair()
	if err != nil {
		return nil, err
	}
	child, err := beam.FileConn(local)
	if err != nil {
		local.Close()
		remote.Close()
		return nil, err
	}
	local.Close()
	cmd.ExtraFiles = append(cmd.ExtraFiles, remote)
	// FIXME: the beam/engine glue has no way to inform the caller
	// of the child's termination. The next call will simply return
	// an error.
	if err := cmd.Start(); err != nil {
		child.Close()
		return nil, err
	}
	eng := engine.New()
	if err := engine.NewSender(child).Install(eng); err != nil {
		child.Close()
		return nil, err
	}
	return eng, nil
}
