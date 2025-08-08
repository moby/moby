package overlay

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/gogo/protobuf/proto"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(ctx context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, _, options map[string]any) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.drivers.windows_overlay.Join", trace.WithAttributes(
		attribute.String("nid", nid),
		attribute.String("eid", eid),
		attribute.String("sboxKey", sboxKey)))
	defer span.End()

	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("could not find network with id %s", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}

	buf, err := proto.Marshal(&overlay.PeerRecord{
		EndpointIP:       ep.addr.String(),
		EndpointMAC:      ep.mac.String(),
		TunnelEndpointIP: n.providerAddress,
	})
	if err != nil {
		return err
	}

	if err := jinfo.AddTableEntry(overlay.OverlayPeerTable, eid, buf); err != nil {
		log.G(ctx).Errorf("overlay: Failed adding table entry to joininfo: %v", err)
	}

	if ep.disablegateway {
		jinfo.DisableGatewayService()
	}

	return nil
}

func (d *driver) EventNotify(nid, tableName, key string, prev, value []byte) {
	if tableName != overlay.OverlayPeerTable {
		log.G(context.TODO()).Errorf("Unexpected table notification for table %s received", tableName)
		return
	}

	eid := key

	n := d.network(nid)
	if n == nil {
		return
	}

	var prevPeer, newPeer *overlay.Peer
	if prev != nil {
		var err error
		prevPeer, err = overlay.UnmarshalPeerRecord(prev)
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("Failed to unmarshal previous peer record")
		} else if prevPeer.TunnelEndpointIP.String() == n.providerAddress {
			// Ignore local peers. We don't add them to the VXLAN
			// FDB so don't need to remove them.
			prevPeer = nil
		}
	}
	if value != nil {
		var err error
		newPeer, err = overlay.UnmarshalPeerRecord(value)
		if err != nil {
			log.G(context.TODO()).WithError(err).Error("Failed to unmarshal peer record")
		} else if newPeer.TunnelEndpointIP.String() == n.providerAddress {
			newPeer = nil
		}
	}

	if prevPeer == nil && newPeer == nil {
		// Nothing to do! Either the event was for a local peer,
		// or unmarshaling failed.
		return
	}
	if prevPeer != nil && newPeer != nil && *prevPeer == *newPeer {
		// The update did not materially change the FDB entry.
		return
	}

	if prevPeer != nil {
		if err := d.peerDelete(nid, eid, prevPeer.EndpointIP.Addr().AsSlice(), true); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error": err,
				"nid":   n.id,
				"peer":  prevPeer,
			}).Warn("overlay: failed to delete peer entry")
		}
	}
	if newPeer != nil {
		if err := d.peerAdd(nid, eid, newPeer.EndpointIP.Addr().AsSlice(), newPeer.EndpointMAC.AsSlice(), newPeer.TunnelEndpointIP.AsSlice(), true); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error": err,
				"nid":   n.id,
				"peer":  newPeer,
			}).Warn("overlay: failed to add peer entry")
		}
	}
}

func (d *driver) DecodeTableEntry(tablename string, key string, value []byte) (string, map[string]string) {
	return "", nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	return nil
}
