package otelutil

import (
	"context"

	"github.com/containerd/log"
	"go.opentelemetry.io/otel/baggage"
)

// TriggerKey is the key used for the 'trigger' member in the baggage. It is
// used to know what triggered a code path (e.g. API call, libnet init, etc...)
const TriggerKey = "trigger"

// MustNewBaggage creates an OTel Baggage containing the provided members. It
// panics if the baggage cannot be created.
//
// DO NOT USE this function with dynamic values.
func MustNewBaggage(members ...baggage.Member) baggage.Baggage {
	b, err := baggage.New(members...)
	if err != nil {
		log.G(context.Background()).WithFields(log.Fields{
			"error":   err,
			"members": members,
		}).Fatal("OTel baggage creation failure")
	}
	return b
}

// MustNewMemberRaw creates an OTel Baggage member with the provided key and
// value. It panics if the key or value aren't valid UTF-8 strings.
//
// DO NOT USE this function with dynamic key/value.
func MustNewMemberRaw(key, value string) baggage.Member {
	m, err := baggage.NewMemberRaw(key, value)
	if err != nil {
		log.G(context.Background()).WithFields(log.Fields{
			"error": err,
			"key":   key,
			"value": value,
		}).Fatal("OTel baggage member creation failure")
	}
	return m
}
