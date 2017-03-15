package formatter

import "github.com/docker/go-connections/nat"

// PortWrite writes the context
func PortWrite(ctx Context, frontends []nat.PortBinding) error {
	render := func(format func(subContext subContext) error) error {
		for _, frontend := range frontends {
			pCtx := &portContext{PortBinding: frontend}
			if err := format(pCtx); err != nil {
				return err
			}
		}
		return nil
	}
	subCtx := &portContext{}
	return ctx.Write(subCtx, render)
}

type portContext struct {
	HeaderContext
	nat.PortBinding
}

func (p *portContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(p)
}
