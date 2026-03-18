package multierror

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

func TestErrorJoin(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		err := Join(fmt.Errorf("invalid config: %w", Join(errors.New("foo"))))
		const expected = `invalid config: foo`
		assert.Equal(t, err.Error(), expected)
	})
	t.Run("multiple", func(t *testing.T) {
		err := Join(errors.New("foobar"), fmt.Errorf("invalid config: \n%w", Join(errors.New("foo"), errors.New("bar"))))
		const expected = `* foobar
* invalid config: 
	* foo
	* bar`
		assert.Equal(t, err.Error(), expected)
	})
}
