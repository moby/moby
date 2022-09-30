package pubsub // import "github.com/docker/docker/pkg/pubsub"

import "github.com/moby/pubsub"

// NewPublisher creates a new pub/sub publisher to broadcast messages.
// The duration is used as the send timeout as to not block the publisher publishing
// messages to other clients if one client is slow or unresponsive.
// The buffer is used when creating new channels for subscribers.
//
// Deprecated: use github.com/moby/pubsub.NewPublisher
var NewPublisher = pubsub.NewPublisher

// Publisher is basic pub/sub structure. Allows to send events and subscribe
// to them. Can be safely used from multiple goroutines.
//
// Deprecated: use github.com/moby/pubsub.Publisher
type Publisher = pubsub.Publisher
