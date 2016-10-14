package goavro

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"github.com/sebglon/goavro/transceiver"
)

var REMOTE_HASHES map[string][]byte
var REMOTE_PROTOCOLS map[string]Protocol

var BUFFER_HEADER_LENGTH = 4
var BUFFER_SIZE = 8192 

var META_WRITER Codec
var META_READER Codec
var HANDSHAKE_REQUESTOR_READER Codec

type Requestor struct {
	// Base class for the client side of protocol interaction.
	local_protocol		Protocol
	transceiver		transceiver.Transceiver
	remote_protocol 	Protocol
	remote_hash		[]byte
	send_protocol		bool
	send_handshake		bool
}
func init() {
	var err error
	HANDSHAKE_REQUESTOR_READER, err = NewCodec(handshakeResponseshema)
	if  err!=nil {
	log.Fatal(err)	
	}
	META_WRITER, err = NewCodec(metadataSchema)
        if  err!=nil {
        log.Fatal(err)
        }
	META_READER, err = NewCodec(metadataSchema)
	if  err!=nil {
		log.Fatal(err)
	}

}

func NewRequestor(localProto Protocol, transceiver transceiver.Transceiver) *Requestor {

	r := &Requestor{
		local_protocol: localProto,
		transceiver: transceiver,
//		remote_protocol: nil,
//		remote_hash: nil,
		send_protocol: false,
		send_handshake: true,
	}
	transceiver.InitHandshake(r.write_handshake_request, r.read_handshake_response)
	return r
}


func (a *Requestor) RemoteProtocol(proto Protocol) {
	a.remote_protocol = proto
	REMOTE_PROTOCOLS[proto.Name] = proto
}

func (a *Requestor) RemoteHash(hash []byte) {
	a.remote_hash =  hash
	REMOTE_HASHES[a.remote_protocol.Name] = hash
}

func (a *Requestor) Request(message_name string, request_datum  interface{})  error {
	// wrtie a request message and reads a response or error message.
	// build handshale and call request
	frame1 := new(bytes.Buffer)
	frame2 := new(bytes.Buffer)

	err := a.write_call_requestHeader(message_name, frame1)
	if err!=nil {
		return err
	}
	err = a.write_call_request(message_name, request_datum, frame2)
	if err!=nil {
		return err
	}

	// sen the handshake and call request; block until call response
	buffer_writers := []bytes.Buffer{*frame1, *frame2}
	responses, err := a.transceiver.Transceive(buffer_writers)

	if err!=nil {
		return fmt.Errorf("Fail to transceive %v", err)
	}
	//buffer_decoder := bytes.NewBuffer(decoder)
	// process the handshake and call response

	if len(responses) >0 {
		a.read_call_responseCode(responses[1])
		if err != nil {
			return err
		}
		//	a.Request(message_name, request_datum)
	}
	return nil
}

func (a *Requestor) write_handshake_request() (handshake []byte ,err error) {
	buffer := new(bytes.Buffer)
	defer 	buffer.Write(handshake)
        local_hash :=a.local_protocol.MD5
        remote_name := a.remote_protocol.Name
	remote_hash := REMOTE_HASHES[remote_name]
        if len(remote_hash)==0  {
                remote_hash = local_hash
		a.remote_protocol = a.local_protocol
        }

        record, err := NewRecord(RecordSchema(handshakeRequestshema))
        if err != nil {
                err = fmt.Errorf("Avro fail to  init record handshakeRequest %v",err)
		return
        }

        record.Set("clientHash", local_hash)
        record.Set("serverHash", remote_hash)
	record.Set("meta", make(map[string]interface{}))
        codecHandshake, err := NewCodec(handshakeRequestshema)
        if err != nil {
               err = fmt.Errorf("Avro fail to  get codec handshakeRequest %v",err)
		return
        }

	if a.send_protocol {
		json, err := a.local_protocol.Json()
		if err!=nil {		
			return nil ,err
		}
		record.Set("clientProtocol", json)
	}



        if err = codecHandshake.Encode(buffer, record); err !=nil {
                err =  fmt.Errorf("Encode handshakeRequest ",err)
		return
        }


        return
}

func (a *Requestor) write_call_request(message_name string, request_datum interface{}, frame io.Writer) (err error) {
	codec, err := a.local_protocol.MessageRequestCodec(message_name)

	if err != nil {
		return fmt.Errorf("fail to get codec for message %s:  %v", message_name, err)
	}
	a.write_request(codec, request_datum, frame)
	return err
}

func (a *Requestor) write_call_requestHeader(message_name string, frame1 io.Writer) error {
	// The format of a call request is:
	//   * request metadata, a map with values of type bytes
	//   * the message name, an Avro string, followed by
	//   * the message parameters. Parameters are serialized according to
	//     the message's request declaration.

	// TODO request metadata (not yet implemented)
	request_metadata := make(map[string]interface{})
	// encode metadata
	if err := META_WRITER.Encode(frame1, request_metadata); err != nil {
		return fmt.Errorf("Encode metadata ", err)
	}


	stringCodec.Encode(frame1,message_name)
	return nil
} 

func (a *Requestor) write_request(request_codec Codec, request_datum interface{}, buffer io.Writer) error {


	if err := request_codec.Encode(buffer, request_datum); err != nil {
		return fmt.Errorf("Fail to encode request_datum %v", err)
	}
	return nil
}

func (a *Requestor) read_handshake_response(decoder io.Reader) (bool, error) {
	if !a.send_handshake {
		return true, nil
	}

	datum, err := HANDSHAKE_REQUESTOR_READER.Decode(decoder)
	if err != nil {

		return false,fmt.Errorf("Fail to decode %v with error %v", decoder, err)
	}

	record, ok := datum.(*Record)
	if !ok {
		return false, fmt.Errorf("Fail to decode handshake %T", datum)
	}

	var we_have_matching_schema  =false
	match, err := record.Get("match")
	if err!= nil {
		return false, err
	}
	switch match {
	case "BOTH":
		a.send_protocol  = false
		we_have_matching_schema =true
	case "CLIENT":
                err = fmt.Errorf("Handshake failure. match == CLIENT")
		if a.send_protocol {
			field , err := record.Get("serverProtocol")
		        if err!= nil {
		                return false, err
		        }
			a.remote_protocol = field.(Protocol)
			field, err =  record.Get("serverHash")
                        if err!= nil {
                                return false, err
                        }
			a.remote_hash = field.([]byte)

			a.send_protocol = false
			we_have_matching_schema = true
		}
	case "NONE":
		err = fmt.Errorf("Handshake failure. match == NONE")
		if a.send_protocol {
                        field , err := record.Get("serverProtocol")
                        if err!= nil {
                                return false, err
                        }
			a.remote_protocol = field.(Protocol)
                        field, err =  record.Get("serverHash")
                        if err!= nil {
                                return false, err
                        }
			a.remote_hash = field.([]byte)

			a.send_protocol = true
		}
	default: 
		err = fmt.Errorf("Unexpected match: #{match}")
	}

	return we_have_matching_schema, nil 
}

func (a *Requestor) read_call_responseCode(decoder io.Reader) error {
	// The format of a call response is:
	//   * response metadata, a map with values of type bytes
	//   * a one-byte error flag boolean, followed by either:
	//     * if the error flag is false,
	//       the message response, serialized per the message's response schema.
	//     * if the error flag is true,
	//       the error, serialized per the message's error union schema.
	_, err := META_READER.Decode(decoder)

	if  err != nil {
		return fmt.Errorf("Decode metadata ", err)
	}
	return nil

}


func (a *Requestor) read_call_responseMessage(message_name string, decoder io.Reader ) error {
	codec, err := a.local_protocol.MessageResponseCodec(message_name)

	if err != nil {
		return fmt.Errorf("fail to get response codec for message %s:  %v", message_name, err)
	}

	datum, err := codec.Decode(decoder);
	if err != nil {

		return fmt.Errorf("Fail to decode %v with error %v", decoder, err)
	}
	status, ok := datum.(string)
	if !ok {
		return fmt.Errorf("Fail to decode Status response %v", datum)
	}

	switch status {
	case "OK":
		err = nil
	case "FAILED":
		err = fmt.Errorf("Reponse failure. status == %v", status)

	case "UNKNOWN":
		err = fmt.Errorf("Reponse failure. match == %v", status)

	default:
		err = fmt.Errorf("Unexpected status: %v", status)
	}

	return err


}


