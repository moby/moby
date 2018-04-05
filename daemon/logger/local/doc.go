// Package local provides a logger implementation that stores logs on disk.
//
// Log messages are encoded as protobufs with a header and footer for each message.
// The header and footer are big-endian binary encoded uint32 values which indicate the size of the log message.
// The header and footer of each message allows you to efficiently read through a file either forwards or in
// backwards (such as is the case when tailing a file)
//
// Example log message format: [22][This is a log message.][22][28][This is another log message.][28]
package local // import "github.com/docker/docker/daemon/logger/local"
