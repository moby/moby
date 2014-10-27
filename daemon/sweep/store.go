package sweep

import "sync"

type sweepStore struct {
	sync.Mutex
	Containers map[string]bool
}

var sweepStoreInstance *sweepStore

func init() {
	sweepStoreInstance = &sweepStore{
		Containers: make(map[string]bool),
	}
}

func AddToSweep(hash string) {
	sweepStoreInstance.Lock()
	defer sweepStoreInstance.Unlock()
	sweepStoreInstance.Containers[hash] = true
}

func InSweepStore(hash string) bool {
	sweepStoreInstance.Lock()
	defer sweepStoreInstance.Unlock()
	_, exist := sweepStoreInstance.Containers[hash]
	return exist
}

func DeleteFromSweepStore(hash string) {
	sweepStoreInstance.Lock()
	defer sweepStoreInstance.Unlock()
	delete(sweepStoreInstance.Containers, hash)
}
