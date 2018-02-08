package jsonfilelog // import "github.com/docker/docker/daemon/logger/jsonfilelog"

import (
	"bytes"
	"testing"
	"time"

	"github.com/docker/docker/daemon/logger"
	"github.com/gotestyourself/gotestyourself/fs"
	"github.com/stretchr/testify/require"
)

func BenchmarkJSONFileLoggerReadLogs(b *testing.B) {
	tmp := fs.NewDir(b, "bench-jsonfilelog")
	defer tmp.Remove()

	jsonlogger, err := New(logger.Info{
		ContainerID: "a7317399f3f857173c6179d44823594f8294678dea9999662e5c625b5a1c7657",
		LogPath:     tmp.Join("container.log"),
		Config: map[string]string{
			"labels": "first,second",
		},
		ContainerLabels: map[string]string{
			"first":  "label_value",
			"second": "label_foo",
		},
	})
	require.NoError(b, err)
	defer jsonlogger.Close()

	msg := &logger.Message{
		Line:      []byte("Line that thinks that it is log line from docker\n"),
		Source:    "stderr",
		Timestamp: time.Now().UTC(),
	}

	buf := bytes.NewBuffer(nil)
	require.NoError(b, marshalMessage(msg, nil, buf))
	b.SetBytes(int64(buf.Len()))

	b.ResetTimer()

	chError := make(chan error, b.N+1)
	go func() {
		for i := 0; i < b.N; i++ {
			chError <- jsonlogger.Log(msg)
		}
		chError <- jsonlogger.Close()
	}()

	lw := jsonlogger.(*JSONFileLogger).ReadLogs(logger.ReadConfig{Follow: true})
	watchClose := lw.WatchClose()
	for {
		select {
		case <-lw.Msg:
		case <-watchClose:
			return
		case err := <-chError:
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
