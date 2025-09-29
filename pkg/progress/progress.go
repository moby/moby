package progress

import (
	"fmt"

	"github.com/moby/moby/api/pkg/progress"
)

// Progress represents the progress of a transfer.
type Progress = progress.Progress

// Output is an interface for writing progress information. It's
// like a writer for progress, but we don't call it Writer because
// that would be confusing next to ProgressReader (also, because it
// doesn't implement the io.Writer interface).
type Output = progress.Output

// ChanOutput returns an Output that writes progress updates to the
// supplied channel.
func ChanOutput(progressChan chan<- progress.Progress) progress.Output {
	return progress.ChanOutput(progressChan)
}

// DiscardOutput returns an Output that discards progress
func DiscardOutput() progress.Output {
	return progress.DiscardOutput()
}

// Update is a convenience function to write a progress update to the channel.
func Update(out progress.Output, id, action string) {
	out.WriteProgress(progress.Progress{ID: id, Action: action})
}

// Updatef is a convenience function to write a printf-formatted progress update
// to the channel.
func Updatef(out progress.Output, id, format string, a ...interface{}) {
	Update(out, id, fmt.Sprintf(format, a...))
}

// Message is a convenience function to write a progress message to the channel.
func Message(out progress.Output, id, message string) {
	out.WriteProgress(progress.Progress{ID: id, Message: message})
}

// Messagef is a convenience function to write a printf-formatted progress
// message to the channel.
func Messagef(out progress.Output, id, format string, a ...interface{}) {
	Message(out, id, fmt.Sprintf(format, a...))
}

// Aux sends auxiliary information over a progress interface, which will not be
// formatted for the UI. This is used for things such as push signing.
func Aux(out progress.Output, a interface{}) {
	out.WriteProgress(progress.Progress{Aux: a})
}
