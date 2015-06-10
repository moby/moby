// Copyright 2012 SocialCode. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package gelf

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewWriter(t *testing.T) {
	w, err := NewWriter("")
	if err == nil || w != nil {
		t.Errorf("New didn't fail")
		return
	}
}

func sendAndRecv(msgData string, compress CompressType) (*Message, error) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}

	w, err := NewWriter(r.Addr())
	if err != nil {
		return nil, fmt.Errorf("NewWriter: %s", err)
	}
	w.CompressionType = compress

	if _, err = w.Write([]byte(msgData)); err != nil {
		return nil, fmt.Errorf("w.Write: %s", err)
	}

	return r.ReadMessage()
}

func sendAndRecvMsg(msg *Message, compress CompressType) (*Message, error) {
	r, err := NewReader("127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("NewReader: %s", err)
	}

	w, err := NewWriter(r.Addr())
	if err != nil {
		return nil, fmt.Errorf("NewWriter: %s", err)
	}
	w.CompressionType = compress

	if err = w.WriteMessage(msg); err != nil {
		return nil, fmt.Errorf("w.Write: %s", err)
	}

	return r.ReadMessage()
}

// tests single-message (non-chunked) messages that are split over
// multiple lines
func TestWriteSmallMultiLine(t *testing.T) {
	for _, i := range []CompressType{CompressGzip, CompressZlib} {
		msgData := "awesomesauce\nbananas"

		msg, err := sendAndRecv(msgData, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != "awesomesauce" {
			t.Errorf("msg.Short: expected %s, got %s", msgData, msg.Full)
			return
		}

		if msg.Full != msgData {
			t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
			return
		}
	}
}

// tests single-message (non-chunked) messages that are a single line long
func TestWriteSmallOneLine(t *testing.T) {
	msgData := "some awesome thing\n"
	msgDataTrunc := msgData[:len(msgData)-1]

	msg, err := sendAndRecv(msgData, CompressGzip)
	if err != nil {
		t.Errorf("sendAndRecv: %s", err)
		return
	}

	// we should remove the trailing newline
	if msg.Short != msgDataTrunc {
		t.Errorf("msg.Short: expected %s, got %s",
			msgDataTrunc, msg.Short)
		return
	}

	if msg.Full != "" {
		t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
		return
	}

	fileExpected := "/go-gelf/gelf/writer_test.go"
	if !strings.HasSuffix(msg.Extra["_file"].(string), fileExpected) {
		t.Errorf("msg.File: expected %s, got %s", fileExpected,
			msg.Extra["_file"].(string))
		return
	}

	if len(msg.Extra) != 2 {
		t.Errorf("extra fields in %v (expect only file and line)", msg.Extra)
		return
	}
}

func TestGetCaller(t *testing.T) {
	file, line := getCallerIgnoringLogMulti(1000)
	if line != 0 || file != "???" {
		t.Errorf("didn't fail 1 %s %d", file, line)
		return
	}

	file, _ = getCaller(0)
	if !strings.HasSuffix(file, "/gelf/writer_test.go") {
		t.Errorf("not writer_test.go 1? %s", file)
	}

	file, _ = getCallerIgnoringLogMulti(0)
	if !strings.HasSuffix(file, "/gelf/writer_test.go") {
		t.Errorf("not writer_test.go 2? %s", file)
	}
}

// tests single-message (chunked) messages
func TestWriteBigChunked(t *testing.T) {
	randData := make([]byte, 4096)
	if _, err := rand.Read(randData); err != nil {
		t.Errorf("cannot get random data: %s", err)
		return
	}
	msgData := "awesomesauce\n" + base64.StdEncoding.EncodeToString(randData)

	for _, i := range []CompressType{CompressGzip, CompressZlib} {
		msg, err := sendAndRecv(msgData, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != "awesomesauce" {
			t.Errorf("msg.Short: expected %s, got %s", msgData, msg.Full)
			return
		}

		if msg.Full != msgData {
			t.Errorf("msg.Full: expected %s, got %s", msgData, msg.Full)
			return
		}
	}
}

// tests messages with extra data
func TestExtraData(t *testing.T) {

	// time.Now().Unix() seems fine, UnixNano() won't roundtrip
	// through string -> float64 -> int64
	extra := map[string]interface{}{
		"_a":    10 * time.Now().Unix(),
		"C":     9,
		"_file": "writer_test.go",
		"_line": 186,
	}

	short := "quick"
	full := short + "\nwith more detail"
	m := Message{
		Version:  "1.0",
		Host:     "fake-host",
		Short:    string(short),
		Full:     string(full),
		TimeUnix: float64(time.Now().Unix()),
		Level:    6, // info
		Facility: "writer_test",
		Extra:    extra,
	}

	for _, i := range []CompressType{CompressGzip, CompressZlib} {
		msg, err := sendAndRecvMsg(&m, i)
		if err != nil {
			t.Errorf("sendAndRecv: %s", err)
			return
		}

		if msg.Short != short {
			t.Errorf("msg.Short: expected %s, got %s", short, msg.Full)
			return
		}

		if msg.Full != full {
			t.Errorf("msg.Full: expected %s, got %s", full, msg.Full)
			return
		}

		if len(msg.Extra) != 3 {
			t.Errorf("extra extra fields in %v", msg.Extra)
			return
		}

		if int64(msg.Extra["_a"].(float64)) != extra["_a"].(int64) {
			t.Errorf("_a didn't roundtrip (%v != %v)", int64(msg.Extra["_a"].(float64)), extra["_a"].(int64))
			return
		}

		if string(msg.Extra["_file"].(string)) != extra["_file"] {
			t.Errorf("_file didn't roundtrip (%v != %v)", msg.Extra["_file"].(string), extra["_file"].(string))
			return
		}

		if int(msg.Extra["_line"].(float64)) != extra["_line"].(int) {
			t.Errorf("_line didn't roundtrip (%v != %v)", int(msg.Extra["_line"].(float64)), extra["_line"].(int))
			return
		}
	}
}
