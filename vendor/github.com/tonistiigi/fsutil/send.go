package fsutil

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1<<10)
	},
}

type Stream interface {
	RecvMsg(interface{}) error
	SendMsg(m interface{}) error
}

func Send(ctx context.Context, conn Stream, root string, opt *WalkOpt, progressCb func(int, bool)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &sender{
		ctx:        ctx,
		cancel:     cancel,
		conn:       &syncStream{Stream: conn},
		root:       root,
		opt:        opt,
		files:      make(map[uint32]string),
		progressCb: progressCb,
	}
	return s.run()
}

type sender struct {
	ctx             context.Context
	conn            Stream
	cancel          func()
	opt             *WalkOpt
	root            string
	files           map[uint32]string
	mu              sync.RWMutex
	progressCb      func(int, bool)
	progressCurrent int
}

func (s *sender) run() error {
	go s.send()
	defer s.updateProgress(0, true)
	for {
		var p Packet
		if err := s.conn.RecvMsg(&p); err != nil {
			return err
		}
		switch p.Type {
		case PACKET_REQ:
			if err := s.queue(p.ID); err != nil {
				return err
			}
		case PACKET_FIN:
			return s.conn.SendMsg(&Packet{Type: PACKET_FIN})
		}
	}
}

func (s *sender) updateProgress(size int, last bool) {
	if s.progressCb != nil {
		s.progressCurrent += size
		s.progressCb(s.progressCurrent, last)
	}
}

func (s *sender) queue(id uint32) error {
	// TODO: add worker threads
	// TODO: use something faster than map
	s.mu.Lock()
	p, ok := s.files[id]
	if !ok {
		s.mu.Unlock()
		return errors.Errorf("invalid file id %d", id)
	}
	delete(s.files, id)
	s.mu.Unlock()
	go s.sendFile(id, p)
	return nil
}

func (s *sender) sendFile(id uint32, p string) error {
	f, err := os.Open(filepath.Join(s.root, p))
	if err == nil {
		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)
		if _, err := io.CopyBuffer(&fileSender{sender: s, id: id}, f, buf); err != nil {
			return err // TODO: handle error
		}
	}
	return s.conn.SendMsg(&Packet{ID: id, Type: PACKET_DATA})
}

func (s *sender) send() error {
	var i uint32 = 0
	err := Walk(s.ctx, s.root, s.opt, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		stat, ok := fi.Sys().(*Stat)
		if !ok {
			return errors.Wrapf(err, "invalid fileinfo without stat info: %s", path)
		}
		if runtime.GOOS == "windows" {
			stat.Mode &= (0755 | uint32(os.ModeDir))
			// Add the x bit: make everything +x from windows
			stat.Mode |= 0111
		}
		p := &Packet{
			Type: PACKET_STAT,
			Stat: stat,
		}
		s.mu.Lock()
		s.files[i] = stat.Path
		i++
		s.mu.Unlock()
		s.updateProgress(p.Size(), false)
		return errors.Wrapf(s.conn.SendMsg(p), "failed to send stat %s", path)
	})
	if err != nil {
		return err
	}
	return errors.Wrapf(s.conn.SendMsg(&Packet{Type: PACKET_STAT}), "failed to send last stat")
}

type fileSender struct {
	sender *sender
	id     uint32
}

func (fs *fileSender) Write(dt []byte) (int, error) {
	if len(dt) == 0 {
		return 0, nil
	}
	p := &Packet{Type: PACKET_DATA, ID: fs.id, Data: dt}
	if err := fs.sender.conn.SendMsg(p); err != nil {
		return 0, err
	}
	fs.sender.updateProgress(p.Size(), false)
	return len(dt), nil
}

type syncStream struct {
	Stream
	mu sync.Mutex
}

func (ss *syncStream) SendMsg(m interface{}) error {
	ss.mu.Lock()
	err := ss.Stream.SendMsg(m)
	ss.mu.Unlock()
	return err
}
