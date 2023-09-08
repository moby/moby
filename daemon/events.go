package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/libnetwork"
	gogotypes "github.com/gogo/protobuf/types"
	swarmapi "github.com/moby/swarmkit/v2/api"
)

// LogContainerEvent generates an event related to a container with only the default attributes.
func (daemon *Daemon) LogContainerEvent(container *container.Container, action string) {
	daemon.LogContainerEventWithAttributes(container, action, map[string]string{})
}

// LogContainerEventWithAttributes generates an event related to a container with specific given attributes.
func (daemon *Daemon) LogContainerEventWithAttributes(container *container.Container, action string, attributes map[string]string) {
	copyAttributes(attributes, container.Config.Labels)
	if container.Config.Image != "" {
		attributes["image"] = container.Config.Image
	}
	attributes["name"] = strings.TrimLeft(container.Name, "/")
	daemon.EventsService.Log(action, events.ContainerEventType, events.Actor{
		ID:         container.ID,
		Attributes: attributes,
	})
}

// LogPluginEvent generates an event related to a plugin with only the default attributes.
func (daemon *Daemon) LogPluginEvent(pluginID, refName, action string) {
	daemon.EventsService.Log(action, events.PluginEventType, events.Actor{
		ID:         pluginID,
		Attributes: map[string]string{"name": refName},
	})
}

// LogVolumeEvent generates an event related to a volume.
func (daemon *Daemon) LogVolumeEvent(volumeID, action string, attributes map[string]string) {
	daemon.EventsService.Log(action, events.VolumeEventType, events.Actor{
		ID:         volumeID,
		Attributes: attributes,
	})
}

// LogNetworkEvent generates an event related to a network with only the default attributes.
func (daemon *Daemon) LogNetworkEvent(nw *libnetwork.Network, action string) {
	daemon.LogNetworkEventWithAttributes(nw, action, map[string]string{})
}

// LogNetworkEventWithAttributes generates an event related to a network with specific given attributes.
func (daemon *Daemon) LogNetworkEventWithAttributes(nw *libnetwork.Network, action string, attributes map[string]string) {
	attributes["name"] = nw.Name()
	attributes["type"] = nw.Type()
	daemon.EventsService.Log(action, events.NetworkEventType, events.Actor{
		ID:         nw.ID(),
		Attributes: attributes,
	})
}

// LogDaemonEventWithAttributes generates an event related to the daemon itself with specific given attributes.
func (daemon *Daemon) LogDaemonEventWithAttributes(action string, attributes map[string]string) {
	if daemon.EventsService != nil {
		if name := hostName(); name != "" {
			attributes["name"] = name
		}
		daemon.EventsService.Log(action, events.DaemonEventType, events.Actor{
			ID:         daemon.id,
			Attributes: attributes,
		})
	}
}

// SubscribeToEvents returns the currently record of events, a channel to stream new events from, and a function to cancel the stream of events.
func (daemon *Daemon) SubscribeToEvents(since, until time.Time, filter filters.Args) ([]events.Message, chan interface{}) {
	return daemon.EventsService.SubscribeTopic(since, until, daemonevents.NewFilter(filter))
}

// UnsubscribeFromEvents stops the event subscription for a client by closing the
// channel where the daemon sends events to.
func (daemon *Daemon) UnsubscribeFromEvents(listener chan interface{}) {
	daemon.EventsService.Evict(listener)
}

// copyAttributes guarantees that labels are not mutated by event triggers.
func copyAttributes(attributes, labels map[string]string) {
	if labels == nil {
		return
	}
	for k, v := range labels {
		attributes[k] = v
	}
}

// ProcessClusterNotifications gets changes from store and add them to event list
func (daemon *Daemon) ProcessClusterNotifications(ctx context.Context, watchStream chan *swarmapi.WatchMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-watchStream:
			if !ok {
				log.G(ctx).Debug("cluster event channel has stopped")
				return
			}
			daemon.generateClusterEvent(message)
		}
	}
}

func (daemon *Daemon) generateClusterEvent(msg *swarmapi.WatchMessage) {
	for _, event := range msg.Events {
		if event.Object == nil {
			log.G(context.TODO()).Errorf("event without object: %v", event)
			continue
		}
		switch v := event.Object.GetObject().(type) {
		case *swarmapi.Object_Node:
			daemon.logNodeEvent(event.Action, v.Node, event.OldObject.GetNode())
		case *swarmapi.Object_Service:
			daemon.logServiceEvent(event.Action, v.Service, event.OldObject.GetService())
		case *swarmapi.Object_Network:
			daemon.logNetworkEvent(event.Action, v.Network)
		case *swarmapi.Object_Secret:
			daemon.logSecretEvent(event.Action, v.Secret)
		case *swarmapi.Object_Config:
			daemon.logConfigEvent(event.Action, v.Config)
		default:
			log.G(context.TODO()).Warnf("unrecognized event: %v", event)
		}
	}
}

func (daemon *Daemon) logNetworkEvent(action swarmapi.WatchActionKind, net *swarmapi.Network) {
	daemon.logClusterEvent(action, net.ID, events.NetworkEventType, eventTimestamp(net.Meta, action), map[string]string{
		"name": net.Spec.Annotations.Name,
	})
}

func (daemon *Daemon) logSecretEvent(action swarmapi.WatchActionKind, secret *swarmapi.Secret) {
	daemon.logClusterEvent(action, secret.ID, events.SecretEventType, eventTimestamp(secret.Meta, action), map[string]string{
		"name": secret.Spec.Annotations.Name,
	})
}

func (daemon *Daemon) logConfigEvent(action swarmapi.WatchActionKind, config *swarmapi.Config) {
	daemon.logClusterEvent(action, config.ID, events.ConfigEventType, eventTimestamp(config.Meta, action), map[string]string{
		"name": config.Spec.Annotations.Name,
	})
}

func (daemon *Daemon) logNodeEvent(action swarmapi.WatchActionKind, node *swarmapi.Node, oldNode *swarmapi.Node) {
	name := node.Spec.Annotations.Name
	if name == "" && node.Description != nil {
		name = node.Description.Hostname
	}
	attributes := map[string]string{
		"name": name,
	}
	eventTime := eventTimestamp(node.Meta, action)
	// In an update event, display the changes in attributes
	if action == swarmapi.WatchActionKindUpdate && oldNode != nil {
		if node.Spec.Availability != oldNode.Spec.Availability {
			attributes["availability.old"] = strings.ToLower(oldNode.Spec.Availability.String())
			attributes["availability.new"] = strings.ToLower(node.Spec.Availability.String())
		}
		if node.Role != oldNode.Role {
			attributes["role.old"] = strings.ToLower(oldNode.Role.String())
			attributes["role.new"] = strings.ToLower(node.Role.String())
		}
		if node.Status.State != oldNode.Status.State {
			attributes["state.old"] = strings.ToLower(oldNode.Status.State.String())
			attributes["state.new"] = strings.ToLower(node.Status.State.String())
		}
		// This handles change within manager role
		if node.ManagerStatus != nil && oldNode.ManagerStatus != nil {
			// leader change
			if node.ManagerStatus.Leader != oldNode.ManagerStatus.Leader {
				if node.ManagerStatus.Leader {
					attributes["leader.old"] = "false"
					attributes["leader.new"] = "true"
				} else {
					attributes["leader.old"] = "true"
					attributes["leader.new"] = "false"
				}
			}
			if node.ManagerStatus.Reachability != oldNode.ManagerStatus.Reachability {
				attributes["reachability.old"] = strings.ToLower(oldNode.ManagerStatus.Reachability.String())
				attributes["reachability.new"] = strings.ToLower(node.ManagerStatus.Reachability.String())
			}
		}
	}

	daemon.logClusterEvent(action, node.ID, events.NodeEventType, eventTime, attributes)
}

func (daemon *Daemon) logServiceEvent(action swarmapi.WatchActionKind, service *swarmapi.Service, oldService *swarmapi.Service) {
	attributes := map[string]string{
		"name": service.Spec.Annotations.Name,
	}
	eventTime := eventTimestamp(service.Meta, action)

	if action == swarmapi.WatchActionKindUpdate && oldService != nil {
		// check image
		if x, ok := service.Spec.Task.GetRuntime().(*swarmapi.TaskSpec_Container); ok {
			containerSpec := x.Container
			if y, ok := oldService.Spec.Task.GetRuntime().(*swarmapi.TaskSpec_Container); ok {
				oldContainerSpec := y.Container
				if containerSpec.Image != oldContainerSpec.Image {
					attributes["image.old"] = oldContainerSpec.Image
					attributes["image.new"] = containerSpec.Image
				}
			} else {
				// This should not happen.
				log.G(context.TODO()).Errorf("service %s runtime changed from %T to %T", service.Spec.Annotations.Name, oldService.Spec.Task.GetRuntime(), service.Spec.Task.GetRuntime())
			}
		}
		// check replicated count change
		if x, ok := service.Spec.GetMode().(*swarmapi.ServiceSpec_Replicated); ok {
			replicas := x.Replicated.Replicas
			if y, ok := oldService.Spec.GetMode().(*swarmapi.ServiceSpec_Replicated); ok {
				oldReplicas := y.Replicated.Replicas
				if replicas != oldReplicas {
					attributes["replicas.old"] = strconv.FormatUint(oldReplicas, 10)
					attributes["replicas.new"] = strconv.FormatUint(replicas, 10)
				}
			} else {
				// This should not happen.
				log.G(context.TODO()).Errorf("service %s mode changed from %T to %T", service.Spec.Annotations.Name, oldService.Spec.GetMode(), service.Spec.GetMode())
			}
		}
		if service.UpdateStatus != nil {
			if oldService.UpdateStatus == nil {
				attributes["updatestate.new"] = strings.ToLower(service.UpdateStatus.State.String())
			} else if service.UpdateStatus.State != oldService.UpdateStatus.State {
				attributes["updatestate.old"] = strings.ToLower(oldService.UpdateStatus.State.String())
				attributes["updatestate.new"] = strings.ToLower(service.UpdateStatus.State.String())
			}
		}
	}
	daemon.logClusterEvent(action, service.ID, events.ServiceEventType, eventTime, attributes)
}

var clusterEventAction = map[swarmapi.WatchActionKind]string{
	swarmapi.WatchActionKindCreate: "create",
	swarmapi.WatchActionKindUpdate: "update",
	swarmapi.WatchActionKindRemove: "remove",
}

func (daemon *Daemon) logClusterEvent(action swarmapi.WatchActionKind, id string, eventType events.Type, eventTime time.Time, attributes map[string]string) {
	daemon.EventsService.PublishMessage(events.Message{
		Action: clusterEventAction[action],
		Type:   eventType,
		Actor: events.Actor{
			ID:         id,
			Attributes: attributes,
		},
		Scope:    "swarm",
		Time:     eventTime.UTC().Unix(),
		TimeNano: eventTime.UTC().UnixNano(),
	})
}

func eventTimestamp(meta swarmapi.Meta, action swarmapi.WatchActionKind) time.Time {
	var eventTime time.Time
	switch action {
	case swarmapi.WatchActionKindCreate:
		eventTime, _ = gogotypes.TimestampFromProto(meta.CreatedAt)
	case swarmapi.WatchActionKindUpdate:
		eventTime, _ = gogotypes.TimestampFromProto(meta.UpdatedAt)
	case swarmapi.WatchActionKindRemove:
		// There is no timestamp from store message for remove operations.
		// Use current time.
		eventTime = time.Now()
	}
	return eventTime
}
