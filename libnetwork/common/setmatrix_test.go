package common

import (
	"context"
	"strconv"
	"testing"
	"time"

	_ "github.com/docker/libnetwork/testutils"
)

func TestSetSerialInsertDelete(t *testing.T) {
	s := NewSetMatrix()

	b, i := s.Insert("a", "1")
	if !b || i != 1 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "1")
	if b || i != 1 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "2")
	if !b || i != 2 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "1")
	if b || i != 2 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "3")
	if !b || i != 3 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "2")
	if b || i != 3 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "3")
	if b || i != 3 {
		t.Fatalf("error in insert %t %d", b, i)
	}
	b, i = s.Insert("a", "4")
	if !b || i != 4 {
		t.Fatalf("error in insert %t %d", b, i)
	}

	b, p := s.Contains("a", "1")
	if !b || !p {
		t.Fatalf("error in contains %t %t", b, p)
	}
	b, p = s.Contains("a", "2")
	if !b || !p {
		t.Fatalf("error in contains %t %t", b, p)
	}
	b, p = s.Contains("a", "3")
	if !b || !p {
		t.Fatalf("error in contains %t %t", b, p)
	}
	b, p = s.Contains("a", "4")
	if !b || !p {
		t.Fatalf("error in contains %t %t", b, p)
	}

	i, b = s.Cardinality("a")
	if !b || i != 4 {
		t.Fatalf("error in cardinality count %t %d", b, i)
	}

	b, i = s.Remove("a", "1")
	if !b || i != 3 {
		t.Fatalf("error in remove %t %d", b, i)
	}
	b, i = s.Remove("a", "3")
	if !b || i != 2 {
		t.Fatalf("error in remove %t %d", b, i)
	}
	b, i = s.Remove("a", "1")
	if b || i != 2 {
		t.Fatalf("error in remove %t %d", b, i)
	}
	b, i = s.Remove("a", "4")
	if !b || i != 1 {
		t.Fatalf("error in remove %t %d", b, i)
	}
	b, i = s.Remove("a", "2")
	if !b || i != 0 {
		t.Fatalf("error in remove %t %d", b, i)
	}
	b, i = s.Remove("a", "2")
	if b || i != 0 {
		t.Fatalf("error in remove %t %d", b, i)
	}

	i, b = s.Cardinality("a")
	if b || i != 0 {
		t.Fatalf("error in cardinality count %t %d", b, i)
	}
}

func insertDeleteRotuine(ctx context.Context, endCh chan int, s SetMatrix, key, value string) {
	for {
		select {
		case <-ctx.Done():
			endCh <- 0
			return
		default:
			b, _ := s.Insert(key, value)
			if !b {
				endCh <- 1
				return
			}

			b, _ = s.Remove(key, value)
			if !b {
				endCh <- 2
				return
			}
		}
	}
}

func TestSetParallelInsertDelete(t *testing.T) {
	s := NewSetMatrix()
	parallelRoutines := 6
	endCh := make(chan int)
	// Let the routines running and competing for 10s
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < parallelRoutines; i++ {
		go insertDeleteRotuine(ctx, endCh, s, "key-"+strconv.Itoa(i%3), strconv.Itoa(i))
	}
	for parallelRoutines > 0 {
		v := <-endCh
		if v == 1 {
			t.Fatalf("error one goroutine failed on the insert")
		}
		if v == 2 {
			t.Fatalf("error one goroutine failed on the remove")
		}
		parallelRoutines--
	}
	if i, b := s.Cardinality("key"); b || i > 0 {
		t.Fatalf("error the set should be empty %t %d", b, i)
	}
}
