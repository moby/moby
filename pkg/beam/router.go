package beam

import (
	"fmt"
	"github.com/dotcloud/docker/pkg/beam/data"
	"io"
	"os"
)

type Router struct {
	routes []*Route
	sink   Sender
}

func NewRouter(sink Sender) *Router {
	return &Router{sink: sink}
}

func (r *Router) Send(payload []byte, attachment *os.File) (err error) {
	//fmt.Printf("Router.Send(%s)\n", MsgDesc(payload, attachment))
	defer func() {
		//fmt.Printf("DONE Router.Send(%s) = %v\n", MsgDesc(payload, attachment), err)
	}()
	for _, route := range r.routes {
		if route.Match(payload, attachment) {
			return route.Handle(payload, attachment)
		}
	}
	if r.sink != nil {
		// fmt.Printf("[%d] [Router.Send] no match. sending %s to sink %#v\n", os.Getpid(), MsgDesc(payload, attachment), r.sink)
		return r.sink.Send(payload, attachment)
	}
	//fmt.Printf("[Router.Send] no match. return error.\n")
	return fmt.Errorf("no matching route")
}

func (r *Router) NewRoute() *Route {
	route := &Route{}
	r.routes = append(r.routes, route)
	return route
}

type Route struct {
	rules   []func([]byte, *os.File) bool
	handler func([]byte, *os.File) error
}

func (route *Route) Match(payload []byte, attachment *os.File) bool {
	for _, rule := range route.rules {
		if !rule(payload, attachment) {
			return false
		}
	}
	return true
}

func (route *Route) Handle(payload []byte, attachment *os.File) error {
	if route.handler == nil {
		return nil
	}
	return route.handler(payload, attachment)
}

func (r *Route) HasAttachment() *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		return attachment != nil
	})
	return r
}

func (route *Route) Tee(dst Sender) *Route {
	inner := route.handler
	route.handler = func(payload []byte, attachment *os.File) error {
		if inner == nil {
			return nil
		}
		if attachment == nil {
			return inner(payload, attachment)
		}
		// Setup the tee
		w, err := SendPipe(dst, payload)
		if err != nil {
			return err
		}
		teeR, teeW, err := os.Pipe()
		if err != nil {
			w.Close()
			return err
		}
		go func() {
			io.Copy(io.MultiWriter(teeW, w), attachment)
			attachment.Close()
			w.Close()
			teeW.Close()
		}()
		return inner(payload, teeR)
	}
	return route
}

func (r *Route) Filter(f func([]byte, *os.File) bool) *Route {
	r.rules = append(r.rules, f)
	return r
}

func (r *Route) KeyStartsWith(k string, beginning ...string) *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		values := data.Message(payload).Get(k)
		if values == nil {
			return false
		}
		if len(values) < len(beginning) {
			return false
		}
		for i, v := range beginning {
			if v != values[i] {
				return false
			}
		}
		return true
	})
	return r
}

func (r *Route) KeyEquals(k string, full ...string) *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		values := data.Message(payload).Get(k)
		if len(values) != len(full) {
			return false
		}
		for i, v := range full {
			if v != values[i] {
				return false
			}
		}
		return true
	})
	return r
}

func (r *Route) KeyIncludes(k, v string) *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		for _, val := range data.Message(payload).Get(k) {
			if val == v {
				return true
			}
		}
		return false
	})
	return r
}

func (r *Route) NoKey(k string) *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		return len(data.Message(payload).Get(k)) == 0
	})
	return r
}

func (r *Route) KeyExists(k string) *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		return data.Message(payload).Get(k) != nil
	})
	return r
}

func (r *Route) Passthrough(dst Sender) *Route {
	r.handler = func(payload []byte, attachment *os.File) error {
		return dst.Send(payload, attachment)
	}
	return r
}

func (r *Route) All() *Route {
	r.rules = append(r.rules, func(payload []byte, attachment *os.File) bool {
		return true
	})
	return r
}

func (r *Route) Handler(h func([]byte, *os.File) error) *Route {
	r.handler = h
	return r
}
