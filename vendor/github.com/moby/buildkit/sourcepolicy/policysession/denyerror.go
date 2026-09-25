package policysession

import (
	"github.com/containerd/typeurl/v2"
	spb "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/pkg/errors"
)

func init() {
	typeurl.Register((*DecisionResponse)(nil), "github.com/moby/buildkit", "policysession.DecisionResponse+json")
}

// DenyMessagesError wraps an error with policy deny messages so they can be
// propagated as a typed error detail.
type DenyMessagesError struct {
	Messages []*DenyMessage
	error
}

func (e *DenyMessagesError) Unwrap() error {
	return e.error
}

func (e *DenyMessagesError) ToProto() grpcerrors.TypedErrorProto {
	return &DecisionResponse{
		Action:       spb.PolicyAction_DENY,
		DenyMessages: e.Messages,
	}
}

// WrapDenyMessages adds deny messages to an error when available.
func WrapDenyMessages(err error, msgs []*DenyMessage) error {
	if err == nil || len(msgs) == 0 {
		return err
	}
	return &DenyMessagesError{Messages: msgs, error: err}
}

// DenyMessages extracts policy deny messages from an error chain.
func DenyMessages(err error) []*DenyMessage {
	var out []*DenyMessage
	var de *DenyMessagesError
	if errors.As(err, &de) {
		out = DenyMessages(de.Unwrap())
		out = append(out, de.Messages...)
	}
	return out
}

// WrapError implements grpcerrors.TypedErrorProto for DecisionResponse.
func (d *DecisionResponse) WrapError(err error) error {
	return WrapDenyMessages(err, d.GetDenyMessages())
}
