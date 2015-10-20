// Package progressreader provides a Reader with a progress bar that can be
// printed out using the streamformatter package.
package progressreader

import (
	"io"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/streamformatter"
)

// Config contains the configuration for a Reader with progress bar.
type Config struct {
	In         io.ReadCloser // Stream to read from
	out        *streamformatter.StdoutFormattedWriter
	Size       int64
	Current    int64
	LastUpdate int64
	NewLines   bool
	ID         string
	Action     string
}

// NewReader initializes a new progress reader configuration.
// It uses a JSON stream formatted writer as default output.
func NewReader(out io.Writer, in io.ReadCloser, size int64, newLines bool, id, action string) *Config {
	return NewReaderWithFormatter(streamformatter.NewStdoutJSONFormattedWriter(out), in, size, newLines, id, action)
}

// NewReaderWithFormatter initializes a new progress reader configuration with a given stream formatted writer.
func NewReaderWithFormatter(out *streamformatter.StdoutFormattedWriter, in io.ReadCloser, size int64, newLines bool, id, action string) *Config {
	return &Config{
		In:       in,
		out:      out,
		Size:     size,
		NewLines: newLines,
		ID:       id,
		Action:   action,
	}
}

func (config *Config) Read(p []byte) (n int, err error) {
	read, err := config.In.Read(p)
	config.Current += int64(read)
	updateEvery := int64(1024 * 512) //512kB
	if config.Size > 0 {
		// Update progress for every 1% read if 1% < 512kB
		if increment := int64(0.01 * float64(config.Size)); increment < updateEvery {
			updateEvery = increment
		}
	}
	if config.Current-config.LastUpdate > updateEvery || err != nil {
		updateProgress(config)
		config.LastUpdate = config.Current
	}

	if err != nil && read == 0 {
		updateProgress(config)
		if config.NewLines {
			config.out.WriteStatus("", "")
		}
	}
	return read, err
}

// Close closes the reader (Config).
func (config *Config) Close() error {
	if config.Current < config.Size {
		//print a full progress bar when closing prematurely
		config.Current = config.Size
		updateProgress(config)
	}
	return config.In.Close()
}

func updateProgress(config *Config) {
	progress := jsonmessage.JSONProgress{Current: config.Current, Total: config.Size}
	config.out.WriteProgress(config.ID, config.Action, &progress)
}
