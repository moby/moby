package netlink

import (
	"syscall"

	"github.com/vishvananda/netlink/nl"
)

// ClassDel will delete a class from the system.
// Equivalent to: `tc class del $class`
func ClassDel(class Class) error {
	req := nl.NewNetlinkRequest(syscall.RTM_DELTCLASS, syscall.NLM_F_ACK)
	base := class.Attrs()
	msg := &nl.TcMsg{
		Family:  nl.FAMILY_ALL,
		Ifindex: int32(base.LinkIndex),
		Handle:  base.Handle,
		Parent:  base.Parent,
	}
	req.AddData(msg)

	_, err := req.Execute(syscall.NETLINK_ROUTE, 0)
	return err
}

// ClassAdd will add a class to the system.
// Equivalent to: `tc class add $class`
func ClassAdd(class Class) error {
	req := nl.NewNetlinkRequest(syscall.RTM_NEWTCLASS, syscall.NLM_F_CREATE|syscall.NLM_F_EXCL|syscall.NLM_F_ACK)
	base := class.Attrs()
	msg := &nl.TcMsg{
		Family:  nl.FAMILY_ALL,
		Ifindex: int32(base.LinkIndex),
		Handle:  base.Handle,
		Parent:  base.Parent,
	}
	req.AddData(msg)
	req.AddData(nl.NewRtAttr(nl.TCA_KIND, nl.ZeroTerminated(class.Type())))

	options := nl.NewRtAttr(nl.TCA_OPTIONS, nil)
	if htb, ok := class.(*HtbClass); ok {
		opt := nl.TcHtbCopt{}
		opt.Rate.Rate = uint32(htb.Rate)
		opt.Ceil.Rate = uint32(htb.Ceil)
		opt.Buffer = htb.Buffer
		opt.Cbuffer = htb.Cbuffer
		opt.Quantum = htb.Quantum
		opt.Level = htb.Level
		opt.Prio = htb.Prio
		// TODO: Handle Debug properly. For now default to 0
		nl.NewRtAttrChild(options, nl.TCA_HTB_PARMS, opt.Serialize())
	}
	req.AddData(options)
	_, err := req.Execute(syscall.NETLINK_ROUTE, 0)
	return err
}

// ClassList gets a list of classes in the system.
// Equivalent to: `tc class show`.
// Generally retunrs nothing if link and parent are not specified.
func ClassList(link Link, parent uint32) ([]Class, error) {
	req := nl.NewNetlinkRequest(syscall.RTM_GETTCLASS, syscall.NLM_F_DUMP)
	msg := &nl.TcMsg{
		Family: nl.FAMILY_ALL,
		Parent: parent,
	}
	if link != nil {
		base := link.Attrs()
		ensureIndex(base)
		msg.Ifindex = int32(base.Index)
	}
	req.AddData(msg)

	msgs, err := req.Execute(syscall.NETLINK_ROUTE, syscall.RTM_NEWTCLASS)
	if err != nil {
		return nil, err
	}

	var res []Class
	for _, m := range msgs {
		msg := nl.DeserializeTcMsg(m)

		attrs, err := nl.ParseRouteAttr(m[msg.Len():])
		if err != nil {
			return nil, err
		}

		base := ClassAttrs{
			LinkIndex: int(msg.Ifindex),
			Handle:    msg.Handle,
			Parent:    msg.Parent,
		}

		var class Class
		classType := ""
		for _, attr := range attrs {
			switch attr.Attr.Type {
			case nl.TCA_KIND:
				classType = string(attr.Value[:len(attr.Value)-1])
				switch classType {
				case "htb":
					class = &HtbClass{}
				default:
					class = &GenericClass{ClassType: classType}
				}
			case nl.TCA_OPTIONS:
				switch classType {
				case "htb":
					data, err := nl.ParseRouteAttr(attr.Value)
					if err != nil {
						return nil, err
					}
					_, err = parseHtbClassData(class, data)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		*class.Attrs() = base
		res = append(res, class)
	}

	return res, nil
}

func parseHtbClassData(class Class, data []syscall.NetlinkRouteAttr) (bool, error) {
	htb := class.(*HtbClass)
	detailed := false
	for _, datum := range data {
		switch datum.Attr.Type {
		case nl.TCA_HTB_PARMS:
			opt := nl.DeserializeTcHtbCopt(datum.Value)
			htb.Rate = uint64(opt.Rate.Rate)
			htb.Ceil = uint64(opt.Ceil.Rate)
			htb.Buffer = opt.Buffer
			htb.Cbuffer = opt.Cbuffer
			htb.Quantum = opt.Quantum
			htb.Level = opt.Level
			htb.Prio = opt.Prio
		}
	}
	return detailed, nil
}
