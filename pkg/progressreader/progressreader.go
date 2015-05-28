package progressreader

import (
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/streamformatter"
)

// Reader with progress bar
type Config struct {
	In         io.ReadCloser // Stream to read from
	Out        io.Writer     // Where to send progress bar to
	Formatter  *streamformatter.StreamFormatter
	Size       int
	Current    int
	LastUpdate int
	NewLines   bool
	ID         string
	Action     string
}

func New(newReader Config) *Config {
	return &newReader
}
func (config *Config) Read(p []byte) (n int, err error) {
	read, err := config.In.Read(p)
	config.Current += read
	updateEvery := 1024 * 512 //512kB
	if config.Size > 0 {
		// Update progress for every 1% read if 1% < 512kB
		if increment := int(0.01 * float64(config.Size)); increment < updateEvery {
			updateEvery = increment
		}
	}
	if config.Current-config.LastUpdate > updateEvery || err != nil {
		config.Out.Write(config.Formatter.FormatProgress(config.ID, config.Action, &jsonmessage.JSONProgress{Current: config.Current, Total: config.Size}))
		config.LastUpdate = config.Current
	}
	// Send newline when complete
	if config.NewLines && err != nil && read == 0 {
		config.Out.Write(config.Formatter.FormatStatus("", ""))
	}
	return read, err
}
func (config *Config) Close() error {
	config.Current = config.Size
	config.Out.Write(config.Formatter.FormatProgress(config.ID, config.Action, &jsonmessage.JSONProgress{Current: config.Current, Total: config.Size}))
	return config.In.Close()
}
