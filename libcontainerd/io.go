package libcontainerd

import "github.com/containerd/containerd"

// Config returns the containerd.IOConfig of this pipe set
func (p *IOPipe) Config() containerd.IOConfig {
	return p.config
}

// Cancel aborts ongoing operations if they have not completed yet
func (p *IOPipe) Cancel() {
	p.cancel()
}

// Wait waits for io operations to finish
func (p *IOPipe) Wait() {
}

// Close closes the underlying pipes
func (p *IOPipe) Close() error {
	p.cancel()

	if p.Stdin != nil {
		p.Stdin.Close()
	}

	if p.Stdout != nil {
		p.Stdout.Close()
	}

	if p.Stderr != nil {
		p.Stderr.Close()
	}

	return nil
}
