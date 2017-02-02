package fsutil

import (
	"io"
	"os"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

func Receive(ctx context.Context, conn Stream, dest string, notifyHashed ChangeFunc) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &receiver{
		conn:         &syncStream{Stream: conn},
		dest:         dest,
		files:        make(map[string]uint32),
		pipes:        make(map[uint32]*io.PipeWriter),
		walkChan:     make(chan *currentPath, 128),
		notifyHashed: notifyHashed,
	}
	return r.run(ctx)
}

type receiver struct {
	dest         string
	conn         Stream
	files        map[string]uint32
	pipes        map[uint32]*io.PipeWriter
	mu           sync.RWMutex
	muPipes      sync.RWMutex
	walkChan     chan *currentPath
	notifyHashed ChangeFunc
}

func (r *receiver) readStat(ctx context.Context, pathC chan<- *currentPath) error {
	for {
		select {
		case p, ok := <-r.walkChan:
			if !ok {
				return nil
			}
			pathC <- p
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (r *receiver) run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	dw := DiskWriter{
		asyncDataFunc: r.asyncDataFunc,
		dest:          r.dest,
		notifyHashed:  r.notifyHashed,
	}

	walkDone := make(chan struct{})

	g.Go(func() error {
		err := doubleWalkDiff(ctx, dw.HandleChange, GetWalkerFn(r.dest), r.readStat)
		close(walkDone)
		return err
	})

	g.Go(func() error {
		var i uint32 = 0

		var p Packet
		for {
			p = Packet{Data: p.Data[:0]}
			if err := r.conn.RecvMsg(&p); err != nil {
				return err
			}
			switch p.Type {
			case PACKET_STAT:
				if p.Stat == nil {
					close(r.walkChan)
					<-walkDone
					go func() {
						dw.Wait()
						r.conn.SendMsg(&Packet{Type: PACKET_FIN})
					}()
					break
				}
				if os.FileMode(p.Stat.Mode)&(os.ModeDir|os.ModeSymlink|os.ModeNamedPipe|os.ModeDevice) == 0 {
					r.mu.Lock()
					r.files[p.Stat.Path] = i
					r.mu.Unlock()
				}
				i++
				r.walkChan <- &currentPath{path: p.Stat.Path, f: &StatInfo{p.Stat}}
			case PACKET_DATA:
				r.muPipes.Lock()
				pw, ok := r.pipes[p.ID]
				if !ok {
					r.muPipes.Unlock()
					return errors.Errorf("invalid file request %s", p.ID)
				}
				r.muPipes.Unlock()
				if len(p.Data) == 0 {
					if err := pw.Close(); err != nil {
						return err
					}
				} else {
					if _, err := pw.Write(p.Data); err != nil {
						return err
					}
				}
			case PACKET_FIN:
				return nil
			}
		}
	})
	return g.Wait()
}

func (r *receiver) asyncDataFunc(ctx context.Context, p string, wc io.WriteCloser) error {
	r.mu.Lock()
	id, ok := r.files[p]
	if !ok {
		r.mu.Unlock()
		return errors.Errorf("invalid file request %s", p)
	}
	delete(r.files, p)
	r.mu.Unlock()

	pr, pw := io.Pipe()
	r.muPipes.Lock()
	r.pipes[id] = pw
	r.muPipes.Unlock()
	if err := r.conn.SendMsg(&Packet{Type: PACKET_REQ, ID: id}); err != nil {
		return err
	}

	buf := bufPool.Get().([]byte)
	defer bufPool.Put(buf)
	if _, err := io.CopyBuffer(wc, pr, buf); err != nil {
		return err
	}
	return wc.Close()
}
