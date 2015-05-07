package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	CmdError = "cmd.error"
	CmdPing  = "cmd.ping"
	CmdStop  = "cmd.stop"

	// From main to plugin
	CmdGetMetadata = "GetMetadata"
)

// This first section defines things used by the 'plugin' side of the
// plugin infrastructure
// ==================================================================

// Passed to the plugin so it can make calls back to the 'main' exe
type Main struct {
	Connection

	start   func(c *Main)
	request PluginRequestFunc
	inFD    int
	outFD   int
}

var ourMain Main

type PluginRequestFunc func(c *Main, cmd string, buf []byte) ([]byte, error)

// Register connects the plugin back to the main
func Register(start func(c *Main), request PluginRequestFunc) error {
	ourMain.start = start
	ourMain.request = request

	if len(os.Args) > 2 {
		var e1, e2 error
		ourMain.inFD, e1 = strconv.Atoi(os.Args[1])
		ourMain.outFD, e2 = strconv.Atoi(os.Args[2])
		if e1 != nil || e2 != nil {
			return fmt.Errorf("Error setting up connectsion: %q %q", e1, e2)
		}
	} else {
		// Just use stdin/out
		ourMain.inFD = 0
		ourMain.inFD = 1
	}

	ourMain.Setup(ourMain.inFD, ourMain.outFD, processChunkFromMain)

	if ourMain.start != nil {
		ourMain.start(&ourMain)
	}

	go func() {
		for {
			if _, err := ourMain.Call(CmdPing, ""); err != nil {
				os.Exit(1)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// If 'Request' is defined then sleep forever. Otherwise
	// the executable will exit too soon
	if ourMain.request != nil {
		select {}
	}

	return nil
}

func processChunkFromMain(c *Connection, chunk *Chunk) *Chunk {
	var buf []byte
	var err error
	var resCmd = chunk.cmd

	switch chunk.cmd {
	case CmdPing:
		// no-op

	case CmdStop:
		c.Close()
		os.Exit(0)

	default:
		// Must be user-defined so pass along to the plugin to deal with
		buf, err = ourMain.request(&ourMain, chunk.cmd, chunk.buffer)
	}

	if chunk.direction == Oneway {
		return nil
	}

	if err != nil {
		resCmd = CmdError
		buf = []byte(err.Error())
	}

	return &Chunk{
		threadID:  chunk.threadID,
		direction: Response,
		cmd:       resCmd,
		buffer:    buf,
	}
}

// This section defines things used by the 'main' side of the
// plugin infrastructure
// ==================================================================

// Used by the 'main' exe to call into the plugin
type Plugin struct {
	Connection
	Config map[string]string

	exePath string
	fn      MainRequestFunc
	name    string
	cmd     *exec.Cmd

	// Saved just so we don't gc the Files
	r1, w1, r2, w2 *os.File
}

type MainRequestFunc func(p *Plugin, cmd string, buf []byte) ([]byte, error)

func NewPlugin(exe string, fn MainRequestFunc) *Plugin {
	return &Plugin{
		exePath: exe,
		fn:      fn,
	}
}

func (p *Plugin) Start() error {
	var e1, e2 error

	p.r1, p.w1, e1 = os.Pipe()
	p.r2, p.w2, e2 = os.Pipe()

	if e1 != nil || e2 != nil {
		return fmt.Errorf("Error setting up pipes: %q\n%q", e1, e2)
	}

	p.cmd = exec.Command(p.exePath, "3", "4")
	p.cmd.Stdout = os.Stdout
	p.cmd.ExtraFiles = []*os.File{p.r1, p.w2}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("Error starting plugin '%s': %q\n", p.exePath, err)
	}

	p.r1.Close()
	p.w2.Close()

	p.Setup(int(p.r2.Fd()), int(p.w1.Fd()), p.processChunkFromPlugin)

	buf, err := p.CallBytes(CmdGetMetadata, nil)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(buf, &p.Config); err != nil {
		return err
	}

	return err
}

func (p *Plugin) Stop() {
	p.NotifyBytes(CmdStop, nil)
	p.Close()
	p.cmd.Process.Kill()
}

func (p *Plugin) processChunkFromPlugin(c *Connection, inChunk *Chunk) *Chunk {
	var resCmd string
	var buf []byte
	var err error

	switch inChunk.cmd {
	case CmdPing:
		// no-op

	default:
		// Must be user-defined so pass along to the plugin to deal with
		buf, err = p.fn(p, inChunk.cmd, inChunk.buffer)
	}

	if err != nil {
		resCmd = CmdError
		buf = []byte(err.Error())
	}

	if inChunk.direction == Oneway {
		return nil
	}

	return &Chunk{
		threadID:  inChunk.threadID,
		direction: Response,
		cmd:       resCmd,
		buffer:    buf,
	}
}
