package nodes

import (
	"testing"

	"github.com/docker/swarm/discovery"
	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.1:1111,2.2.2.2:2222", 0, 0)
	assert.Equal(t, len(d.entries), 2)
	assert.Equal(t, d.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(t, d.entries[1].String(), "2.2.2.2:2222")
}

func TestInitializeWithPattern(t *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.[1:2]:1111,2.2.2.[2:4]:2222", 0, 0)
	assert.Equal(t, len(d.entries), 5)
	assert.Equal(t, d.entries[0].String(), "1.1.1.1:1111")
	assert.Equal(t, d.entries[1].String(), "1.1.1.2:1111")
	assert.Equal(t, d.entries[2].String(), "2.2.2.2:2222")
	assert.Equal(t, d.entries[3].String(), "2.2.2.3:2222")
	assert.Equal(t, d.entries[4].String(), "2.2.2.4:2222")
}

func TestWatch(t *testing.T) {
	d := &Discovery{}
	d.Initialize("1.1.1.1:1111,2.2.2.2:2222", 0, 0)
	expected := discovery.Entries{
		&discovery.Entry{Host: "1.1.1.1", Port: "1111"},
		&discovery.Entry{Host: "2.2.2.2", Port: "2222"},
	}
	ch, _ := d.Watch(nil)
	assert.True(t, expected.Equals(<-ch))
}

func TestRegister(t *testing.T) {
	d := &Discovery{}
	assert.Error(t, d.Register("0.0.0.0"))
}
