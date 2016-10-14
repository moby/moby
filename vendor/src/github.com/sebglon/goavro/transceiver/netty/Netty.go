package netty

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
	"github.com/sebglon/goavro/transceiver"
	"fmt"
	"log"
)

type NettyTransceiver struct {
	transceiver.Pool
	mu             sync.Mutex
	pending        []byte
	alreadyCalled  bool
	writeHandShake transceiver.WriteHandshake
	readHandshake  transceiver.ReadHandshake
}
func NewTransceiver(config transceiver.Config) (f* NettyTransceiver, err error){
	f = &NettyTransceiver{}
	pool , err := transceiver.NewPool(config)
	if err !=nil {
		return
	}
	f.Pool =*pool
	return
}

func (t *NettyTransceiver) InitHandshake(writer transceiver.WriteHandshake,reader transceiver.ReadHandshake ) {
	t.writeHandShake=writer
	t.readHandshake=reader
}





func (t *NettyTransceiver) Transceive(requests []bytes.Buffer) ([]io.Reader, error){
	nettyFrame := new(bytes.Buffer)
	t.Pack(nettyFrame, requests)
	log.Printf("%#v",t.Pool)
	conn, pc, err := t.Pool.Conn()

	if err!=nil {
		return nil, err
	}
	defer pc.Close()

	if !conn.IsChecked() {
		frame0 := requests[0]
		if t.writeHandShake ==nil {
			return nil, fmt.Errorf("InitHandshake not called before Transceive")
		}
		handshake, err := t.writeHandShake()
		if err!=nil {
			return nil, err
		}

		requests[0].Reset()
		_, err = requests[0].Write(append( handshake, frame0.Bytes()...))
		if err!=nil {
			return nil, err
		}
	}

	bodyBytes, err := t.Pool.Call(conn, pc, nettyFrame.Bytes())
	if err != nil {
		return nil, err
	}



	resps, err := t.Unpack(bodyBytes)
	if err != nil {
		return nil, err
	}

	if !conn.IsChecked() && len(resps)>1{
		ok, err := t.readHandshake(resps[0])
		if err!=nil {
			return nil, err
		}
		conn.Checked(ok)
		if !ok {
			return nil, fmt.Errorf("Fail to validate Handshake")
		}
		return resps[1:], nil
	} else {
		return resps, nil
	}

}

func (t *NettyTransceiver) Pack(frame *bytes.Buffer, requests []bytes.Buffer) {
	// Set Netty Serial

	nettySerial :=make([]byte, 4)
	binary.BigEndian.PutUint32(nettySerial, uint32(1))
	frame.Write(nettySerial)


	nettySizeBuffer :=make([]byte, 4)
	binary.BigEndian.PutUint32(nettySizeBuffer, uint32(len(requests)))
	frame.Write(nettySizeBuffer)

	for _, request := range requests {
		requestSize :=make([]byte, 4)
		binary.BigEndian.PutUint32(requestSize, uint32(request.Len()))
		frame.Write(requestSize)
		frame.Write(request.Bytes())
	}
}

func (t *NettyTransceiver) Unpack(frame []byte) ([]io.Reader, error) {
	nettyNumberFame := binary.BigEndian.Uint32(frame[4:8])
	result := make([]io.Reader, nettyNumberFame)
	startFrame := uint32(8)
	i:=uint32(0)
	for i < nettyNumberFame  {
		messageSize := uint32(binary.BigEndian.Uint32(frame[startFrame:startFrame+4]))
		message := frame[startFrame+4:startFrame+4+messageSize]
		startFrame = startFrame+4+messageSize
		br := bytes.NewReader(message)
		result[i] = br
		i++
	}

	return  result, nil
}
