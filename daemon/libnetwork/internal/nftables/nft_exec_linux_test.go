//go:build !cgo || static_build || !libnftables

package nftables

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

const nftStderrHelperEnv = "GO_WANT_NFT_STDERR_HELPER"

func init() {
	if os.Getenv(nftStderrHelperEnv) != "1" {
		return
	}
	// This must be larger than a pipe buffer so that reading stdout before
	// stderr would deadlock with this write.
	_, _ = os.Stderr.Write(bytes.Repeat([]byte("x"), 4<<20))
	os.Exit(1)
}

func TestApplyDrainsStderrConcurrently(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	originalLookPathNft := lookPathNft
	lookPathNft = sync.OnceValues(func() (string, error) {
		return executable, nil
	})
	t.Cleanup(func() {
		lookPathNft = originalLookPathNft
	})
	t.Setenv(nftStderrHelperEnv, "1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- (&nftCtx{}).Apply(ctx, []byte("add table inet test"))
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Apply succeeded with a failing nft command")
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("Apply deadlocked while nft was writing stderr")
	}
}
