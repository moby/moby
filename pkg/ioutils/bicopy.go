package ioutils

import (
	"io"
	"sync"
)

// Direction denotes the error direction for BidirectionalCopy.
type Direction int

const (
	// RemoteToLocal is remote to local
	RemoteToLocal Direction = iota
	// LocalToRemote is local to remote
	LocalToRemote
)

// BidirectionalCopy is similar to io.Copy but bidirectional.
// TODO: improve error handling interface.
// Maybe we should just return err instead of having errHandler.
func BidirectionalCopy(local io.ReadWriteCloser, remote io.ReadWriteCloser,
	errHandler func(error, Direction)) {
	var remoteClosed, localClosed bool
	var m sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer func() {
			m.Lock()
			remoteClosed = true
			if !localClosed {
				local.Close()
				localClosed = true
			}
			m.Unlock()
		}()
		_, err := io.Copy(local, remote)
		if err != nil && !localClosed {
			errHandler(err, RemoteToLocal)
		}
	}()
	go func() {
		defer wg.Done()
		defer func() {
			m.Lock()
			if !remoteClosed {
				remote.Close()
				remoteClosed = true
			}
			localClosed = true
			m.Unlock()
		}()
		_, err := io.Copy(remote, local)
		if err != nil && !remoteClosed {
			errHandler(err, LocalToRemote)
		}
	}()
	wg.Wait()
}
