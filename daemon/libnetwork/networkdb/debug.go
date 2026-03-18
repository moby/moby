package networkdb

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/containerd/log"
)

func logEncKeys(ctx context.Context, keys ...[]byte) {
	klpath := os.Getenv("NETWORKDBKEYLOGFILE")
	if klpath == "" {
		return
	}

	die := func(err error) {
		log.G(ctx).WithFields(log.Fields{
			"error": err,
			"path":  klpath,
		}).Error("could not write to NetworkDB encryption-key log")
	}
	f, err := os.OpenFile(klpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		die(err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			die(err)
		}
	}()

	tohex := hex.NewEncoder(f)
	for _, key := range keys {
		if _, err := tohex.Write(key); err != nil {
			die(err)
			return
		}
		if _, err := f.WriteString("\n"); err != nil {
			die(err)
			return
		}
	}
}

func (nDB *NetworkDB) DebugDumpTable(tname string) string {
	nDB.RLock()
	root := nDB.indexes[byTable].Root()
	nDB.RUnlock()
	var sb strings.Builder
	root.WalkPrefix([]byte("/"+tname), func(path []byte, v *entry) bool {
		fmt.Fprintf(&sb, "    %q: %+v\n", path, v)
		return false
	})
	return sb.String()
}
