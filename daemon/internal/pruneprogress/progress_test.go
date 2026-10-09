package pruneprogress

import (
	"sync"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
	"gotest.tools/v3/assert"
)

func TestReporterRequestIsolation(t *testing.T) {
	var wg sync.WaitGroup
	for _, id := range []string{"first request", "second request"} {
		wg.Go(func() {
			var messages []jsonstream.Message
			ctx := WithReporter(t.Context(), func(message jsonstream.Message) {
				messages = append(messages, message)
			})
			Notify(ctx, id, "deleted")
			assert.DeepEqual(t, messages, []jsonstream.Message{{ID: id, Status: "deleted"}})
		})
	}
	wg.Wait()
}
