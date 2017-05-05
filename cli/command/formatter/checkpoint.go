package formatter

import "github.com/docker/docker/api/types"

const (
	defaultCheckpointFormat = "table {{.Name}}"

	checkpointNameHeader = "CHECKPOINT NAME"
)

// NewCheckpointFormat returns a format for use with a checkpoint Context
func NewCheckpointFormat(source string) Format {
	switch source {
	case TableFormatKey:
		return defaultCheckpointFormat
	}
	return Format(source)
}

// CheckpointWrite writes formatted checkpoints using the Context
func CheckpointWrite(ctx Context, checkpoints []types.Checkpoint) error {
	render := func(format func(subContext subContext) error) error {
		for _, checkpoint := range checkpoints {
			if err := format(&checkpointContext{c: checkpoint}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(newCheckpointContext(), render)
}

type checkpointContext struct {
	HeaderContext
	c types.Checkpoint
}

func newCheckpointContext() *checkpointContext {
	cpCtx := checkpointContext{}
	cpCtx.header = volumeHeaderContext{
		"Name": checkpointNameHeader,
	}
	return &cpCtx
}

func (c *checkpointContext) MarshalJSON() ([]byte, error) {
	return marshalJSON(c)
}

func (c *checkpointContext) Name() string {
	return c.c.Name
}
