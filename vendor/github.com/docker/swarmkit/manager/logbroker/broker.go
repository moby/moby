package logbroker

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/watch"
	"golang.org/x/net/context"
)

var (
	errAlreadyRunning = errors.New("broker is already running")
	errNotRunning     = errors.New("broker is not running")
)

type logMessage struct {
	*api.PublishLogsMessage
	completed bool
	err       error
}

// LogBroker coordinates log subscriptions to services and tasks. Clients can
// publish and subscribe to logs channels.
//
// Log subscriptions are pushed to the work nodes by creating log subscsription
// tasks. As such, the LogBroker also acts as an orchestrator of these tasks.
type LogBroker struct {
	mu                sync.RWMutex
	logQueue          *watch.Queue
	subscriptionQueue *watch.Queue

	registeredSubscriptions map[string]*subscription
	connectedNodes          map[string]struct{}

	pctx      context.Context
	cancelAll context.CancelFunc

	store *store.MemoryStore
}

// New initializes and returns a new LogBroker
func New(store *store.MemoryStore) *LogBroker {
	return &LogBroker{
		store: store,
	}
}

// Run the log broker
func (lb *LogBroker) Run(ctx context.Context) error {
	lb.mu.Lock()

	if lb.cancelAll != nil {
		lb.mu.Unlock()
		return errAlreadyRunning
	}

	lb.pctx, lb.cancelAll = context.WithCancel(ctx)
	lb.logQueue = watch.NewQueue()
	lb.subscriptionQueue = watch.NewQueue()
	lb.registeredSubscriptions = make(map[string]*subscription)
	lb.connectedNodes = make(map[string]struct{})
	lb.mu.Unlock()

	select {
	case <-lb.pctx.Done():
		return lb.pctx.Err()
	}
}

// Stop stops the log broker
func (lb *LogBroker) Stop() error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.cancelAll == nil {
		return errNotRunning
	}
	lb.cancelAll()
	lb.cancelAll = nil

	lb.logQueue.Close()
	lb.subscriptionQueue.Close()

	return nil
}

func validateSelector(selector *api.LogSelector) error {
	if selector == nil {
		return grpc.Errorf(codes.InvalidArgument, "log selector must be provided")
	}

	if len(selector.ServiceIDs) == 0 && len(selector.TaskIDs) == 0 && len(selector.NodeIDs) == 0 {
		return grpc.Errorf(codes.InvalidArgument, "log selector must not be empty")
	}

	return nil
}

func (lb *LogBroker) newSubscription(selector *api.LogSelector, options *api.LogSubscriptionOptions) *subscription {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	subscription := newSubscription(lb.store, &api.SubscriptionMessage{
		ID:       identity.NewID(),
		Selector: selector,
		Options:  options,
	}, lb.subscriptionQueue)

	return subscription
}

func (lb *LogBroker) getSubscription(id string) *subscription {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	subscription, ok := lb.registeredSubscriptions[id]
	if !ok {
		return nil
	}
	return subscription
}

func (lb *LogBroker) registerSubscription(subscription *subscription) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.registeredSubscriptions[subscription.message.ID] = subscription
	lb.subscriptionQueue.Publish(subscription)

	// Mark nodes that won't receive the message as done.
	for _, node := range subscription.Nodes() {
		if _, ok := lb.connectedNodes[node]; !ok {
			subscription.Done(node, fmt.Errorf("node %s is not available", node))
		}
	}
}

func (lb *LogBroker) unregisterSubscription(subscription *subscription) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	delete(lb.registeredSubscriptions, subscription.message.ID)

	subscription.Close()
	lb.subscriptionQueue.Publish(subscription)
}

// watchSubscriptions grabs all current subscriptions and notifies of any
// subscription change for this node.
//
// Subscriptions may fire multiple times and the caller has to protect against
// dupes.
func (lb *LogBroker) watchSubscriptions(nodeID string) ([]*subscription, chan events.Event, func()) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Watch for subscription changes for this node.
	ch, cancel := lb.subscriptionQueue.CallbackWatch(events.MatcherFunc(func(event events.Event) bool {
		s := event.(*subscription)
		return s.Contains(nodeID)
	}))

	// Grab current subscriptions.
	var subscriptions []*subscription
	for _, s := range lb.registeredSubscriptions {
		if s.Contains(nodeID) {
			subscriptions = append(subscriptions, s)
		}
	}

	return subscriptions, ch, cancel
}

func (lb *LogBroker) subscribe(id string) (chan events.Event, func()) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	return lb.logQueue.CallbackWatch(events.MatcherFunc(func(event events.Event) bool {
		publish := event.(*logMessage)
		return publish.SubscriptionID == id
	}))
}

func (lb *LogBroker) publish(log *api.PublishLogsMessage) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	lb.logQueue.Publish(&logMessage{PublishLogsMessage: log})
}

// SubscribeLogs creates a log subscription and streams back logs
func (lb *LogBroker) SubscribeLogs(request *api.SubscribeLogsRequest, stream api.Logs_SubscribeLogsServer) error {
	ctx := stream.Context()

	if err := validateSelector(request.Selector); err != nil {
		return err
	}

	subscription := lb.newSubscription(request.Selector, request.Options)
	subscription.Run(lb.pctx)
	defer subscription.Stop()

	log := log.G(ctx).WithFields(
		logrus.Fields{
			"method":          "(*LogBroker).SubscribeLogs",
			"subscription.id": subscription.message.ID,
		},
	)
	log.Debug("subscribed")

	publishCh, publishCancel := lb.subscribe(subscription.message.ID)
	defer publishCancel()

	lb.registerSubscription(subscription)
	defer lb.unregisterSubscription(subscription)

	completed := subscription.Wait(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lb.pctx.Done():
			return lb.pctx.Err()
		case event := <-publishCh:
			publish := event.(*logMessage)
			if publish.completed {
				return publish.err
			}
			if err := stream.Send(&api.SubscribeLogsMessage{
				Messages: publish.Messages,
			}); err != nil {
				return err
			}
		case <-completed:
			completed = nil
			lb.logQueue.Publish(&logMessage{
				PublishLogsMessage: &api.PublishLogsMessage{
					SubscriptionID: subscription.message.ID,
				},
				completed: true,
				err:       subscription.Err(),
			})
		}
	}
}

func (lb *LogBroker) nodeConnected(nodeID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.connectedNodes[nodeID] = struct{}{}
}

func (lb *LogBroker) nodeDisconnected(nodeID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	delete(lb.connectedNodes, nodeID)
}

// ListenSubscriptions returns a stream of matching subscriptions for the current node
func (lb *LogBroker) ListenSubscriptions(request *api.ListenSubscriptionsRequest, stream api.LogBroker_ListenSubscriptionsServer) error {
	remote, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}

	lb.nodeConnected(remote.NodeID)
	defer lb.nodeDisconnected(remote.NodeID)

	log := log.G(stream.Context()).WithFields(
		logrus.Fields{
			"method": "(*LogBroker).ListenSubscriptions",
			"node":   remote.NodeID,
		},
	)
	subscriptions, subscriptionCh, subscriptionCancel := lb.watchSubscriptions(remote.NodeID)
	defer subscriptionCancel()

	log.Debug("node registered")

	activeSubscriptions := make(map[string]*subscription)
	defer func() {
		// If the worker quits, mark all active subscriptions as finished.
		for _, subscription := range activeSubscriptions {
			subscription.Done(remote.NodeID, fmt.Errorf("node %s disconnected unexpectedly", remote.NodeID))
		}
	}()

	// Start by sending down all active subscriptions.
	for _, subscription := range subscriptions {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-lb.pctx.Done():
			return nil
		default:
		}

		if err := stream.Send(subscription.message); err != nil {
			log.Error(err)
			return err
		}
		activeSubscriptions[subscription.message.ID] = subscription
	}

	// Send down new subscriptions.
	for {
		select {
		case v := <-subscriptionCh:
			subscription := v.(*subscription)

			if subscription.Closed() {
				log.WithField("subscription.id", subscription.message.ID).Debug("subscription closed")
				delete(activeSubscriptions, subscription.message.ID)
			} else {
				// Avoid sending down the same subscription multiple times
				if _, ok := activeSubscriptions[subscription.message.ID]; ok {
					continue
				}
				activeSubscriptions[subscription.message.ID] = subscription
				log.WithField("subscription.id", subscription.message.ID).Debug("subscription added")
			}
			if err := stream.Send(subscription.message); err != nil {
				log.Error(err)
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-lb.pctx.Done():
			return nil
		}
	}
}

// PublishLogs publishes log messages for a given subscription
func (lb *LogBroker) PublishLogs(stream api.LogBroker_PublishLogsServer) (err error) {
	remote, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}

	var currentSubscription *subscription
	defer func() {
		if currentSubscription != nil {
			currentSubscription.Done(remote.NodeID, err)
		}
	}()

	for {
		log, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&api.PublishLogsResponse{})
		}
		if err != nil {
			return err
		}

		if log.SubscriptionID == "" {
			return grpc.Errorf(codes.InvalidArgument, "missing subscription ID")
		}

		if currentSubscription == nil {
			currentSubscription = lb.getSubscription(log.SubscriptionID)
			if currentSubscription == nil {
				return grpc.Errorf(codes.NotFound, "unknown subscription ID")
			}
		} else {
			if log.SubscriptionID != currentSubscription.message.ID {
				return grpc.Errorf(codes.InvalidArgument, "different subscription IDs in the same session")
			}
		}

		// Make sure logs are emitted using the right Node ID to avoid impersonation.
		for _, msg := range log.Messages {
			if msg.Context.NodeID != remote.NodeID {
				return grpc.Errorf(codes.PermissionDenied, "invalid NodeID: expected=%s;received=%s", remote.NodeID, msg.Context.NodeID)
			}
		}

		lb.publish(log)
	}
}
