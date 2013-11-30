package utils

import (
	"crypto/sha1"
	"encoding/binary"
	"net"
	"time"
)

// timeNTP() retrieves the current time from NTP's official server, returning uint64-encoded 'seconds' and 'fractions'
// See RFC 5905
func timeNTP() (uint64, uint64, error) {
	ntps, err := net.ResolveUDPAddr("udp", "0.pool.ntp.org:123")

	data := make([]byte, 48)
	data[0] = 3<<3 | 3

	con, err := net.DialUDP("udp", nil, ntps)
	defer con.Close()

	_, err = con.Write(data)

	con.SetDeadline(time.Now().Add(5 * time.Second))

	_, err = con.Read(data)
	if err != nil {
		return 0, 0, err
	}

	var sec, frac uint64

	sec = uint64(data[43]) | uint64(data[42])<<8 | uint64(data[41])<<16 | uint64(data[40])<<24
	frac = uint64(data[47]) | uint64(data[46])<<8 | uint64(data[45])<<16 | uint64(data[44])<<24
	return sec, frac, nil
}

// Uint48([]byte) encodes a 48-bit (6 byte) []byte such as an interface MAC address into a uint64.
func Uint48(b []byte) uint64 {
	return uint64(b[5]) | uint64(b[4])<<8 | uint64(b[3])<<16 | uint64(b[2])<<24 | uint64(b[1])<<32 | uint64(b[0])<<40
}

// findMAC() discovers the 'best' interface to use for IPv6 ULA generation; it loops through each available interface, looking for a non-zero, non-one MAC address.
// If none are found, it returns 0.
func findMAC() uint64 {
	interfaces, _ := net.Interfaces()
	for i := range interfaces {
		mac := interfaces[i].HardwareAddr
		if mac != nil {
			macint := Uint48(mac)
			if macint > 1 {
				return macint
			}
		}
	}
	return 0
}

// GenULA() generates Unique Local Addresses for IPv6, implementing the algorithm suggested in RFC 4193
func GenULA() string {
	ntpsec, ntpfrac, _ := timeNTP()
	mac := findMAC()
	if mac == 0 {
		mac = uint64(123456789123) // non-standard-compliant placeholder in case of error
	}
	key := ntpsec + ntpfrac + uint64(mac)
	keyb := make([]byte, 8)
	binary.BigEndian.PutUint64(keyb, key)
	sha := sha1.New()
	shakey := sha.Sum(keyb)
	ip := net.IP(make([]byte, 16))
	pre := []byte{252}

	for i := 0; i < len(pre); i++ {
		ip[i] = pre[i]
	}

	for i := 0; i < 7; i++ {
		n := i + 1
		ip[n] = shakey[i]
	}

	return ip.String() + "/64"
}
