package allocator

import (
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/state"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"github.com/stretchr/testify/assert"
)

func (suite *testSuite) TestIPAMNotNil() {
	s := store.NewMemoryStore(nil)
	suite.NotNil(s)
	defer s.Close()

	a := suite.newAllocator(s)

	// Predefined node-local network
	p := &api.Network{
		ID: "one_unIque_id",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "pred_bridge_network",
				Labels: map[string]string{
					"com.docker.swarm.predefined": "true",
				},
			},
			DriverConfig: &api.Driver{Name: "bridge"},
		},
	}

	// Node-local swarm scope network
	nln := &api.Network{
		ID: "another_unIque_id",
		Spec: api.NetworkSpec{
			Annotations: api.Annotations{
				Name: "swarm-macvlan",
			},
			DriverConfig: &api.Driver{Name: "macvlan"},
		},
	}

	// Try adding some objects to store before allocator is started
	suite.NoError(s.Update(func(tx store.Tx) error {
		// populate ingress network
		in := &api.Network{
			ID: "ingress-nw-id",
			Spec: api.NetworkSpec{
				Annotations: api.Annotations{
					Name: "default-ingress",
				},
				Ingress: true,
			},
		}
		suite.NoError(store.CreateNetwork(tx, in))

		// Create the predefined node-local network with one service
		suite.NoError(store.CreateNetwork(tx, p))

		// Create the the swarm level node-local network with one service
		suite.NoError(store.CreateNetwork(tx, nln))

		return nil
	}))

	netWatch, cancel := state.Watch(s.WatchQueue(), api.EventUpdateNetwork{}, api.EventDeleteNetwork{})
	defer cancel()

	defer suite.startAllocator(a)()

	// Now verify if we get network and tasks updated properly
	watchNetwork(suite.T(), netWatch, false, func(t assert.TestingT, n *api.Network) bool { return true })
	watchNetwork(suite.T(), netWatch, false, func(t assert.TestingT, n *api.Network) bool { return true })
	watchNetwork(suite.T(), netWatch, false, func(t assert.TestingT, n *api.Network) bool { return true })

	// Verify no allocation was done for the node-local networks
	var (
		ps *api.Network
		sn *api.Network
	)
	s.View(func(tx store.ReadTx) {
		ps = store.GetNetwork(tx, p.ID)
		sn = store.GetNetwork(tx, nln.ID)

	})
	suite.NotNil(ps)
	suite.NotNil(sn)
	suite.NotNil(ps.IPAM)
	suite.NotNil(sn.IPAM)
}
