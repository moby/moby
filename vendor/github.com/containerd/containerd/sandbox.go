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

package containerd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/protobuf/types"
	api "github.com/containerd/containerd/sandbox"
	"github.com/containerd/typeurl/v2"
)

// Sandbox is a high level client to containerd's sandboxes.
type Sandbox interface {
	// ID is a sandbox identifier
	ID() string
	// PID returns sandbox's process PID or error if its not yet started.
	PID() (uint32, error)
	// NewContainer creates new container that will belong to this sandbox
	NewContainer(ctx context.Context, id string, opts ...NewContainerOpts) (Container, error)
	// Labels returns the labels set on the sandbox
	Labels(ctx context.Context) (map[string]string, error)
	// Start starts new sandbox instance
	Start(ctx context.Context) error
	// Stop sends stop request to the shim instance.
	Stop(ctx context.Context) error
	// Wait blocks until sandbox process exits.
	Wait(ctx context.Context) (<-chan ExitStatus, error)
	// Shutdown removes sandbox from the metadata store and shutdowns shim instance.
	Shutdown(ctx context.Context) error
}

type sandboxClient struct {
	pid      *uint32
	client   *Client
	metadata api.Sandbox
}

func (s *sandboxClient) ID() string {
	return s.metadata.ID
}

func (s *sandboxClient) PID() (uint32, error) {
	if s.pid == nil {
		return 0, fmt.Errorf("sandbox not started")
	}

	return *s.pid, nil
}

func (s *sandboxClient) NewContainer(ctx context.Context, id string, opts ...NewContainerOpts) (Container, error) {
	return s.client.NewContainer(ctx, id, append(opts, WithSandbox(s.ID()))...)
}

func (s *sandboxClient) Labels(ctx context.Context) (map[string]string, error) {
	sandbox, err := s.client.SandboxStore().Get(ctx, s.ID())
	if err != nil {
		return nil, err
	}

	return sandbox.Labels, nil
}

func (s *sandboxClient) Start(ctx context.Context) error {
	resp, err := s.client.SandboxController().Start(ctx, s.ID())
	if err != nil {
		return err
	}

	s.pid = &resp.Pid
	return nil
}

func (s *sandboxClient) Wait(ctx context.Context) (<-chan ExitStatus, error) {
	c := make(chan ExitStatus, 1)
	go func() {
		defer close(c)

		exitStatus, err := s.client.SandboxController().Wait(ctx, s.ID())
		if err != nil {
			c <- ExitStatus{
				code: UnknownExitStatus,
				err:  err,
			}
			return
		}

		c <- ExitStatus{
			code:     exitStatus.ExitStatus,
			exitedAt: exitStatus.ExitedAt,
		}
	}()

	return c, nil
}

func (s *sandboxClient) Stop(ctx context.Context) error {
	return s.client.SandboxController().Stop(ctx, s.ID())
}

func (s *sandboxClient) Shutdown(ctx context.Context) error {
	if err := s.client.SandboxController().Shutdown(ctx, s.ID()); err != nil {
		return fmt.Errorf("failed to shutdown sandbox: %w", err)
	}

	if err := s.client.SandboxStore().Delete(ctx, s.ID()); err != nil {
		return fmt.Errorf("failed to delete sandbox from store: %w", err)
	}

	return nil
}

// NewSandbox creates new sandbox client
func (c *Client) NewSandbox(ctx context.Context, sandboxID string, opts ...NewSandboxOpts) (Sandbox, error) {
	if sandboxID == "" {
		return nil, errors.New("sandbox ID must be specified")
	}

	newSandbox := api.Sandbox{
		ID:        sandboxID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	for _, opt := range opts {
		if err := opt(ctx, c, &newSandbox); err != nil {
			return nil, err
		}
	}

	metadata, err := c.SandboxStore().Create(ctx, newSandbox)
	if err != nil {
		return nil, err
	}

	return &sandboxClient{
		pid:      nil, // Not yet started
		client:   c,
		metadata: metadata,
	}, nil
}

// LoadSandbox laods existing sandbox metadata object using the id
func (c *Client) LoadSandbox(ctx context.Context, id string) (Sandbox, error) {
	sandbox, err := c.SandboxStore().Get(ctx, id)
	if err != nil {
		return nil, err
	}

	status, err := c.SandboxController().Status(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("failed to load sandbox %s, status request failed: %w", id, err)
	}

	return &sandboxClient{
		pid:      &status.Pid,
		client:   c,
		metadata: sandbox,
	}, nil
}

// NewSandboxOpts is a sandbox options and extensions to be provided by client
type NewSandboxOpts func(ctx context.Context, client *Client, sandbox *api.Sandbox) error

// WithSandboxRuntime allows a user to specify the runtime to be used to run a sandbox
func WithSandboxRuntime(name string, options interface{}) NewSandboxOpts {
	return func(ctx context.Context, client *Client, s *api.Sandbox) error {
		if options == nil {
			options = &types.Empty{}
		}

		opts, err := typeurl.MarshalAny(options)
		if err != nil {
			return fmt.Errorf("failed to marshal sandbox runtime options: %w", err)
		}

		s.Runtime = api.RuntimeOpts{
			Name:    name,
			Options: opts,
		}

		return nil
	}
}

// WithSandboxSpec will provide the sandbox runtime spec
func WithSandboxSpec(s *oci.Spec, opts ...oci.SpecOpts) NewSandboxOpts {
	return func(ctx context.Context, client *Client, sandbox *api.Sandbox) error {
		c := &containers.Container{ID: sandbox.ID}

		if err := oci.ApplyOpts(ctx, client, c, s, opts...); err != nil {
			return err
		}

		spec, err := typeurl.MarshalAny(s)
		if err != nil {
			return fmt.Errorf("failed to marshal spec: %w", err)
		}

		sandbox.Spec = spec
		return nil
	}
}

// WithSandboxExtension attaches an extension to sandbox
func WithSandboxExtension(name string, ext interface{}) NewSandboxOpts {
	return func(ctx context.Context, client *Client, s *api.Sandbox) error {
		if s.Extensions == nil {
			s.Extensions = make(map[string]typeurl.Any)
		}

		any, err := typeurl.MarshalAny(ext)
		if err != nil {
			return fmt.Errorf("failed to marshal sandbox extension: %w", err)
		}

		s.Extensions[name] = any
		return err
	}
}

// WithSandboxLabels attaches map of labels to sandbox
func WithSandboxLabels(labels map[string]string) NewSandboxOpts {
	return func(ctx context.Context, client *Client, sandbox *api.Sandbox) error {
		sandbox.Labels = labels
		return nil
	}
}
