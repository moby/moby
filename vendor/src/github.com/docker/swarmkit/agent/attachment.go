package agent

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"golang.org/x/net/context"
)

// AttachmentListener allow to receive notifications about network attachment objects.
type AttachmentListener interface {
	// Notify notifies the listener about the updates over the network attachment objects.
	Notify([]*api.NetworkAttachment)
}

// NetworkAttachmentManager provides control over network attachments on this node.
type NetworkAttachmentManager interface {
	// CreateAttachment allows the node to request the allocation of resources
	// needed for a network attachment on this node.
	CreateAttachment(context.Context, *api.NetworkAttachmentConfig) (string, error)

	// RemoveAttachment allows the node to request the release of
	// the resources associated to the network attachment.
	RemoveAttachment(context.Context, string) error

	// ListAttachments lists all attachments currently allocated on this node.
	ListAttachments() []*api.NetworkAttachment

	// Register allows clients to register to network attachment notifications.
	Register(AttachmentListener) (string, error)

	// Leave leaves the notification clients pool.
	Leave(string)
}

type attachmentManager struct {
	agent       *Agent
	attachments map[string]*api.NetworkAttachment
	listeners   map[string]AttachmentListener
	sync.RWMutex
}

func newAttachmentManager(agent *Agent) *attachmentManager {
	return &attachmentManager{
		agent:       agent,
		attachments: make(map[string]*api.NetworkAttachment),
		listeners:   make(map[string]AttachmentListener),
	}
}

// CreateAttachment allows the node to request the allocation of
// resources needed for a network attachment on this node.
func (m *attachmentManager) CreateAttachment(ctx context.Context, config *api.NetworkAttachmentConfig) (string, error) {
	client := api.NewResourceAllocatorClient(m.agent.config.Conn)
	req := &api.CreateNetworkAttachmentRequest{Config: config}
	rsp, err := client.CreateNetworkAttachment(ctx, req)
	if err != nil {
		return "", err
	}
	return rsp.ID, nil
}

// RemoveAttachment allows the node to request the release of
// the resources associated to the network attachment.
func (m *attachmentManager) RemoveAttachment(ctx context.Context, id string) error {
	client := api.NewResourceAllocatorClient(m.agent.config.Conn)
	req := &api.RemoveNetworkAttachmentRequest{ID: id}
	_, err := client.RemoveNetworkAttachment(ctx, req)
	// Update local db regardless
	m.Lock()
	delete(m.attachments, id)
	m.Unlock()
	return err
}

// ListAttachments lists all attachments currently allocated on this node.
func (m *attachmentManager) ListAttachments() []*api.NetworkAttachment {
	list := make([]*api.NetworkAttachment, 0, len(m.attachments))
	m.Lock()
	for _, a := range m.attachments {
		list = append(list, a)
	}
	m.Unlock()
	return list
}

// Register allows clients to register for notifications.
func (m *attachmentManager) Register(l AttachmentListener) (string, error) {
	if l == nil {
		return "", fmt.Errorf("invalid listener")
	}
	id := identity.NewID()
	m.Lock()
	m.listeners[id] = l
	m.Unlock()
	logrus.Debugf("Notifier: AttachmentListener (%s) registered", id)
	return id, nil
}

// Leave let the client leave the notification pool.
func (m *attachmentManager) Leave(id string) {
	m.Lock()
	delete(m.listeners, id)
	m.Unlock()
	logrus.Debugf("Notifier: AttachmentListener (%s) left", id)
}

func (m *attachmentManager) notify(ctx context.Context, eal []*api.NetworkAttachment) {
	for _, a := range eal {
		m.Lock()
		m.attachments[a.ID] = a
		m.Unlock()
	}
	for i, l := range m.listeners {
		log.G(ctx).Debugf("Notifier: Notifying listener (%s):", i)
		l.Notify(eal)
	}
}
