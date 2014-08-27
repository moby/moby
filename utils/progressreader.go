package utils

import (
	"io"
	"time"

	"github.com/docker/docker/pkg/units"
)

// Reader with progress bar
type progressReader struct {
	reader   io.ReadCloser // Stream to read from
	output   io.Writer     // Where to send progress bar to
	progress JSONProgress
	ID       string
	action   string
	sf       *StreamFormatter
	newLine  bool

	stop   chan struct{}
	ticker *time.Ticker
}

func (r *progressReader) Read(p []byte) (n int, err error) {
	read, err := r.reader.Read(p)
	if err == io.EOF {
		r.progress.Current = r.progress.Total
		close(r.stop)
		r.ticker.Stop()
	} else {
		r.progress.Current += read
	}
	return read, err
}
func (r *progressReader) Close() error {
	r.progress.Current = r.progress.Total
	r.output.Write(r.sf.FormatProgress(r.ID, r.action, &r.progress))
	return r.reader.Close()
}

func (r *progressReader) Update() {
	var (
		previous      int
		previousSpeed int
	)
	for {
		select {
		case <-r.ticker.C:
			speed := 10 * (r.progress.Current - previous)
			// use median to smooth speed display
			r.progress.Speed = units.HumanSize(int64((speed+previousSpeed)/2)) + "/s"
			previous = r.progress.Current
			previousSpeed = speed
			r.output.Write(r.sf.FormatProgress(r.ID, r.action, &r.progress))
		case <-r.stop:
			r.output.Write(r.sf.FormatProgress(r.ID, r.action, &r.progress))
			if r.newLine {
				r.output.Write(r.sf.FormatStatus("", ""))
			}
			return
		}
	}
}

func ProgressReader(r io.ReadCloser, size int, output io.Writer, sf *StreamFormatter, newline bool, ID, action string) *progressReader {
	pr := progressReader{
		reader:   r,
		output:   NewWriteFlusher(output),
		ID:       ID,
		action:   action,
		progress: JSONProgress{Total: size, Start: time.Now().UTC().Unix()},
		sf:       sf,
		newLine:  newline,
		stop:     make(chan struct{}),
		ticker:   time.NewTicker(100 * time.Millisecond),
	}
	go pr.Update()
	return &pr
}
