package redis

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	errorReply  = '-'
	statusReply = '+'
	intReply    = ':'
	stringReply = '$'
	arrayReply  = '*'
)

type multiBulkParser func(cn *conn, n int64) (interface{}, error)

var (
	errReaderTooSmall = errors.New("redis: reader is too small")
)

//------------------------------------------------------------------------------

// Copy of encoding.BinaryMarshaler.
type binaryMarshaler interface {
	MarshalBinary() (data []byte, err error)
}

// Copy of encoding.BinaryUnmarshaler.
type binaryUnmarshaler interface {
	UnmarshalBinary(data []byte) error
}

func appendString(b []byte, s string) []byte {
	b = append(b, '$')
	b = strconv.AppendUint(b, uint64(len(s)), 10)
	b = append(b, '\r', '\n')
	b = append(b, s...)
	b = append(b, '\r', '\n')
	return b
}

func appendBytes(b, bb []byte) []byte {
	b = append(b, '$')
	b = strconv.AppendUint(b, uint64(len(bb)), 10)
	b = append(b, '\r', '\n')
	b = append(b, bb...)
	b = append(b, '\r', '\n')
	return b
}

func appendArg(b []byte, val interface{}) ([]byte, error) {
	switch v := val.(type) {
	case nil:
		b = appendString(b, "")
	case string:
		b = appendString(b, v)
	case []byte:
		b = appendBytes(b, v)
	case int:
		b = appendString(b, formatInt(int64(v)))
	case int8:
		b = appendString(b, formatInt(int64(v)))
	case int16:
		b = appendString(b, formatInt(int64(v)))
	case int32:
		b = appendString(b, formatInt(int64(v)))
	case int64:
		b = appendString(b, formatInt(v))
	case uint:
		b = appendString(b, formatUint(uint64(v)))
	case uint8:
		b = appendString(b, formatUint(uint64(v)))
	case uint16:
		b = appendString(b, formatUint(uint64(v)))
	case uint32:
		b = appendString(b, formatUint(uint64(v)))
	case uint64:
		b = appendString(b, formatUint(v))
	case float32:
		b = appendString(b, formatFloat(float64(v)))
	case float64:
		b = appendString(b, formatFloat(v))
	case bool:
		if v {
			b = appendString(b, "1")
		} else {
			b = appendString(b, "0")
		}
	default:
		if bm, ok := val.(binaryMarshaler); ok {
			bb, err := bm.MarshalBinary()
			if err != nil {
				return nil, err
			}
			b = appendBytes(b, bb)
		} else {
			err := fmt.Errorf(
				"redis: can't marshal %T (consider implementing BinaryMarshaler)", val)
			return nil, err
		}
	}
	return b, nil
}

func appendArgs(b []byte, args []interface{}) ([]byte, error) {
	b = append(b, '*')
	b = strconv.AppendUint(b, uint64(len(args)), 10)
	b = append(b, '\r', '\n')
	for _, arg := range args {
		var err error
		b, err = appendArg(b, arg)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func scan(b []byte, val interface{}) error {
	switch v := val.(type) {
	case nil:
		return errorf("redis: Scan(nil)")
	case *string:
		*v = bytesToString(b)
		return nil
	case *[]byte:
		*v = b
		return nil
	case *int:
		var err error
		*v, err = strconv.Atoi(bytesToString(b))
		return err
	case *int8:
		n, err := strconv.ParseInt(bytesToString(b), 10, 8)
		if err != nil {
			return err
		}
		*v = int8(n)
		return nil
	case *int16:
		n, err := strconv.ParseInt(bytesToString(b), 10, 16)
		if err != nil {
			return err
		}
		*v = int16(n)
		return nil
	case *int32:
		n, err := strconv.ParseInt(bytesToString(b), 10, 16)
		if err != nil {
			return err
		}
		*v = int32(n)
		return nil
	case *int64:
		n, err := strconv.ParseInt(bytesToString(b), 10, 64)
		if err != nil {
			return err
		}
		*v = n
		return nil
	case *uint:
		n, err := strconv.ParseUint(bytesToString(b), 10, 64)
		if err != nil {
			return err
		}
		*v = uint(n)
		return nil
	case *uint8:
		n, err := strconv.ParseUint(bytesToString(b), 10, 8)
		if err != nil {
			return err
		}
		*v = uint8(n)
		return nil
	case *uint16:
		n, err := strconv.ParseUint(bytesToString(b), 10, 16)
		if err != nil {
			return err
		}
		*v = uint16(n)
		return nil
	case *uint32:
		n, err := strconv.ParseUint(bytesToString(b), 10, 32)
		if err != nil {
			return err
		}
		*v = uint32(n)
		return nil
	case *uint64:
		n, err := strconv.ParseUint(bytesToString(b), 10, 64)
		if err != nil {
			return err
		}
		*v = n
		return nil
	case *float32:
		n, err := strconv.ParseFloat(bytesToString(b), 32)
		if err != nil {
			return err
		}
		*v = float32(n)
		return err
	case *float64:
		var err error
		*v, err = strconv.ParseFloat(bytesToString(b), 64)
		return err
	case *bool:
		*v = len(b) == 1 && b[0] == '1'
		return nil
	default:
		if bu, ok := val.(binaryUnmarshaler); ok {
			return bu.UnmarshalBinary(b)
		}
		err := fmt.Errorf(
			"redis: can't unmarshal %T (consider implementing BinaryUnmarshaler)", val)
		return err
	}
}

//------------------------------------------------------------------------------

func readLine(cn *conn) ([]byte, error) {
	line, isPrefix, err := cn.rd.ReadLine()
	if err != nil {
		return line, err
	}
	if isPrefix {
		return line, errReaderTooSmall
	}
	if isNilReply(line) {
		return nil, Nil
	}
	return line, nil
}

func isNilReply(b []byte) bool {
	return len(b) == 3 && (b[0] == '$' || b[0] == '*') && b[1] == '-' && b[2] == '1'
}

func readN(cn *conn, n int) ([]byte, error) {
	var b []byte
	if cap(cn.buf) < n {
		b = make([]byte, n)
	} else {
		b = cn.buf[:n]
	}
	_, err := io.ReadFull(cn.rd, b)
	return b, err
}

//------------------------------------------------------------------------------

func parseErrorReply(cn *conn, line []byte) error {
	return errorf(string(line[1:]))
}

func parseStatusReply(cn *conn, line []byte) ([]byte, error) {
	return line[1:], nil
}

func parseIntReply(cn *conn, line []byte) (int64, error) {
	n, err := strconv.ParseInt(bytesToString(line[1:]), 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func readIntReply(cn *conn) (int64, error) {
	line, err := readLine(cn)
	if err != nil {
		return 0, err
	}
	switch line[0] {
	case errorReply:
		return 0, parseErrorReply(cn, line)
	case intReply:
		return parseIntReply(cn, line)
	default:
		return 0, fmt.Errorf("redis: can't parse int reply: %.100q", line)
	}
}

func parseBytesReply(cn *conn, line []byte) ([]byte, error) {
	if isNilReply(line) {
		return nil, Nil
	}

	replyLen, err := strconv.Atoi(bytesToString(line[1:]))
	if err != nil {
		return nil, err
	}

	b, err := readN(cn, replyLen+2)
	if err != nil {
		return nil, err
	}

	return b[:replyLen], nil
}

func readBytesReply(cn *conn) ([]byte, error) {
	line, err := readLine(cn)
	if err != nil {
		return nil, err
	}
	switch line[0] {
	case errorReply:
		return nil, parseErrorReply(cn, line)
	case stringReply:
		return parseBytesReply(cn, line)
	case statusReply:
		return parseStatusReply(cn, line)
	default:
		return nil, fmt.Errorf("redis: can't parse string reply: %.100q", line)
	}
}

func readStringReply(cn *conn) (string, error) {
	b, err := readBytesReply(cn)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readFloatReply(cn *conn) (float64, error) {
	b, err := readBytesReply(cn)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(bytesToString(b), 64)
}

func parseArrayHeader(cn *conn, line []byte) (int64, error) {
	if len(line) == 3 && line[1] == '-' && line[2] == '1' {
		return 0, Nil
	}

	n, err := strconv.ParseInt(bytesToString(line[1:]), 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func parseArrayReply(cn *conn, p multiBulkParser, line []byte) (interface{}, error) {
	n, err := parseArrayHeader(cn, line)
	if err != nil {
		return nil, err
	}
	return p(cn, n)
}

func readArrayHeader(cn *conn) (int64, error) {
	line, err := readLine(cn)
	if err != nil {
		return 0, err
	}
	switch line[0] {
	case errorReply:
		return 0, parseErrorReply(cn, line)
	case arrayReply:
		return parseArrayHeader(cn, line)
	default:
		return 0, fmt.Errorf("redis: can't parse array reply: %.100q", line)
	}
}

func readArrayReply(cn *conn, p multiBulkParser) (interface{}, error) {
	line, err := readLine(cn)
	if err != nil {
		return nil, err
	}
	switch line[0] {
	case errorReply:
		return nil, parseErrorReply(cn, line)
	case arrayReply:
		return parseArrayReply(cn, p, line)
	default:
		return nil, fmt.Errorf("redis: can't parse array reply: %.100q", line)
	}
}

func readReply(cn *conn, p multiBulkParser) (interface{}, error) {
	line, err := readLine(cn)
	if err != nil {
		return nil, err
	}

	switch line[0] {
	case errorReply:
		return nil, parseErrorReply(cn, line)
	case statusReply:
		return parseStatusReply(cn, line)
	case intReply:
		return parseIntReply(cn, line)
	case stringReply:
		return parseBytesReply(cn, line)
	case arrayReply:
		return parseArrayReply(cn, p, line)
	}
	return nil, fmt.Errorf("redis: can't parse %.100q", line)
}

func readScanReply(cn *conn) ([]string, int64, error) {
	n, err := readArrayHeader(cn)
	if err != nil {
		return nil, 0, err
	}
	if n != 2 {
		return nil, 0, fmt.Errorf("redis: got %d elements in scan reply, expected 2")
	}

	b, err := readBytesReply(cn)
	if err != nil {
		return nil, 0, err
	}

	cursor, err := strconv.ParseInt(bytesToString(b), 10, 64)
	if err != nil {
		return nil, 0, err
	}

	n, err = readArrayHeader(cn)
	if err != nil {
		return nil, 0, err
	}

	keys := make([]string, n)
	for i := int64(0); i < n; i++ {
		key, err := readStringReply(cn)
		if err != nil {
			return nil, 0, err
		}
		keys[i] = key
	}

	return keys, cursor, err
}

func sliceParser(cn *conn, n int64) (interface{}, error) {
	vals := make([]interface{}, 0, n)
	for i := int64(0); i < n; i++ {
		v, err := readReply(cn, sliceParser)
		if err == Nil {
			vals = append(vals, nil)
		} else if err != nil {
			return nil, err
		} else {
			switch vv := v.(type) {
			case []byte:
				vals = append(vals, string(vv))
			default:
				vals = append(vals, v)
			}
		}
	}
	return vals, nil
}

func intSliceParser(cn *conn, n int64) (interface{}, error) {
	ints := make([]int64, 0, n)
	for i := int64(0); i < n; i++ {
		n, err := readIntReply(cn)
		if err != nil {
			return nil, err
		}
		ints = append(ints, n)
	}
	return ints, nil
}

func boolSliceParser(cn *conn, n int64) (interface{}, error) {
	bools := make([]bool, 0, n)
	for i := int64(0); i < n; i++ {
		n, err := readIntReply(cn)
		if err != nil {
			return nil, err
		}
		bools = append(bools, n == 1)
	}
	return bools, nil
}

func stringSliceParser(cn *conn, n int64) (interface{}, error) {
	ss := make([]string, 0, n)
	for i := int64(0); i < n; i++ {
		s, err := readStringReply(cn)
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}
	return ss, nil
}

func floatSliceParser(cn *conn, n int64) (interface{}, error) {
	nn := make([]float64, 0, n)
	for i := int64(0); i < n; i++ {
		n, err := readFloatReply(cn)
		if err != nil {
			return nil, err
		}
		nn = append(nn, n)
	}
	return nn, nil
}

func stringStringMapParser(cn *conn, n int64) (interface{}, error) {
	m := make(map[string]string, n/2)
	for i := int64(0); i < n; i += 2 {
		key, err := readStringReply(cn)
		if err != nil {
			return nil, err
		}

		value, err := readStringReply(cn)
		if err != nil {
			return nil, err
		}

		m[key] = value
	}
	return m, nil
}

func stringIntMapParser(cn *conn, n int64) (interface{}, error) {
	m := make(map[string]int64, n/2)
	for i := int64(0); i < n; i += 2 {
		key, err := readStringReply(cn)
		if err != nil {
			return nil, err
		}

		n, err := readIntReply(cn)
		if err != nil {
			return nil, err
		}

		m[key] = n
	}
	return m, nil
}

func zSliceParser(cn *conn, n int64) (interface{}, error) {
	zz := make([]Z, n/2)
	for i := int64(0); i < n; i += 2 {
		var err error

		z := &zz[i/2]

		z.Member, err = readStringReply(cn)
		if err != nil {
			return nil, err
		}

		z.Score, err = readFloatReply(cn)
		if err != nil {
			return nil, err
		}
	}
	return zz, nil
}

func clusterSlotInfoSliceParser(cn *conn, n int64) (interface{}, error) {
	infos := make([]ClusterSlotInfo, 0, n)
	for i := int64(0); i < n; i++ {
		n, err := readArrayHeader(cn)
		if err != nil {
			return nil, err
		}
		if n < 2 {
			err := fmt.Errorf("redis: got %d elements in cluster info, expected at least 2", n)
			return nil, err
		}

		start, err := readIntReply(cn)
		if err != nil {
			return nil, err
		}

		end, err := readIntReply(cn)
		if err != nil {
			return nil, err
		}

		addrsn := n - 2
		info := ClusterSlotInfo{
			Start: int(start),
			End:   int(end),
			Addrs: make([]string, addrsn),
		}

		for i := int64(0); i < addrsn; i++ {
			n, err := readArrayHeader(cn)
			if err != nil {
				return nil, err
			}
			if n != 2 {
				return nil, fmt.Errorf("got %d elements in cluster info address, expected 2", n)
			}

			ip, err := readStringReply(cn)
			if err != nil {
				return nil, err
			}

			port, err := readIntReply(cn)
			if err != nil {
				return nil, err
			}

			info.Addrs[i] = net.JoinHostPort(ip, strconv.FormatInt(port, 10))
		}

		infos = append(infos, info)
	}
	return infos, nil
}

func newGeoLocationParser(q *GeoRadiusQuery) multiBulkParser {
	return func(cn *conn, n int64) (interface{}, error) {
		var loc GeoLocation

		var err error
		loc.Name, err = readStringReply(cn)
		if err != nil {
			return nil, err
		}
		if q.WithDist {
			loc.Dist, err = readFloatReply(cn)
			if err != nil {
				return nil, err
			}
		}
		if q.WithGeoHash {
			loc.GeoHash, err = readIntReply(cn)
			if err != nil {
				return nil, err
			}
		}
		if q.WithCoord {
			n, err := readArrayHeader(cn)
			if err != nil {
				return nil, err
			}
			if n != 2 {
				return nil, fmt.Errorf("got %d coordinates, expected 2", n)
			}

			loc.Longitude, err = readFloatReply(cn)
			if err != nil {
				return nil, err
			}
			loc.Latitude, err = readFloatReply(cn)
			if err != nil {
				return nil, err
			}
		}

		return &loc, nil
	}
}

func newGeoLocationSliceParser(q *GeoRadiusQuery) multiBulkParser {
	return func(cn *conn, n int64) (interface{}, error) {
		locs := make([]GeoLocation, 0, n)
		for i := int64(0); i < n; i++ {
			v, err := readReply(cn, newGeoLocationParser(q))
			if err != nil {
				return nil, err
			}
			switch vv := v.(type) {
			case []byte:
				locs = append(locs, GeoLocation{
					Name: string(vv),
				})
			case *GeoLocation:
				locs = append(locs, *vv)
			default:
				return nil, fmt.Errorf("got %T, expected string or *GeoLocation", v)
			}
		}
		return locs, nil
	}
}
