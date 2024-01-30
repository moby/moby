/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package streaming

import (
	"context"

	"github.com/containerd/typeurl/v2"
)

type StreamManager interface {
	StreamGetter
	Register(context.Context, string, Stream) error
}

type StreamGetter interface {
	Get(context.Context, string) (Stream, error)
}

type StreamCreator interface {
	Create(context.Context, string) (Stream, error)
}

type Stream interface {
	// Send sends the object on the stream
	Send(typeurl.Any) error

	// Recv receives an object on the stream
	Recv() (typeurl.Any, error)

	// Close closes the stream
	Close() error
}
