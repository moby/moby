package progress

import (
	"context"
	"io"
	"sync"
)

type MultiReader struct {
	mu          sync.Mutex
	main        Reader
	initialized bool
	done        chan struct{}
	writers     map[*progressWriter]func()
	sent        []*Progress
}

func NewMultiReader(pr Reader) *MultiReader {
	mr := &MultiReader{
		main:    pr,
		writers: make(map[*progressWriter]func()),
		done:    make(chan struct{}),
	}
	return mr
}

func (mr *MultiReader) Reader(ctx context.Context) Reader {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	pr, ctx, closeWriter := NewContext(ctx)
	pw, _, ctx := NewFromContext(ctx)

	w := pw.(*progressWriter)

	isBehind := len(mr.sent) > 0

	select {
	case <-mr.done:
		isBehind = true
	default:
		if !isBehind {
			mr.writers[w] = closeWriter
		}
	}

	go func() {
		if isBehind {
			close := func() {
				w.Close()
				closeWriter()
			}
			i := 0
			for {
				mr.mu.Lock()
				sent := mr.sent
				count := len(sent) - i
				if count == 0 {
					select {
					case <-ctx.Done():
						close()
						mr.mu.Unlock()
						return
					case <-mr.done:
						close()
						mr.mu.Unlock()
						return
					default:
					}
					mr.writers[w] = closeWriter
					mr.mu.Unlock()
					break
				}
				mr.mu.Unlock()
				for i, p := range sent[i:] {
					w.writeRawProgress(p)
					if i%100 == 0 {
						select {
						case <-ctx.Done():
							close()
							return
						default:
						}
					}
				}
				i += count
			}
		}

		select {
		case <-ctx.Done():
		case <-mr.done:
		}
		mr.mu.Lock()
		defer mr.mu.Unlock()
		delete(mr.writers, w)
	}()

	if !mr.initialized {
		go mr.handle()
		mr.initialized = true
	}

	return pr
}

func (mr *MultiReader) handle() error {
	for {
		p, err := mr.main.Read(context.TODO())
		if err != nil {
			if err == io.EOF {
				mr.mu.Lock()
				for w, c := range mr.writers {
					w.Close()
					c()
				}
				close(mr.done)
				mr.mu.Unlock()
				return nil
			}
			return err
		}
		mr.mu.Lock()
		for _, p := range p {
			for w := range mr.writers {
				w.writeRawProgress(p)
			}
		}
		mr.sent = append(mr.sent, p...)
		mr.mu.Unlock()
	}
}
