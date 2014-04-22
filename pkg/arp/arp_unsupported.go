// +build !linux

package arp

const (
	ETH_P_IP = 0
)

func (a *ARP) Send(dstIP string) error {
	return ErrUnsupported
}

func (a *ARP) Bytes() []byte {
	return nil
}
