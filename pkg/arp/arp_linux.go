// +build: linux

package arp

import (
	"encoding/binary"
	"net"
	"syscall"
)

const (
	ETH_P_IP = syscall.ETH_P_IP
)

func (a *ARP) Send(dstIP string) error {
	a.TargetIP = net.ParseIP(dstIP).To4()

	// frame length = MAC + MAC + ethernet type + ARP header
	frameLength := 6 + 6 + 2 + ARP_HDRLEN

	ethernetFrame := make([]byte, frameLength)
	copy(ethernetFrame[0:6], a.TargetMac)
	copy(ethernetFrame[6:12], a.SenderMac)
	// Ethernet type (ARP)
	ethernetFrame[12] = syscall.ETH_P_ARP / 256
	ethernetFrame[13] = syscall.ETH_P_ARP % 256
	copy(ethernetFrame[14:], a.Bytes())

	sd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	if err != nil {
		return err
	}
	defer syscall.Close(sd)

	s := syscall.SockaddrLinklayer{
		Halen:   6,
		Ifindex: a.ifaceIndex,
	}
	copy(s.Addr[:], a.SenderMac)

	return syscall.Sendto(sd, ethernetFrame, 0, &s)
}

func (a *ARP) Bytes() []byte {
	buf := make([]byte, ARP_HDRLEN)
	binary.BigEndian.PutUint16(buf[0:2], a.HType)
	binary.BigEndian.PutUint16(buf[2:4], a.PType)
	buf[4] = a.HLen
	buf[5] = a.PLen
	binary.BigEndian.PutUint16(buf[6:8], a.OpCode)
	copy(buf[8:14], a.SenderMac)
	copy(buf[14:18], a.SenderIP)
	copy(buf[18:24], a.TargetMac)
	copy(buf[24:28], a.TargetIP)
	return buf
}
