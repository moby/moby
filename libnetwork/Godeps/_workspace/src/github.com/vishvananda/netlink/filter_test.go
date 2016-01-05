package netlink

import (
	"syscall"
	"testing"

	"github.com/vishvananda/netlink/nl"
)

func TestFilterAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  syscall.ETH_P_IP,
		},
		RedirIndex: redir.Attrs().Index,
	}
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}
	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}
	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterFwAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	attrs := QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Handle:    MakeHandle(0xffff, 0),
		Parent:    HANDLE_ROOT,
	}
	qdisc := NewHtb(attrs)
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Htb)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classattrs := ClassAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    MakeHandle(0xffff, 0),
		Handle:    MakeHandle(0xffff, 2),
	}

	htbclassattrs := HtbClassAttrs{
		Rate:    1234000,
		Cbuffer: 1690,
	}
	class := NewHtbClass(classattrs, htbclassattrs)
	if err := ClassAdd(class); err != nil {
		t.Fatal(err)
	}
	classes, err := ClassList(link, MakeHandle(0xffff, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}

	filterattrs := FilterAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    MakeHandle(0xffff, 0),
		Handle:    MakeHandle(0, 0x6),
		Priority:  1,
		Protocol:  syscall.ETH_P_IP,
	}
	fwattrs := FilterFwAttrs{
		Buffer:   12345,
		Rate:     1234,
		PeakRate: 2345,
		Action:   nl.TC_POLICE_SHOT,
		ClassId:  MakeHandle(0xffff, 2),
	}

	filter, err := NewFw(filterattrs, fwattrs)
	if err != nil {
		t.Fatal(err)
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	fw, ok := filters[0].(*Fw)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}
	if fw.Police.Rate.Rate != filter.Police.Rate.Rate {
		t.Fatal("Police Rate doesn't match")
	}
	for i := range fw.Rtab {
		if fw.Rtab[i] != filter.Rtab[i] {
			t.Fatal("Rtab doesn't match")
		}
		if fw.Ptab[i] != filter.Ptab[i] {
			t.Fatal("Ptab doesn't match")
		}
	}
	if fw.ClassId != filter.ClassId {
		t.Fatal("ClassId doesn't match")
	}
	if fw.InDev != filter.InDev {
		t.Fatal("InDev doesn't match")
	}
	if fw.AvRate != filter.AvRate {
		t.Fatal("AvRate doesn't match")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}
	if err := ClassDel(class); err != nil {
		t.Fatal(err)
	}
	classes, err = ClassList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 0 {
		t.Fatal("Failed to remove class")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = QdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}
