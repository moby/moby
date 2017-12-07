package transport

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/raft/membership"
	"github.com/pkg/errors"
)

const (
	// GRPCMaxMsgSize is the max allowed gRPC message size for raft messages.
	GRPCMaxMsgSize = 4 << 20
)

type peer struct {
	id uint64

	tr *Transport

	msgc chan raftpb.Message

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	cc      *grpc.ClientConn
	addr    string
	newAddr string

	active       bool
	becameActive time.Time
}

func newPeer(id uint64, addr string, tr *Transport) (*peer, error) {
	cc, err := tr.dial(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create conn for %x with addr %s", id, addr)
	}
	ctx, cancel := context.WithCancel(tr.ctx)
	ctx = log.WithField(ctx, "peer_id", fmt.Sprintf("%x", id))
	p := &peer{
		id:     id,
		addr:   addr,
		cc:     cc,
		tr:     tr,
		ctx:    ctx,
		cancel: cancel,
		msgc:   make(chan raftpb.Message, 4096),
		done:   make(chan struct{}),
	}
	go p.run(ctx)
	return p, nil
}

func (p *peer) send(m raftpb.Message) (err error) {
	p.mu.Lock()
	defer func() {
		if err != nil {
			p.active = false
			p.becameActive = time.Time{}
		}
		p.mu.Unlock()
	}()
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
	}
	select {
	case p.msgc <- m:
	case <-p.ctx.Done():
		return p.ctx.Err()
	default:
		p.tr.config.ReportUnreachable(p.id)
		return errors.Errorf("peer is unreachable")
	}
	return nil
}

func (p *peer) update(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.addr == addr {
		return nil
	}
	cc, err := p.tr.dial(addr)
	if err != nil {
		return err
	}

	p.cc.Close()
	p.cc = cc
	p.addr = addr
	return nil
}

func (p *peer) updateAddr(addr string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.addr == addr {
		return nil
	}
	log.G(p.ctx).Debugf("peer %x updated to address %s, it will be used if old failed", p.id, addr)
	p.newAddr = addr
	return nil
}

func (p *peer) conn() *grpc.ClientConn {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cc
}

func (p *peer) address() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

func (p *peer) resolveAddr(ctx context.Context, id uint64) (string, error) {
	resp, err := api.NewRaftClient(p.conn()).ResolveAddress(ctx, &api.ResolveAddressRequest{RaftID: id})
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve address")
	}
	return resp.Addr, nil
}

// Returns the raft message struct size (not including the payload size) for the given raftpb.Message.
// The payload is typically the snapshot or append entries.
func raftMessageStructSize(m *raftpb.Message) int {
	return (&api.ProcessRaftMessageRequest{Message: m}).Size() - len(m.Snapshot.Data)
}

// Returns the max allowable payload based on MaxRaftMsgSize and
// the struct size for the given raftpb.Message.
func raftMessagePayloadSize(m *raftpb.Message) int {
	return GRPCMaxMsgSize - raftMessageStructSize(m)
}

// Split a large raft message into smaller messages.
// Currently this means splitting the []Snapshot.Data into chunks whose size
// is dictacted by MaxRaftMsgSize.
func splitSnapshotData(ctx context.Context, m *raftpb.Message) []api.StreamRaftMessageRequest {
	var messages []api.StreamRaftMessageRequest
	if m.Type != raftpb.MsgSnap {
		return messages
	}

	// get the size of the data to be split.
	size := len(m.Snapshot.Data)

	// Get the max payload size.
	payloadSize := raftMessagePayloadSize(m)

	// split the snapshot into smaller messages.
	for snapDataIndex := 0; snapDataIndex < size; {
		chunkSize := size - snapDataIndex
		if chunkSize > payloadSize {
			chunkSize = payloadSize
		}

		raftMsg := *m

		// sub-slice for this snapshot chunk.
		raftMsg.Snapshot.Data = m.Snapshot.Data[snapDataIndex : snapDataIndex+chunkSize]

		snapDataIndex += chunkSize

		// add message to the list of messages to be sent.
		msg := api.StreamRaftMessageRequest{Message: &raftMsg}
		messages = append(messages, msg)
	}

	return messages
}

// Function to check if this message needs to be split to be streamed
// (because it is larger than GRPCMaxMsgSize).
// Returns true if the message type is MsgSnap
// and size larger than MaxRaftMsgSize.
func needsSplitting(m *raftpb.Message) bool {
	raftMsg := api.ProcessRaftMessageRequest{Message: m}
	return m.Type == raftpb.MsgSnap && raftMsg.Size() > GRPCMaxMsgSize
}

func (p *peer) sendProcessMessage(ctx context.Context, m raftpb.Message) error {
	ctx, cancel := context.WithTimeout(ctx, p.tr.config.SendTimeout)
	defer cancel()

	var err error
	var stream api.Raft_StreamRaftMessageClient
	stream, err = api.NewRaftClient(p.conn()).StreamRaftMessage(ctx)

	if err == nil {
		// Split the message if needed.
		// Currently only supported for MsgSnap.
		var msgs []api.StreamRaftMessageRequest
		if needsSplitting(&m) {
			msgs = splitSnapshotData(ctx, &m)
		} else {
			raftMsg := api.StreamRaftMessageRequest{Message: &m}
			msgs = append(msgs, raftMsg)
		}

		// Stream
		for _, msg := range msgs {
			err = stream.Send(&msg)
			if err != nil {
				log.G(ctx).WithError(err).Error("error streaming message to peer")
				stream.CloseAndRecv()
				break
			}
		}

		// Finished sending all the messages.
		// Close and receive response.
		if err == nil {
			_, err = stream.CloseAndRecv()

			if err != nil {
				log.G(ctx).WithError(err).Error("error receiving response")
			}
		}
	} else {
		log.G(ctx).WithError(err).Error("error sending message to peer")
	}

	// Try doing a regular rpc if the receiver doesn't support streaming.
	if grpc.Code(err) == codes.Unimplemented {
		_, err = api.NewRaftClient(p.conn()).ProcessRaftMessage(ctx, &api.ProcessRaftMessageRequest{Message: &m})
	}

	// Handle errors.
	if grpc.Code(err) == codes.NotFound && grpc.ErrorDesc(err) == membership.ErrMemberRemoved.Error() {
		p.tr.config.NodeRemoved()
	}
	if m.Type == raftpb.MsgSnap {
		if err != nil {
			p.tr.config.ReportSnapshot(m.To, raft.SnapshotFailure)
		} else {
			p.tr.config.ReportSnapshot(m.To, raft.SnapshotFinish)
		}
	}
	if err != nil {
		p.tr.config.ReportUnreachable(m.To)
		return err
	}
	return nil
}

func healthCheckConn(ctx context.Context, cc *grpc.ClientConn) error {
	resp, err := api.NewHealthClient(cc).Check(ctx, &api.HealthCheckRequest{Service: "Raft"})
	if err != nil {
		return errors.Wrap(err, "failed to check health")
	}
	if resp.Status != api.HealthCheckResponse_SERVING {
		return errors.Errorf("health check returned status %s", resp.Status)
	}
	return nil
}

func (p *peer) healthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, p.tr.config.SendTimeout)
	defer cancel()
	return healthCheckConn(ctx, p.conn())
}

func (p *peer) setActive() {
	p.mu.Lock()
	if !p.active {
		p.active = true
		p.becameActive = time.Now()
	}
	p.mu.Unlock()
}

func (p *peer) setInactive() {
	p.mu.Lock()
	p.active = false
	p.becameActive = time.Time{}
	p.mu.Unlock()
}

func (p *peer) activeTime() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.becameActive
}

func (p *peer) drain() error {
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
	defer cancel()
	for {
		select {
		case m, ok := <-p.msgc:
			if !ok {
				// all messages proceeded
				return nil
			}
			if err := p.sendProcessMessage(ctx, m); err != nil {
				return errors.Wrap(err, "send drain message")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *peer) handleAddressChange(ctx context.Context) error {
	p.mu.Lock()
	newAddr := p.newAddr
	p.newAddr = ""
	p.mu.Unlock()
	if newAddr == "" {
		return nil
	}
	cc, err := p.tr.dial(newAddr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, p.tr.config.SendTimeout)
	defer cancel()
	if err := healthCheckConn(ctx, cc); err != nil {
		cc.Close()
		return err
	}
	// there is possibility of race if host changing address too fast, but
	// it's unlikely and eventually thing should be settled
	p.mu.Lock()
	p.cc.Close()
	p.cc = cc
	p.addr = newAddr
	p.tr.config.UpdateNode(p.id, p.addr)
	p.mu.Unlock()
	return nil
}

func (p *peer) run(ctx context.Context) {
	defer func() {
		p.mu.Lock()
		p.active = false
		p.becameActive = time.Time{}
		// at this point we can be sure that nobody will write to msgc
		if p.msgc != nil {
			close(p.msgc)
		}
		p.mu.Unlock()
		if err := p.drain(); err != nil {
			log.G(ctx).WithError(err).Error("failed to drain message queue")
		}
		close(p.done)
	}()
	if err := p.healthCheck(ctx); err == nil {
		p.setActive()
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case m := <-p.msgc:
			// we do not propagate context here, because this operation should be finished
			// or timed out for correct raft work.
			err := p.sendProcessMessage(context.Background(), m)
			if err != nil {
				log.G(ctx).WithError(err).Debugf("failed to send message %s", m.Type)
				p.setInactive()
				if err := p.handleAddressChange(ctx); err != nil {
					log.G(ctx).WithError(err).Error("failed to change address after failure")
				}
				continue
			}
			p.setActive()
		case <-ctx.Done():
			return
		}
	}
}

func (p *peer) stop() {
	p.cancel()
	<-p.done
}
