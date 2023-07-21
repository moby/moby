// Copyright 2017 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/gopacket"
)

// SIPVersion defines the different versions of the SIP Protocol
type SIPVersion uint8

// Represents all the versions of SIP protocol
const (
	SIPVersion1 SIPVersion = 1
	SIPVersion2 SIPVersion = 2
)

func (sv SIPVersion) String() string {
	switch sv {
	default:
		// Defaulting to SIP/2.0
		return "SIP/2.0"
	case SIPVersion1:
		return "SIP/1.0"
	case SIPVersion2:
		return "SIP/2.0"
	}
}

// GetSIPVersion is used to get SIP version constant
func GetSIPVersion(version string) (SIPVersion, error) {
	switch strings.ToUpper(version) {
	case "SIP/1.0":
		return SIPVersion1, nil
	case "SIP/2.0":
		return SIPVersion2, nil
	default:
		return 0, fmt.Errorf("Unknown SIP version: '%s'", version)

	}
}

// SIPMethod defines the different methods of the SIP Protocol
// defined in the different RFC's
type SIPMethod uint16

// Here are all the SIP methods
const (
	SIPMethodInvite    SIPMethod = 1  // INVITE	[RFC3261]
	SIPMethodAck       SIPMethod = 2  // ACK	[RFC3261]
	SIPMethodBye       SIPMethod = 3  // BYE	[RFC3261]
	SIPMethodCancel    SIPMethod = 4  // CANCEL	[RFC3261]
	SIPMethodOptions   SIPMethod = 5  // OPTIONS	[RFC3261]
	SIPMethodRegister  SIPMethod = 6  // REGISTER	[RFC3261]
	SIPMethodPrack     SIPMethod = 7  // PRACK	[RFC3262]
	SIPMethodSubscribe SIPMethod = 8  // SUBSCRIBE	[RFC6665]
	SIPMethodNotify    SIPMethod = 9  // NOTIFY	[RFC6665]
	SIPMethodPublish   SIPMethod = 10 // PUBLISH	[RFC3903]
	SIPMethodInfo      SIPMethod = 11 // INFO	[RFC6086]
	SIPMethodRefer     SIPMethod = 12 // REFER	[RFC3515]
	SIPMethodMessage   SIPMethod = 13 // MESSAGE	[RFC3428]
	SIPMethodUpdate    SIPMethod = 14 // UPDATE	[RFC3311]
	SIPMethodPing      SIPMethod = 15 // PING	[https://tools.ietf.org/html/draft-fwmiller-ping-03]
)

func (sm SIPMethod) String() string {
	switch sm {
	default:
		return "Unknown method"
	case SIPMethodInvite:
		return "INVITE"
	case SIPMethodAck:
		return "ACK"
	case SIPMethodBye:
		return "BYE"
	case SIPMethodCancel:
		return "CANCEL"
	case SIPMethodOptions:
		return "OPTIONS"
	case SIPMethodRegister:
		return "REGISTER"
	case SIPMethodPrack:
		return "PRACK"
	case SIPMethodSubscribe:
		return "SUBSCRIBE"
	case SIPMethodNotify:
		return "NOTIFY"
	case SIPMethodPublish:
		return "PUBLISH"
	case SIPMethodInfo:
		return "INFO"
	case SIPMethodRefer:
		return "REFER"
	case SIPMethodMessage:
		return "MESSAGE"
	case SIPMethodUpdate:
		return "UPDATE"
	case SIPMethodPing:
		return "PING"
	}
}

// GetSIPMethod returns the constant of a SIP method
// from its string
func GetSIPMethod(method string) (SIPMethod, error) {
	switch strings.ToUpper(method) {
	case "INVITE":
		return SIPMethodInvite, nil
	case "ACK":
		return SIPMethodAck, nil
	case "BYE":
		return SIPMethodBye, nil
	case "CANCEL":
		return SIPMethodCancel, nil
	case "OPTIONS":
		return SIPMethodOptions, nil
	case "REGISTER":
		return SIPMethodRegister, nil
	case "PRACK":
		return SIPMethodPrack, nil
	case "SUBSCRIBE":
		return SIPMethodSubscribe, nil
	case "NOTIFY":
		return SIPMethodNotify, nil
	case "PUBLISH":
		return SIPMethodPublish, nil
	case "INFO":
		return SIPMethodInfo, nil
	case "REFER":
		return SIPMethodRefer, nil
	case "MESSAGE":
		return SIPMethodMessage, nil
	case "UPDATE":
		return SIPMethodUpdate, nil
	case "PING":
		return SIPMethodPing, nil
	default:
		return 0, fmt.Errorf("Unknown SIP method: '%s'", method)
	}
}

// Here is a correspondance between long header names and short
// as defined in rfc3261 in section 20
var compactSipHeadersCorrespondance = map[string]string{
	"accept-contact":      "a",
	"allow-events":        "u",
	"call-id":             "i",
	"contact":             "m",
	"content-encoding":    "e",
	"content-length":      "l",
	"content-type":        "c",
	"event":               "o",
	"from":                "f",
	"identity":            "y",
	"refer-to":            "r",
	"referred-by":         "b",
	"reject-contact":      "j",
	"request-disposition": "d",
	"session-expires":     "x",
	"subject":             "s",
	"supported":           "k",
	"to":                  "t",
	"via":                 "v",
}

// SIP object will contains information about decoded SIP packet.
// -> The SIP Version
// -> The SIP Headers (in a map[string][]string because of multiple headers with the same name
// -> The SIP Method
// -> The SIP Response code (if it's a response)
// -> The SIP Status line (if it's a response)
// You can easily know the type of the packet with the IsResponse boolean
//
type SIP struct {
	BaseLayer

	// Base information
	Version SIPVersion
	Method  SIPMethod
	Headers map[string][]string

	// Request
	RequestURI string

	// Response
	IsResponse     bool
	ResponseCode   int
	ResponseStatus string

	// Private fields
	cseq             int64
	contentLength    int64
	lastHeaderParsed string
}

// decodeSIP decodes the byte slice into a SIP type. It also
// setups the application Layer in PacketBuilder.
func decodeSIP(data []byte, p gopacket.PacketBuilder) error {
	s := NewSIP()
	err := s.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(s)
	p.SetApplicationLayer(s)
	return nil
}

// NewSIP instantiates a new empty SIP object
func NewSIP() *SIP {
	s := new(SIP)
	s.Headers = make(map[string][]string)
	return s
}

// LayerType returns gopacket.LayerTypeSIP.
func (s *SIP) LayerType() gopacket.LayerType {
	return LayerTypeSIP
}

// Payload returns the base layer payload
func (s *SIP) Payload() []byte {
	return s.BaseLayer.Payload
}

// CanDecode returns the set of layer types that this DecodingLayer can decode
func (s *SIP) CanDecode() gopacket.LayerClass {
	return LayerTypeSIP
}

// NextLayerType returns the layer type contained by this DecodingLayer
func (s *SIP) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

// DecodeFromBytes decodes the slice into the SIP struct.
func (s *SIP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	// Init some vars for parsing follow-up
	var countLines int
	var line []byte
	var err error
	var offset int

	// Iterate on all lines of the SIP Headers
	// and stop when we reach the SDP (aka when the new line
	// is at index 0 of the remaining packet)
	buffer := bytes.NewBuffer(data)

	for {

		// Read next line
		line, err = buffer.ReadBytes(byte('\n'))
		if err != nil {
			if err == io.EOF {
				if len(bytes.Trim(line, "\r\n")) > 0 {
					df.SetTruncated()
				}
				break
			} else {
				return err
			}
		}
		offset += len(line)

		// Trim the new line delimiters
		line = bytes.Trim(line, "\r\n")

		// Empty line, we hit Body
		if len(line) == 0 {
			break
		}

		// First line is the SIP request/response line
		// Other lines are headers
		if countLines == 0 {
			err = s.ParseFirstLine(line)
			if err != nil {
				return err
			}

		} else {
			err = s.ParseHeader(line)
			if err != nil {
				return err
			}
		}

		countLines++
	}
	s.BaseLayer = BaseLayer{Contents: data[:offset], Payload: data[offset:]}

	return nil
}

// ParseFirstLine will compute the first line of a SIP packet.
// The first line will tell us if it's a request or a response.
//
// Examples of first line of SIP Prococol :
//
// 	Request 	: INVITE bob@example.com SIP/2.0
// 	Response 	: SIP/2.0 200 OK
// 	Response	: SIP/2.0 501 Not Implemented
//
func (s *SIP) ParseFirstLine(firstLine []byte) error {

	var err error

	// Splits line by space
	splits := strings.SplitN(string(firstLine), " ", 3)

	// We must have at least 3 parts
	if len(splits) < 3 {
		return fmt.Errorf("invalid first SIP line: '%s'", string(firstLine))
	}

	// Determine the SIP packet type
	if strings.HasPrefix(splits[0], "SIP") {

		// --> Response
		s.IsResponse = true

		// Validate SIP Version
		s.Version, err = GetSIPVersion(splits[0])
		if err != nil {
			return err
		}

		// Compute code
		s.ResponseCode, err = strconv.Atoi(splits[1])
		if err != nil {
			return err
		}

		// Compute status line
		s.ResponseStatus = splits[2]

	} else {

		// --> Request

		// Validate method
		s.Method, err = GetSIPMethod(splits[0])
		if err != nil {
			return err
		}

		s.RequestURI = splits[1]

		// Validate SIP Version
		s.Version, err = GetSIPVersion(splits[2])
		if err != nil {
			return err
		}
	}

	return nil
}

// ParseHeader will parse a SIP Header
// SIP Headers are quite simple, there are colon separated name and value
// Headers can be spread over multiple lines
//
// Examples of header :
//
//  CSeq: 1 REGISTER
//  Via: SIP/2.0/UDP there.com:5060
//  Authorization:Digest username="UserB",
//	  realm="MCI WorldCom SIP",
//    nonce="1cec4341ae6cbe5a359ea9c8e88df84f", opaque="",
//    uri="sip:ss2.wcom.com", response="71ba27c64bd01de719686aa4590d5824"
//
func (s *SIP) ParseHeader(header []byte) (err error) {

	// Ignore empty headers
	if len(header) == 0 {
		return
	}

	// Check if this is the following of last header
	// RFC 3261 - 7.3.1 - Header Field Format specify that following lines of
	// multiline headers must begin by SP or TAB
	if header[0] == '\t' || header[0] == ' ' {

		header = bytes.TrimSpace(header)
		s.Headers[s.lastHeaderParsed][len(s.Headers[s.lastHeaderParsed])-1] += fmt.Sprintf(" %s", string(header))
		return
	}

	// Find the ':' to separate header name and value
	index := bytes.Index(header, []byte(":"))
	if index >= 0 {

		headerName := strings.ToLower(string(bytes.Trim(header[:index], " ")))
		headerValue := string(bytes.Trim(header[index+1:], " "))

		// Add header to object
		s.Headers[headerName] = append(s.Headers[headerName], headerValue)
		s.lastHeaderParsed = headerName

		// Compute specific headers
		err = s.ParseSpecificHeaders(headerName, headerValue)
		if err != nil {
			return err
		}
	}

	return nil
}

// ParseSpecificHeaders will parse some specific key values from
// specific headers like CSeq or Content-Length integer values
func (s *SIP) ParseSpecificHeaders(headerName string, headerValue string) (err error) {

	switch headerName {
	case "cseq":

		// CSeq header value is formatted like that :
		// CSeq: 123 INVITE
		// We split the value to parse Cseq integer value, and method
		splits := strings.Split(headerValue, " ")
		if len(splits) > 1 {

			// Parse Cseq
			s.cseq, err = strconv.ParseInt(splits[0], 10, 64)
			if err != nil {
				return err
			}

			// Validate method
			if s.IsResponse {
				s.Method, err = GetSIPMethod(splits[1])
				if err != nil {
					return err
				}
			}
		}

	case "content-length":

		// Parse Content-Length
		s.contentLength, err = strconv.ParseInt(headerValue, 10, 64)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetAllHeaders will return the full headers of the
// current SIP packets in a map[string][]string
func (s *SIP) GetAllHeaders() map[string][]string {
	return s.Headers
}

// GetHeader will return all the headers with
// the specified name.
func (s *SIP) GetHeader(headerName string) []string {
	headerName = strings.ToLower(headerName)
	h := make([]string, 0)
	if _, ok := s.Headers[headerName]; ok {
		return s.Headers[headerName]
	}
	compactHeader := compactSipHeadersCorrespondance[headerName]
	if _, ok := s.Headers[compactHeader]; ok {
		return s.Headers[compactHeader]
	}
	return h
}

// GetFirstHeader will return the first header with
// the specified name. If the current SIP packet has multiple
// headers with the same name, it returns the first.
func (s *SIP) GetFirstHeader(headerName string) string {
	headers := s.GetHeader(headerName)
	if len(headers) > 0 {
		return headers[0]
	}
	return ""
}

//
// Some handy getters for most used SIP headers
//

// GetAuthorization will return the Authorization
// header of the current SIP packet
func (s *SIP) GetAuthorization() string {
	return s.GetFirstHeader("Authorization")
}

// GetFrom will return the From
// header of the current SIP packet
func (s *SIP) GetFrom() string {
	return s.GetFirstHeader("From")
}

// GetTo will return the To
// header of the current SIP packet
func (s *SIP) GetTo() string {
	return s.GetFirstHeader("To")
}

// GetContact will return the Contact
// header of the current SIP packet
func (s *SIP) GetContact() string {
	return s.GetFirstHeader("Contact")
}

// GetCallID will return the Call-ID
// header of the current SIP packet
func (s *SIP) GetCallID() string {
	return s.GetFirstHeader("Call-ID")
}

// GetUserAgent will return the User-Agent
// header of the current SIP packet
func (s *SIP) GetUserAgent() string {
	return s.GetFirstHeader("User-Agent")
}

// GetContentLength will return the parsed integer
// Content-Length header of the current SIP packet
func (s *SIP) GetContentLength() int64 {
	return s.contentLength
}

// GetCSeq will return the parsed integer CSeq header
// header of the current SIP packet
func (s *SIP) GetCSeq() int64 {
	return s.cseq
}
