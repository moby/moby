//+build !go1.6

package libvirt

import (
	"testing"
	"time"
)

func TestDomainEventRegister(t *testing.T) {

	callbackId := -1

	conn := buildTestConnection()
	defer func() {
		if callbackId >= 0 {
			conn.DomainEventDeregister(callbackId)
		}
		conn.CloseConnection()
	}()

	nodom := VirDomain{}
	defName := time.Now().String()

	nbEvents := 0

	callback := DomainEventCallback(
		func(c *VirConnection, d *VirDomain, eventDetails interface{}, f func()) int {
			if lifecycleEvent, ok := eventDetails.(DomainLifecycleEvent); ok {
				if lifecycleEvent.Event == VIR_DOMAIN_EVENT_STARTED {
					domName, _ := d.GetName()
					if defName != domName {
						t.Fatalf("Name was not '%s': %s", defName, domName)
					}
				}
			} else {
				t.Fatalf("event details isn't DomainLifecycleEvent")
			}
			f()
			return 0
		},
	)

	EventRegisterDefaultImpl()

	callbackId = conn.DomainEventRegister(
		VirDomain{},
		VIR_DOMAIN_EVENT_ID_LIFECYCLE,
		&callback,
		func() {
			nbEvents++
		},
	)

	// Test a minimally valid xml
	xml := `<domain type="test">
		<name>` + defName + `</name>
		<memory unit="KiB">8192</memory>
		<os>
			<type>hvm</type>
		</os>
	</domain>`
	dom, err := conn.DomainCreateXML(xml, VIR_DOMAIN_NONE)
	if err != nil {
		t.Error(err)
		return
	}

	// This is blocking as long as there is no message
	EventRunDefaultImpl()
	if nbEvents == 0 {
		t.Fatal("At least one event was expected")
	}

	defer func() {
		if dom != nodom {
			dom.Destroy()
			dom.Free()
		}
	}()

	// Deregister the event
	if ret := conn.DomainEventDeregister(callbackId); ret < 0 {
		t.Fatal("Event deregistration failed")
	}
	callbackId = -1 // Don't deregister twice
}
