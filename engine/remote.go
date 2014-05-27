package engine

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/beam"
	"github.com/dotcloud/docker/pkg/beam/data"
	"io"
	"os"
	"strconv"
	"sync"
)

type Sender struct {
	beam.Sender
}

func NewSender(s beam.Sender) *Sender {
	return &Sender{s}
}

func (s *Sender) Install(eng *Engine) error {
	// FIXME: this doesn't exist yet.
	eng.RegisterCatchall(s.Handle)
	return nil
}

func (s *Sender) Handle(job *Job) Status {
	cmd := append([]string{job.Name}, job.Args...)
	env := data.Encode(job.Env().MultiMap())
	msg := data.Empty().Set("cmd", cmd...).Set("env", env)
	peer, err := beam.SendConn(s, msg.Bytes())
	if err != nil {
		return job.Errorf("beamsend: %v", err)
	}
	defer peer.Close()
	var tasks sync.WaitGroup
	defer tasks.Wait()
	r := beam.NewRouter(nil)
	r.NewRoute().KeyStartsWith("cmd", "log", "stdout").HasAttachment().Handler(func(p []byte, stdout *os.File) error {
		tasks.Add(1)
		go func() {
			io.Copy(job.Stdout, stdout)
			stdout.Close()
			tasks.Done()
		}()
		return nil
	})
	r.NewRoute().KeyStartsWith("cmd", "log", "stderr").HasAttachment().Handler(func(p []byte, stderr *os.File) error {
		tasks.Add(1)
		go func() {
			io.Copy(job.Stderr, stderr)
			stderr.Close()
			tasks.Done()
		}()
		return nil
	})
	r.NewRoute().KeyStartsWith("cmd", "log", "stdin").HasAttachment().Handler(func(p []byte, stdin *os.File) error {
		go func() {
			io.Copy(stdin, job.Stdin)
			stdin.Close()
		}()
		return nil
	})
	var status int
	r.NewRoute().KeyStartsWith("cmd", "status").Handler(func(p []byte, f *os.File) error {
		cmd := data.Message(p).Get("cmd")
		if len(cmd) != 2 {
			return fmt.Errorf("usage: %s <0-127>", cmd[0])
		}
		s, err := strconv.ParseUint(cmd[1], 10, 8)
		if err != nil {
			return fmt.Errorf("usage: %s <0-127>", cmd[0])
		}
		status = int(s)
		return nil

	})
	if _, err := beam.Copy(r, peer); err != nil {
		return job.Errorf("%v", err)
	}
	return Status(status)
}

type Receiver struct {
	*Engine
	peer beam.Receiver
}

func NewReceiver(peer beam.Receiver) *Receiver {
	return &Receiver{Engine: New(), peer: peer}
}

func (rcv *Receiver) Run() error {
	r := beam.NewRouter(nil)
	r.NewRoute().KeyExists("cmd").Handler(func(p []byte, f *os.File) error {
		// Use the attachment as a beam return channel
		peer, err := beam.FileConn(f)
		if err != nil {
			f.Close()
			return err
		}
		f.Close()
		defer peer.Close()
		msg := data.Message(p)
		cmd := msg.Get("cmd")
		job := rcv.Engine.Job(cmd[0], cmd[1:]...)
		// Decode env
		env, err := data.Decode(msg.GetOne("env"))
		if err != nil {
			return fmt.Errorf("error decoding 'env': %v", err)
		}
		job.Env().InitMultiMap(env)
		stdout, err := beam.SendRPipe(peer, data.Empty().Set("cmd", "log", "stdout").Bytes())
		if err != nil {
			return err
		}
		job.Stdout.Add(stdout)
		stderr, err := beam.SendRPipe(peer, data.Empty().Set("cmd", "log", "stderr").Bytes())
		if err != nil {
			return err
		}
		job.Stderr.Add(stderr)
		stdin, err := beam.SendWPipe(peer, data.Empty().Set("cmd", "log", "stdin").Bytes())
		if err != nil {
			return err
		}
		job.Stdin.Add(stdin)
		// ignore error because we pass the raw status
		job.Run()
		err = peer.Send(data.Empty().Set("cmd", "status", fmt.Sprintf("%d", job.status)).Bytes(), nil)
		if err != nil {
			return err
		}
		return nil
	})
	_, err := beam.Copy(r, rcv.peer)
	return err
}
