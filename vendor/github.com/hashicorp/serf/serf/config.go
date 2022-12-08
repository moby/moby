package serf

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
)

// ProtocolVersionMap is the mapping of Serf delegate protocol versions
// to memberlist protocol versions. We mask the memberlist protocols using
// our own protocol version.
var ProtocolVersionMap map[uint8]uint8

func init() {
	ProtocolVersionMap = map[uint8]uint8{
		5: 2,
		4: 2,
		3: 2,
		2: 2,
	}
}

// Config is the configuration for creating a Serf instance.
type Config struct {
	// The name of this node. This must be unique in the cluster. If this
	// is not set, Serf will set it to the hostname of the running machine.
	NodeName string

	// The tags for this role, if any. This is used to provide arbitrary
	// key/value metadata per-node. For example, a "role" tag may be used to
	// differentiate "load-balancer" from a "web" role as parts of the same cluster.
	// Tags are deprecating 'Role', and instead it acts as a special key in this
	// map.
	Tags map[string]string

	// EventCh is a channel that receives all the Serf events. The events
	// are sent on this channel in proper ordering. Care must be taken that
	// this channel doesn't block, either by processing the events quick
	// enough or buffering the channel, otherwise it can block state updates
	// within Serf itself. If no EventCh is specified, no events will be fired,
	// but point-in-time snapshots of members can still be retrieved by
	// calling Members on Serf.
	EventCh chan<- Event

	// ProtocolVersion is the protocol version to speak. This must be between
	// ProtocolVersionMin and ProtocolVersionMax.
	ProtocolVersion uint8

	// BroadcastTimeout is the amount of time to wait for a broadcast
	// message to be sent to the cluster. Broadcast messages are used for
	// things like leave messages and force remove messages. If this is not
	// set, a timeout of 5 seconds will be set.
	BroadcastTimeout time.Duration

	// LeavePropagateDelay is for our leave (node dead) message to propagate
	// through the cluster. In particular, we want to stay up long enough to
	// service any probes from other nodes before they learn about us
	// leaving and stop probing. Otherwise, we risk getting node failures as
	// we leave.
	LeavePropagateDelay time.Duration

	// The settings below relate to Serf's event coalescence feature. Serf
	// is able to coalesce multiple events into single events in order to
	// reduce the amount of noise that is sent along the EventCh. For example
	// if five nodes quickly join, the EventCh will be sent one EventMemberJoin
	// containing the five nodes rather than five individual EventMemberJoin
	// events. Coalescence can mitigate potential flapping behavior.
	//
	// Coalescence is disabled by default and can be enabled by setting
	// CoalescePeriod.
	//
	// CoalescePeriod specifies the time duration to coalesce events.
	// For example, if this is set to 5 seconds, then all events received
	// within 5 seconds that can be coalesced will be.
	//
	// QuiescentPeriod specifies the duration of time where if no events
	// are received, coalescence immediately happens. For example, if
	// CoalscePeriod is set to 10 seconds but QuiscentPeriod is set to 2
	// seconds, then the events will be coalesced and dispatched if no
	// new events are received within 2 seconds of the last event. Otherwise,
	// every event will always be delayed by at least 10 seconds.
	CoalescePeriod  time.Duration
	QuiescentPeriod time.Duration

	// The settings below relate to Serf's user event coalescing feature.
	// The settings operate like above but only affect user messages and
	// not the Member* messages that Serf generates.
	UserCoalescePeriod  time.Duration
	UserQuiescentPeriod time.Duration

	// The settings below relate to Serf keeping track of recently
	// failed/left nodes and attempting reconnects.
	//
	// ReapInterval is the interval when the reaper runs. If this is not
	// set (it is zero), it will be set to a reasonable default.
	//
	// ReconnectInterval is the interval when we attempt to reconnect
	// to failed nodes. If this is not set (it is zero), it will be set
	// to a reasonable default.
	//
	// ReconnectTimeout is the amount of time to attempt to reconnect to
	// a failed node before giving up and considering it completely gone.
	//
	// TombstoneTimeout is the amount of time to keep around nodes
	// that gracefully left as tombstones for syncing state with other
	// Serf nodes.
	ReapInterval      time.Duration
	ReconnectInterval time.Duration
	ReconnectTimeout  time.Duration
	TombstoneTimeout  time.Duration

	// FlapTimeout is the amount of time less than which we consider a node
	// being failed and rejoining looks like a flap for telemetry purposes.
	// This should be set less than a typical reboot time, but large enough
	// to see actual events, given our expected detection times for a failed
	// node.
	FlapTimeout time.Duration

	// QueueCheckInterval is the interval at which we check the message
	// queue to apply the warning and max depth.
	QueueCheckInterval time.Duration

	// QueueDepthWarning is used to generate warning message if the
	// number of queued messages to broadcast exceeds this number. This
	// is to provide the user feedback if events are being triggered
	// faster than they can be disseminated
	QueueDepthWarning int

	// MaxQueueDepth is used to start dropping messages if the number
	// of queued messages to broadcast exceeds this number. This is to
	// prevent an unbounded growth of memory utilization
	MaxQueueDepth int

	// MinQueueDepth, if >0 will enforce a lower limit for dropping messages
	// and then the max will be max(MinQueueDepth, 2*SizeOfCluster). This
	// defaults to 0 which disables this dynamic sizing feature. If this is
	// >0 then MaxQueueDepth will be ignored.
	MinQueueDepth int

	// RecentIntentTimeout is used to determine how long we store recent
	// join and leave intents. This is used to guard against the case where
	// Serf broadcasts an intent that arrives before the Memberlist event.
	// It is important that this not be too short to avoid continuous
	// rebroadcasting of dead events.
	RecentIntentTimeout time.Duration

	// EventBuffer is used to control how many events are buffered.
	// This is used to prevent re-delivery of events to a client. The buffer
	// must be large enough to handle all "recent" events, since Serf will
	// not deliver messages that are older than the oldest entry in the buffer.
	// Thus if a client is generating too many events, it's possible that the
	// buffer gets overrun and messages are not delivered.
	EventBuffer int

	// QueryBuffer is used to control how many queries are buffered.
	// This is used to prevent re-delivery of queries to a client. The buffer
	// must be large enough to handle all "recent" events, since Serf will not
	// deliver queries older than the oldest entry in the buffer.
	// Thus if a client is generating too many queries, it's possible that the
	// buffer gets overrun and messages are not delivered.
	QueryBuffer int

	// QueryTimeoutMult configures the default timeout multipler for a query to run if no
	// specific value is provided. Queries are real-time by nature, where the
	// reply is time sensitive. As a result, results are collected in an async
	// fashion, however the query must have a bounded duration. We want the timeout
	// to be long enough that all nodes have time to receive the message, run a handler,
	// and generate a reply. Once the timeout is exceeded, any further replies are ignored.
	// The default value is
	//
	// Timeout = GossipInterval * QueryTimeoutMult * log(N+1)
	//
	QueryTimeoutMult int

	// QueryResponseSizeLimit and QuerySizeLimit limit the inbound and
	// outbound payload sizes for queries, respectively. These must fit
	// in a UDP packet with some additional overhead, so tuning these
	// past the default values of 1024 will depend on your network
	// configuration.
	QueryResponseSizeLimit int
	QuerySizeLimit         int

	// MemberlistConfig is the memberlist configuration that Serf will
	// use to do the underlying membership management and gossip. Some
	// fields in the MemberlistConfig will be overwritten by Serf no
	// matter what:
	//
	//   * Name - This will always be set to the same as the NodeName
	//     in this configuration.
	//
	//   * Events - Serf uses a custom event delegate.
	//
	//   * Delegate - Serf uses a custom delegate.
	//
	MemberlistConfig *memberlist.Config

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// Logger is a custom logger which you provide. If Logger is set, it will use
	// this for the internal logger. If Logger is not set, it will fall back to the
	// behavior for using LogOutput. You cannot specify both LogOutput and Logger
	// at the same time.
	Logger *log.Logger

	// SnapshotPath if provided is used to snapshot live nodes as well
	// as lamport clock values. When Serf is started with a snapshot,
	// it will attempt to join all the previously known nodes until one
	// succeeds and will also avoid replaying old user events.
	SnapshotPath string

	// RejoinAfterLeave controls our interaction with the snapshot file.
	// When set to false (default), a leave causes a Serf to not rejoin
	// the cluster until an explicit join is received. If this is set to
	// true, we ignore the leave, and rejoin the cluster on start.
	RejoinAfterLeave bool

	// EnableNameConflictResolution controls if Serf will actively attempt
	// to resolve a name conflict. Since each Serf member must have a unique
	// name, a cluster can run into issues if multiple nodes claim the same
	// name. Without automatic resolution, Serf merely logs some warnings, but
	// otherwise does not take any action. Automatic resolution detects the
	// conflict and issues a special query which asks the cluster for the
	// Name -> IP:Port mapping. If there is a simple majority of votes, that
	// node stays while the other node will leave the cluster and exit.
	EnableNameConflictResolution bool

	// DisableCoordinates controls if Serf will maintain an estimate of this
	// node's network coordinate internally. A network coordinate is useful
	// for estimating the network distance (i.e. round trip time) between
	// two nodes. Enabling this option adds some overhead to ping messages.
	DisableCoordinates bool

	// KeyringFile provides the location of a writable file where Serf can
	// persist changes to the encryption keyring.
	KeyringFile string

	// Merge can be optionally provided to intercept a cluster merge
	// and conditionally abort the merge.
	Merge MergeDelegate
}

// Init allocates the subdata structures
func (c *Config) Init() {
	if c.Tags == nil {
		c.Tags = make(map[string]string)
	}
}

// DefaultConfig returns a Config struct that contains reasonable defaults
// for most of the configurations.
func DefaultConfig() *Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return &Config{
		NodeName:                     hostname,
		BroadcastTimeout:             5 * time.Second,
		LeavePropagateDelay:          1 * time.Second,
		EventBuffer:                  512,
		QueryBuffer:                  512,
		LogOutput:                    os.Stderr,
		ProtocolVersion:              4,
		ReapInterval:                 15 * time.Second,
		RecentIntentTimeout:          5 * time.Minute,
		ReconnectInterval:            30 * time.Second,
		ReconnectTimeout:             24 * time.Hour,
		QueueCheckInterval:           30 * time.Second,
		QueueDepthWarning:            128,
		MaxQueueDepth:                4096,
		TombstoneTimeout:             24 * time.Hour,
		FlapTimeout:                  60 * time.Second,
		MemberlistConfig:             memberlist.DefaultLANConfig(),
		QueryTimeoutMult:             16,
		QueryResponseSizeLimit:       1024,
		QuerySizeLimit:               1024,
		EnableNameConflictResolution: true,
		DisableCoordinates:           false,
	}
}
