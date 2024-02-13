package dns

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"strconv"
	"strings"
	"time"
)

// HMAC hashing codes. These are transmitted as domain names.
const (
	HmacSHA1   = "hmac-sha1."
	HmacSHA224 = "hmac-sha224."
	HmacSHA256 = "hmac-sha256."
	HmacSHA384 = "hmac-sha384."
	HmacSHA512 = "hmac-sha512."

	HmacMD5 = "hmac-md5.sig-alg.reg.int." // Deprecated: HmacMD5 is no longer supported.
)

// TsigProvider provides the API to plug-in a custom TSIG implementation.
type TsigProvider interface {
	// Generate is passed the DNS message to be signed and the partial TSIG RR. It returns the signature and nil, otherwise an error.
	Generate(msg []byte, t *TSIG) ([]byte, error)
	// Verify is passed the DNS message to be verified and the TSIG RR. If the signature is valid it will return nil, otherwise an error.
	Verify(msg []byte, t *TSIG) error
}

type tsigHMACProvider string

func (key tsigHMACProvider) Generate(msg []byte, t *TSIG) ([]byte, error) {
	// If we barf here, the caller is to blame
	rawsecret, err := fromBase64([]byte(key))
	if err != nil {
		return nil, err
	}
	var h hash.Hash
	switch CanonicalName(t.Algorithm) {
	case HmacSHA1:
		h = hmac.New(sha1.New, rawsecret)
	case HmacSHA224:
		h = hmac.New(sha256.New224, rawsecret)
	case HmacSHA256:
		h = hmac.New(sha256.New, rawsecret)
	case HmacSHA384:
		h = hmac.New(sha512.New384, rawsecret)
	case HmacSHA512:
		h = hmac.New(sha512.New, rawsecret)
	default:
		return nil, ErrKeyAlg
	}
	h.Write(msg)
	return h.Sum(nil), nil
}

func (key tsigHMACProvider) Verify(msg []byte, t *TSIG) error {
	b, err := key.Generate(msg, t)
	if err != nil {
		return err
	}
	mac, err := hex.DecodeString(t.MAC)
	if err != nil {
		return err
	}
	if !hmac.Equal(b, mac) {
		return ErrSig
	}
	return nil
}

// TSIG is the RR the holds the transaction signature of a message.
// See RFC 2845 and RFC 4635.
type TSIG struct {
	Hdr        RR_Header
	Algorithm  string `dns:"domain-name"`
	TimeSigned uint64 `dns:"uint48"`
	Fudge      uint16
	MACSize    uint16
	MAC        string `dns:"size-hex:MACSize"`
	OrigId     uint16
	Error      uint16
	OtherLen   uint16
	OtherData  string `dns:"size-hex:OtherLen"`
}

// TSIG has no official presentation format, but this will suffice.

func (rr *TSIG) String() string {
	s := "\n;; TSIG PSEUDOSECTION:\n; " // add another semi-colon to signify TSIG does not have a presentation format
	s += rr.Hdr.String() +
		" " + rr.Algorithm +
		" " + tsigTimeToString(rr.TimeSigned) +
		" " + strconv.Itoa(int(rr.Fudge)) +
		" " + strconv.Itoa(int(rr.MACSize)) +
		" " + strings.ToUpper(rr.MAC) +
		" " + strconv.Itoa(int(rr.OrigId)) +
		" " + strconv.Itoa(int(rr.Error)) + // BIND prints NOERROR
		" " + strconv.Itoa(int(rr.OtherLen)) +
		" " + rr.OtherData
	return s
}

func (*TSIG) parse(c *zlexer, origin string) *ParseError {
	return &ParseError{err: "TSIG records do not have a presentation format"}
}

// The following values must be put in wireformat, so that the MAC can be calculated.
// RFC 2845, section 3.4.2. TSIG Variables.
type tsigWireFmt struct {
	// From RR_Header
	Name  string `dns:"domain-name"`
	Class uint16
	Ttl   uint32
	// Rdata of the TSIG
	Algorithm  string `dns:"domain-name"`
	TimeSigned uint64 `dns:"uint48"`
	Fudge      uint16
	// MACSize, MAC and OrigId excluded
	Error     uint16
	OtherLen  uint16
	OtherData string `dns:"size-hex:OtherLen"`
}

// If we have the MAC use this type to convert it to wiredata. Section 3.4.3. Request MAC
type macWireFmt struct {
	MACSize uint16
	MAC     string `dns:"size-hex:MACSize"`
}

// 3.3. Time values used in TSIG calculations
type timerWireFmt struct {
	TimeSigned uint64 `dns:"uint48"`
	Fudge      uint16
}

// TsigGenerate fills out the TSIG record attached to the message.
// The message should contain
// a "stub" TSIG RR with the algorithm, key name (owner name of the RR),
// time fudge (defaults to 300 seconds) and the current time
// The TSIG MAC is saved in that Tsig RR.
// When TsigGenerate is called for the first time requestMAC is set to the empty string and
// timersOnly is false.
// If something goes wrong an error is returned, otherwise it is nil.
func TsigGenerate(m *Msg, secret, requestMAC string, timersOnly bool) ([]byte, string, error) {
	return tsigGenerateProvider(m, tsigHMACProvider(secret), requestMAC, timersOnly)
}

func tsigGenerateProvider(m *Msg, provider TsigProvider, requestMAC string, timersOnly bool) ([]byte, string, error) {
	if m.IsTsig() == nil {
		panic("dns: TSIG not last RR in additional")
	}

	rr := m.Extra[len(m.Extra)-1].(*TSIG)
	m.Extra = m.Extra[0 : len(m.Extra)-1] // kill the TSIG from the msg
	mbuf, err := m.Pack()
	if err != nil {
		return nil, "", err
	}
	buf, err := tsigBuffer(mbuf, rr, requestMAC, timersOnly)
	if err != nil {
		return nil, "", err
	}

	t := new(TSIG)
	// Copy all TSIG fields except MAC and its size, which are filled using the computed digest.
	*t = *rr
	mac, err := provider.Generate(buf, rr)
	if err != nil {
		return nil, "", err
	}
	t.MAC = hex.EncodeToString(mac)
	t.MACSize = uint16(len(t.MAC) / 2) // Size is half!

	tbuf := make([]byte, Len(t))
	off, err := PackRR(t, tbuf, 0, nil, false)
	if err != nil {
		return nil, "", err
	}
	mbuf = append(mbuf, tbuf[:off]...)
	// Update the ArCount directly in the buffer.
	binary.BigEndian.PutUint16(mbuf[10:], uint16(len(m.Extra)+1))

	return mbuf, t.MAC, nil
}

// TsigVerify verifies the TSIG on a message.
// If the signature does not validate err contains the
// error, otherwise it is nil.
func TsigVerify(msg []byte, secret, requestMAC string, timersOnly bool) error {
	return tsigVerify(msg, tsigHMACProvider(secret), requestMAC, timersOnly, uint64(time.Now().Unix()))
}

func tsigVerifyProvider(msg []byte, provider TsigProvider, requestMAC string, timersOnly bool) error {
	return tsigVerify(msg, provider, requestMAC, timersOnly, uint64(time.Now().Unix()))
}

// actual implementation of TsigVerify, taking the current time ('now') as a parameter for the convenience of tests.
func tsigVerify(msg []byte, provider TsigProvider, requestMAC string, timersOnly bool, now uint64) error {
	// Strip the TSIG from the incoming msg
	stripped, tsig, err := stripTsig(msg)
	if err != nil {
		return err
	}

	buf, err := tsigBuffer(stripped, tsig, requestMAC, timersOnly)
	if err != nil {
		return err
	}

	if err := provider.Verify(buf, tsig); err != nil {
		return err
	}

	// Fudge factor works both ways. A message can arrive before it was signed because
	// of clock skew.
	// We check this after verifying the signature, following draft-ietf-dnsop-rfc2845bis
	// instead of RFC2845, in order to prevent a security vulnerability as reported in CVE-2017-3142/3143.
	ti := now - tsig.TimeSigned
	if now < tsig.TimeSigned {
		ti = tsig.TimeSigned - now
	}
	if uint64(tsig.Fudge) < ti {
		return ErrTime
	}

	return nil
}

// Create a wiredata buffer for the MAC calculation.
func tsigBuffer(msgbuf []byte, rr *TSIG, requestMAC string, timersOnly bool) ([]byte, error) {
	var buf []byte
	if rr.TimeSigned == 0 {
		rr.TimeSigned = uint64(time.Now().Unix())
	}
	if rr.Fudge == 0 {
		rr.Fudge = 300 // Standard (RFC) default.
	}

	// Replace message ID in header with original ID from TSIG
	binary.BigEndian.PutUint16(msgbuf[0:2], rr.OrigId)

	if requestMAC != "" {
		m := new(macWireFmt)
		m.MACSize = uint16(len(requestMAC) / 2)
		m.MAC = requestMAC
		buf = make([]byte, len(requestMAC)) // long enough
		n, err := packMacWire(m, buf)
		if err != nil {
			return nil, err
		}
		buf = buf[:n]
	}

	tsigvar := make([]byte, DefaultMsgSize)
	if timersOnly {
		tsig := new(timerWireFmt)
		tsig.TimeSigned = rr.TimeSigned
		tsig.Fudge = rr.Fudge
		n, err := packTimerWire(tsig, tsigvar)
		if err != nil {
			return nil, err
		}
		tsigvar = tsigvar[:n]
	} else {
		tsig := new(tsigWireFmt)
		tsig.Name = CanonicalName(rr.Hdr.Name)
		tsig.Class = ClassANY
		tsig.Ttl = rr.Hdr.Ttl
		tsig.Algorithm = CanonicalName(rr.Algorithm)
		tsig.TimeSigned = rr.TimeSigned
		tsig.Fudge = rr.Fudge
		tsig.Error = rr.Error
		tsig.OtherLen = rr.OtherLen
		tsig.OtherData = rr.OtherData
		n, err := packTsigWire(tsig, tsigvar)
		if err != nil {
			return nil, err
		}
		tsigvar = tsigvar[:n]
	}

	if requestMAC != "" {
		x := append(buf, msgbuf...)
		buf = append(x, tsigvar...)
	} else {
		buf = append(msgbuf, tsigvar...)
	}
	return buf, nil
}

// Strip the TSIG from the raw message.
func stripTsig(msg []byte) ([]byte, *TSIG, error) {
	// Copied from msg.go's Unpack() Header, but modified.
	var (
		dh  Header
		err error
	)
	off, tsigoff := 0, 0

	if dh, off, err = unpackMsgHdr(msg, off); err != nil {
		return nil, nil, err
	}
	if dh.Arcount == 0 {
		return nil, nil, ErrNoSig
	}

	// Rcode, see msg.go Unpack()
	if int(dh.Bits&0xF) == RcodeNotAuth {
		return nil, nil, ErrAuth
	}

	for i := 0; i < int(dh.Qdcount); i++ {
		_, off, err = unpackQuestion(msg, off)
		if err != nil {
			return nil, nil, err
		}
	}

	_, off, err = unpackRRslice(int(dh.Ancount), msg, off)
	if err != nil {
		return nil, nil, err
	}
	_, off, err = unpackRRslice(int(dh.Nscount), msg, off)
	if err != nil {
		return nil, nil, err
	}

	rr := new(TSIG)
	var extra RR
	for i := 0; i < int(dh.Arcount); i++ {
		tsigoff = off
		extra, off, err = UnpackRR(msg, off)
		if err != nil {
			return nil, nil, err
		}
		if extra.Header().Rrtype == TypeTSIG {
			rr = extra.(*TSIG)
			// Adjust Arcount.
			arcount := binary.BigEndian.Uint16(msg[10:])
			binary.BigEndian.PutUint16(msg[10:], arcount-1)
			break
		}
	}
	if rr == nil {
		return nil, nil, ErrNoSig
	}
	return msg[:tsigoff], rr, nil
}

// Translate the TSIG time signed into a date. There is no
// need for RFC1982 calculations as this date is 48 bits.
func tsigTimeToString(t uint64) string {
	ti := time.Unix(int64(t), 0).UTC()
	return ti.Format("20060102150405")
}

func packTsigWire(tw *tsigWireFmt, msg []byte) (int, error) {
	// copied from zmsg.go TSIG packing
	// RR_Header
	off, err := PackDomainName(tw.Name, msg, 0, nil, false)
	if err != nil {
		return off, err
	}
	off, err = packUint16(tw.Class, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(tw.Ttl, msg, off)
	if err != nil {
		return off, err
	}

	off, err = PackDomainName(tw.Algorithm, msg, off, nil, false)
	if err != nil {
		return off, err
	}
	off, err = packUint48(tw.TimeSigned, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint16(tw.Fudge, msg, off)
	if err != nil {
		return off, err
	}

	off, err = packUint16(tw.Error, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint16(tw.OtherLen, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packStringHex(tw.OtherData, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func packMacWire(mw *macWireFmt, msg []byte) (int, error) {
	off, err := packUint16(mw.MACSize, msg, 0)
	if err != nil {
		return off, err
	}
	off, err = packStringHex(mw.MAC, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func packTimerWire(tw *timerWireFmt, msg []byte) (int, error) {
	off, err := packUint48(tw.TimeSigned, msg, 0)
	if err != nil {
		return off, err
	}
	off, err = packUint16(tw.Fudge, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}
