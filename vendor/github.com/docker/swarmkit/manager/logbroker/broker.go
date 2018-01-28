package logbroker

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/watch"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
// Log subscriptions are pushed to the work nodes by creating log subscription
// tasks. As such, the LogBroker also acts as an orchestrator of these tasks.
type LogBroker struct {
	mu                sync.RWMutex
	logQueue          *watch.Queue
	subscriptionQueue *watch.Queue

	registeredSubscriptions map[string]*subscription
	subscriptionsByNode     map[string]map[*subscription]struct{}

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

// Start starts the log broker
func (lb *LogBroker) Start(ctx context.Context) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.cancelAll != nil {
		return errAlreadyRunning
	}

	lb.pctx, lb.cancelAll = context.WithCancel(ctx)
	lb.logQueue = watch.NewQueue()
	lb.subscriptionQueue = watch.NewQueue()
	lb.registeredSubscriptions = make(map[string]*subscription)
	lb.subscriptionsByNode = make(map[string]map[*subscription]struct{})
	return nil
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
		return status.Errorf(codes.InvalidArgument, "log selector must be provided")
	}

	if len(selector.ServiceIDs) == 0 && len(selector.TaskIDs) == 0 && len(selector.NodeIDs) == 0 {
		return status.Errorf(codes.InvalidArgument, "log selector must not be empty")
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

	for _, node := range subscription.Nodes() {
		if _, ok := lb.subscriptionsByNode[node]; !ok {
			// Mark nodes that won't receive the message as done.
			subscription.Done(node, fmt.Errorf("node %s is not available", node))
		} else {
			// otherwise, add the subscription to the node's subscriptions list
			lb.subscriptionsByNode[node][subscription] = struct{}{}
		}
	}
}

func (lb *LogBroker) unregisterSubscription(subscription *subscription) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	delete(lb.registeredSubscriptions, subscription.message.ID)

	// remove the subscription from all of the nodes
	for _, node := range subscription.Nodes() {
		// but only if a node exists
		if _, ok := lb.subscriptionsByNode[node]; ok {
			delete(lb.subscriptionsByNode[node], subscription)
		}
	}

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

// markDone wraps (*Subscription).Done() so that the removal of the sub from
// the node's subscription list is possible
func (lb *LogBroker) markDone(sub *subscription, nodeID string, err error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// remove the subscription from the node's subscription list, if it exists
	if _, ok := lb.subscriptionsByNode[nodeID]; ok {
		delete(lb.subscriptionsByNode[nodeID], sub)
	}

	// mark the sub as done
	sub.Done(nodeID, err)
}

// SubscribeLogs creates a log subscription and streams back logs
func (lb *LogBroker) SubscribeLogs(request *api.SubscribeLogsRequest, stream api.Logs_SubscribeLogsServer) error {
	ctx := stream.Context()

	if err := validateSelector(request.Selector); err != nil {
		return err
	}

	lb.mu.Lock()
	pctx := lb.pctx
	lb.mu.Unlock()
	if pctx == nil {
		return errNotRunning
	}

	subscription := lb.newSubscription(request.Selector, request.Options)
	subscription.Run(pctx)
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
		case <-pctx.Done():
			return pctx.Err()
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

	if _, ok := lb.subscriptionsByNode[nodeID]; !ok {
		lb.subscriptionsByNode[nodeID] = make(map[*subscription]struct{})
	}
}

func (lb *LogBroker) nodeDisconnected(nodeID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for sub := range lb.subscriptionsByNode[nodeID] {
		sub.Done(nodeID, fmt.Errorf("node %s disconnected unexpectedly", nodeID))
	}
	delete(lb.subscriptionsByNode, nodeID)
}

// ListenSubscriptions returns a stream of matching subscriptions for the current node
func (lb *LogBroker) ListenSubscriptions(request *api.ListenSubscriptionsRequest, stream api.LogBroker_ListenSubscriptionsServer) error {
	remote, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}

	lb.mu.Lock()
	pctx := lb.pctx
	lb.mu.Unlock()
	if pctx == nil {
		return errNotRunning
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

	// Start by sending down all active subscriptions.
	for _, subscription := range subscriptions {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-pctx.Done():
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
				delete(activeSubscriptions, subscription.message.ID)
			} else {
				// Avoid sending down the same subscription multiple times
				if _, ok := activeSubscriptions[subscription.message.ID]; ok {
					continue
				}
				activeSubscriptions[subscription.message.ID] = subscription
			}
			if err := stream.Send(subscription.message); err != nil {
				log.Error(err)
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-pctx.Done():
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
			lb.markDone(currentSubscription, remote.NodeID, err)
		}
	}()

	for {
		logMsg, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&api.PublishLogsResponse{})
		}
		if err != nil {
			return err
		}

		if logMsg.SubscriptionID == "" {
			return status.Errorf(codes.InvalidArgument, "missing subscription ID")
		}

		if currentSubscription == nil {
			currentSubscription = lb.getSubscription(logMsg.SubscriptionID)
			if currentSubscription == nil {
				return status.Errorf(codes.NotFound, "unknown subscription ID")
			}
		} else {
			if logMsg.SubscriptionID != currentSubscription.message.ID {
				return status.Errorf(codes.InvalidArgument, "different subscription IDs in the same session")
			}
		}

		// if we have a close message, close out the subscription
		if logMsg.Close {
			// Mark done and then set to nil so if we error after this point,
			// we don't try to close again in the defer
			lb.markDone(currentSubscription, remote.NodeID, err)
			currentSubscription = nil
			return nil
		}

		// Make sure logs are emitted using the right Node ID to avoid impersonation.
		for _, msg := range logMsg.Messages {
			if msg.Context.NodeID != remote.NodeID {
				return status.Errorf(codes.PermissionDenied, "invalid NodeID: expected=%s;received=%s", remote.NodeID, msg.Context.NodeID)
			}
		}

		lb.publish(logMsg)
	}
}
