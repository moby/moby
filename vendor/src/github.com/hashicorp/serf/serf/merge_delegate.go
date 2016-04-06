package serf

import (
	"net"

	"github.com/hashicorp/memberlist"
)

type MergeDelegate interface {
	NotifyMerge([]*Member) (cancel bool)
}

type mergeDelegate struct {
	serf *Serf
}

func (m *mergeDelegate) NotifyMerge(nodes []*memberlist.Node) (cancel bool) {
	members := make([]*Member, len(nodes))
	for idx, n := range nodes {
		members[idx] = &Member{
			Name:        n.Name,
			Addr:        net.IP(n.Addr),
			Port:        n.Port,
			Tags:        m.serf.decodeTags(n.Meta),
			Status:      StatusNone,
			ProtocolMin: n.PMin,
			ProtocolMax: n.PMax,
			ProtocolCur: n.PCur,
			DelegateMin: n.DMin,
			DelegateMax: n.DMax,
			DelegateCur: n.DCur,
		}
	}
	return m.serf.config.Merge.NotifyMerge(members)
}
