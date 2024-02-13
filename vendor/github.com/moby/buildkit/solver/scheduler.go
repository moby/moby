package solver

import (
	"context"
	"os"
	"sync"

	"github.com/moby/buildkit/solver/internal/pipe"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/cond"
	"github.com/pkg/errors"
)

var debugScheduler = false // TODO: replace with logs in build trace

func init() {
	if os.Getenv("BUILDKIT_SCHEDULER_DEBUG") == "1" {
		debugScheduler = true
	}
}

func newScheduler(ef edgeFactory) *scheduler {
	s := &scheduler{
		waitq:    map[*edge]struct{}{},
		incoming: map[*edge][]*edgePipe{},
		outgoing: map[*edge][]*edgePipe{},

		stopped: make(chan struct{}),
		closed:  make(chan struct{}),

		ef: ef,
	}
	s.cond = cond.NewStatefulCond(&s.mu)

	go s.loop()

	return s
}

type dispatcher struct {
	next *dispatcher
	e    *edge
}

type scheduler struct {
	cond *cond.StatefulCond
	mu   sync.Mutex
	muQ  sync.Mutex

	ef edgeFactory

	waitq       map[*edge]struct{}
	next        *dispatcher
	last        *dispatcher
	stopped     chan struct{}
	stoppedOnce sync.Once
	closed      chan struct{}

	incoming map[*edge][]*edgePipe
	outgoing map[*edge][]*edgePipe
}

func (s *scheduler) Stop() {
	s.stoppedOnce.Do(func() {
		close(s.stopped)
	})
	<-s.closed
}

func (s *scheduler) loop() {
	defer func() {
		close(s.closed)
	}()

	go func() {
		<-s.stopped
		s.mu.Lock()
		s.cond.Signal()
		s.mu.Unlock()
	}()

	s.mu.Lock()
	for {
		select {
		case <-s.stopped:
			s.mu.Unlock()
			return
		default:
		}
		s.muQ.Lock()
		l := s.next
		if l != nil {
			if l == s.last {
				s.last = nil
			}
			s.next = l.next
			delete(s.waitq, l.e)
		}
		s.muQ.Unlock()
		if l == nil {
			s.cond.Wait()
			continue
		}
		s.dispatch(l.e)
	}
}

// dispatch schedules an edge to be processed
func (s *scheduler) dispatch(e *edge) {
	inc := make([]pipe.Sender, len(s.incoming[e]))
	for i, p := range s.incoming[e] {
		inc[i] = p.Sender
	}
	out := make([]pipe.Receiver, len(s.outgoing[e]))
	for i, p := range s.outgoing[e] {
		out[i] = p.Receiver
	}

	e.hasActiveOutgoing = false
	updates := []pipe.Receiver{}
	for _, p := range out {
		if ok := p.Receive(); ok {
			updates = append(updates, p)
		}
		if !p.Status().Completed {
			e.hasActiveOutgoing = true
		}
	}

	pf := &pipeFactory{s: s, e: e}

	// unpark the edge
	if debugScheduler {
		debugSchedulerPreUnpark(e, inc, updates, out)
	}
	e.unpark(inc, updates, out, pf)
	if debugScheduler {
		debugSchedulerPostUnpark(e, inc)
	}

	// set up new requests that didn't complete/were added by this run
	openIncoming := make([]*edgePipe, 0, len(inc))
	for _, r := range s.incoming[e] {
		if !r.Sender.Status().Completed {
			openIncoming = append(openIncoming, r)
		}
	}
	if len(openIncoming) > 0 {
		s.incoming[e] = openIncoming
	} else {
		delete(s.incoming, e)
	}

	openOutgoing := make([]*edgePipe, 0, len(out))
	for _, r := range s.outgoing[e] {
		if !r.Receiver.Status().Completed {
			openOutgoing = append(openOutgoing, r)
		}
	}
	if len(openOutgoing) > 0 {
		s.outgoing[e] = openOutgoing
	} else {
		delete(s.outgoing, e)
	}

	// if keys changed there might be possiblity for merge with other edge
	if e.keysDidChange {
		if k := e.currentIndexKey(); k != nil {
			// skip this if not at least 1 key per dep
			origEdge := e.index.LoadOrStore(k, e)
			if origEdge != nil {
				if e.isDep(origEdge) || origEdge.isDep(e) {
					bklog.G(context.TODO()).Debugf("skip merge due to dependency")
				} else {
					bklog.G(context.TODO()).Debugf("merging edge %s to %s\n", e.edge.Vertex.Name(), origEdge.edge.Vertex.Name())
					if s.mergeTo(origEdge, e) {
						s.ef.setEdge(e.edge, origEdge)
					}
				}
			}
		}
		e.keysDidChange = false
	}

	// validation to avoid deadlocks/resource leaks:
	// TODO: if these start showing up in error reports they can be changed
	// to error the edge instead. They can only appear from algorithm bugs in
	// unpark(), not for any external input.
	if len(openIncoming) > 0 && len(openOutgoing) == 0 {
		e.markFailed(pf, errors.New("buildkit scheduler error: return leaving incoming open. Please report this with BUILDKIT_SCHEDULER_DEBUG=1"))
	}
	if len(openIncoming) == 0 && len(openOutgoing) > 0 {
		e.markFailed(pf, errors.New("buildkit scheduler error: return leaving outgoing open. Please report this with BUILDKIT_SCHEDULER_DEBUG=1"))
	}
}

// signal notifies that an edge needs to be processed again
func (s *scheduler) signal(e *edge) {
	s.muQ.Lock()
	if _, ok := s.waitq[e]; !ok {
		d := &dispatcher{e: e}
		if s.last == nil {
			s.next = d
		} else {
			s.last.next = d
		}
		s.last = d
		s.waitq[e] = struct{}{}
		s.cond.Signal()
	}
	s.muQ.Unlock()
}

// build evaluates edge into a result
func (s *scheduler) build(ctx context.Context, edge Edge) (CachedResult, error) {
	s.mu.Lock()
	e := s.ef.getEdge(edge)
	if e == nil {
		s.mu.Unlock()
		return nil, errors.Errorf("invalid request %v for build", edge)
	}

	wait := make(chan struct{})

	p := s.newPipe(e, nil, pipe.Request{Payload: &edgeRequest{desiredState: edgeStatusComplete}})
	p.OnSendCompletion = func() {
		p.Receiver.Receive()
		if p.Receiver.Status().Completed {
			close(wait)
		}
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		p.Receiver.Cancel()
	}()

	<-wait

	if err := p.Receiver.Status().Err; err != nil {
		return nil, err
	}
	return p.Receiver.Status().Value.(*edgeState).result.CloneCachedResult(), nil
}

// newPipe creates a new request pipe between two edges
func (s *scheduler) newPipe(target, from *edge, req pipe.Request) *pipe.Pipe {
	p := &edgePipe{
		Pipe:   pipe.New(req),
		Target: target,
		From:   from,
	}

	s.signal(target)
	if from != nil {
		p.OnSendCompletion = func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			s.signal(p.From)
		}
		s.outgoing[from] = append(s.outgoing[from], p)
	}
	s.incoming[target] = append(s.incoming[target], p)
	p.OnReceiveCompletion = func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		s.signal(p.Target)
	}
	return p.Pipe
}

// newRequestWithFunc creates a new request pipe that invokes a async function
func (s *scheduler) newRequestWithFunc(e *edge, f func(context.Context) (interface{}, error)) pipe.Receiver {
	pp, start := pipe.NewWithFunction(f)
	p := &edgePipe{
		Pipe: pp,
		From: e,
	}
	p.OnSendCompletion = func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		s.signal(p.From)
	}
	s.outgoing[e] = append(s.outgoing[e], p)
	go start()
	return p.Receiver
}

// mergeTo merges the state from one edge to another. source edge is discarded.
func (s *scheduler) mergeTo(target, src *edge) bool {
	if !target.edge.Vertex.Options().IgnoreCache && src.edge.Vertex.Options().IgnoreCache {
		return false
	}
	for _, inc := range s.incoming[src] {
		inc.mu.Lock()
		inc.Target = target
		s.incoming[target] = append(s.incoming[target], inc)
		inc.mu.Unlock()
	}

	for _, out := range s.outgoing[src] {
		out.mu.Lock()
		out.From = target
		s.outgoing[target] = append(s.outgoing[target], out)
		out.mu.Unlock()
		out.Receiver.Cancel()
	}

	delete(s.incoming, src)
	delete(s.outgoing, src)
	s.signal(target)

	for i, d := range src.deps {
		for _, k := range d.keys {
			target.secondaryExporters = append(target.secondaryExporters, expDep{i, CacheKeyWithSelector{CacheKey: k, Selector: src.cacheMap.Deps[i].Selector}})
		}
		if d.slowCacheKey != nil {
			target.secondaryExporters = append(target.secondaryExporters, expDep{i, CacheKeyWithSelector{CacheKey: *d.slowCacheKey}})
		}
		if d.result != nil {
			for _, dk := range d.result.CacheKeys() {
				target.secondaryExporters = append(target.secondaryExporters, expDep{i, CacheKeyWithSelector{CacheKey: dk, Selector: src.cacheMap.Deps[i].Selector}})
			}
		}
	}

	// TODO(tonistiigi): merge cache providers

	return true
}

// edgeFactory allows access to the edges from a shared graph
type edgeFactory interface {
	getEdge(Edge) *edge
	setEdge(Edge, *edge)
}

type pipeFactory struct {
	e *edge
	s *scheduler
}

func (pf *pipeFactory) NewInputRequest(ee Edge, req *edgeRequest) pipe.Receiver {
	target := pf.s.ef.getEdge(ee)
	if target == nil {
		return pf.NewFuncRequest(func(_ context.Context) (interface{}, error) {
			return nil, errors.Errorf("failed to get edge: inconsistent graph state")
		})
	}
	p := pf.s.newPipe(target, pf.e, pipe.Request{Payload: req})
	if debugScheduler {
		bklog.G(context.TODO()).Debugf("> newPipe %s %p desiredState=%s", ee.Vertex.Name(), p, req.desiredState)
	}
	return p.Receiver
}

func (pf *pipeFactory) NewFuncRequest(f func(context.Context) (interface{}, error)) pipe.Receiver {
	p := pf.s.newRequestWithFunc(pf.e, f)
	if debugScheduler {
		bklog.G(context.TODO()).Debugf("> newFunc %p", p)
	}
	return p
}

func debugSchedulerPreUnpark(e *edge, inc []pipe.Sender, updates, allPipes []pipe.Receiver) {
	log := bklog.G(context.TODO())

	log.Debugf(">> unpark %s req=%d upt=%d out=%d state=%s %s", e.edge.Vertex.Name(), len(inc), len(updates), len(allPipes), e.state, e.edge.Vertex.Digest())

	for i, dep := range e.deps {
		des := edgeStatusInitial
		if dep.req != nil {
			des = dep.req.Request().(*edgeRequest).desiredState
		}
		log.Debugf(":: dep%d %s state=%s des=%s keys=%d hasslowcache=%v preprocessfunc=%v", i, e.edge.Vertex.Inputs()[i].Vertex.Name(), dep.state, des, len(dep.keys), e.slowCacheFunc(dep) != nil, e.preprocessFunc(dep) != nil)
	}

	for i, in := range inc {
		req := in.Request()
		log.Debugf("> incoming-%d: %p dstate=%s canceled=%v", i, in, req.Payload.(*edgeRequest).desiredState, req.Canceled)
	}

	for i, up := range updates {
		if up == e.cacheMapReq {
			log.Debugf("> update-%d: %p cacheMapReq complete=%v", i, up, up.Status().Completed)
		} else if up == e.execReq {
			log.Debugf("> update-%d: %p execReq complete=%v", i, up, up.Status().Completed)
		} else {
			st, ok := up.Status().Value.(*edgeState)
			if ok {
				index := -1
				if dep, ok := e.depRequests[up]; ok {
					index = int(dep.index)
				}
				log.Debugf("> update-%d: %p input-%d keys=%d state=%s", i, up, index, len(st.keys), st.state)
			} else {
				log.Debugf("> update-%d: unknown", i)
			}
		}
	}
}

func debugSchedulerPostUnpark(e *edge, inc []pipe.Sender) {
	log := bklog.G(context.TODO())
	for i, in := range inc {
		log.Debugf("< incoming-%d: %p completed=%v", i, in, in.Status().Completed)
	}
	log.Debugf("<< unpark %s\n", e.edge.Vertex.Name())
}
