package network

import (
	"sync"
	"testing"
	"time"
)

func TestNameLocker(t *testing.T) {
	wg := sync.WaitGroup{}
	nl := &nameLocker{}
	nl.init()
	names := []string{"testnetwork1", "testnetwork2", "testnetwork3"}
	counts := make(map[string]int)
	for _, name := range names {
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				c := nl.lock(name)
				defer nl.unlock(name, c)
				time.Sleep(100 * time.Millisecond)
				counts[name]++

			}(name)
		}
	}

	wg.Wait()
	if len(nl.muxMap) > 0 {
		t.Fatal("Leak in named lock map")
	}
	for _, val := range counts {
		if val != 50 {
			t.Fatal("wrong count of succesful locks")
		}
	}
}
