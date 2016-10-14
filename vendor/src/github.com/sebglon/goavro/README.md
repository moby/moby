# goavro

## Description

Goavro is a golang library that implements encoding and decoding of
Avro data. It provides an interface to encode data directly to
`io.Writer` streams, and decoding data from `io.Reader`
streams. Goavro fully adheres to
[version 1.7.7 of the Avro specification](http://avro.apache.org/docs/1.7.7/spec.html).

## Resources

* [Avro CLI Examples](https://github.com/miguno/avro-cli-examples)
* [Avro](http://avro.apache.org/)
* [Google Snappy](https://code.google.com/p/snappy/)
* [JavaScript Object Notation, JSON](http://www.json.org/)
* [Kafka](http://kafka.apache.org)

## Usage

Documentation is available via
[![GoDoc](https://godoc.org/github.com/linkedin/goavro?status.svg)](https://godoc.org/github.com/linkedin/goavro).

Please see the example programs in the `examples` directory for
reference.

Although the Avro specification defines the terms reader and writer as
library components which read and write Avro data, Go has particular
strong emphasis on what a Reader and Writer are. Namely, it is bad
form to define an interface which shares the same name but uses a
different method signature. In other words, all Reader interfaces
should effectively mirror an `io.Reader`, and all Writer interfaces
should mirror an `io.Writer`. Adherence to this standard is essential
to keep libraries easy to use.

An `io.Reader` reads data from the stream specified at object creation
time into the parameterized slice of bytes and returns both the number
of bytes read and an error. An Avro reader also reads from a stream,
but it is possible to create an Avro reader that can read from one
stream, then read from another, using the same compiled schema. In
other words, an Avro reader puts the schema first, whereas an
`io.Reader` puts the stream first.

To support an Avro reader being able to read from multiple streams,
its API must be different and incompatible with `io.Reader` interface
from the Go standard. Instead, an Avro reader looks more like the
Unmarshal functionality provided by the Go `encoding/json` library.

### Codec interface

Creating a `goavro.Codec` is fast, but ought to be performed exactly
once per Avro schema to process. Once a `Codec` is created, it may be
used multiple times to either decode or encode data.

The `Codec` interface exposes two methods, one to encode data and one
to decode data. They encode directly into an `io.Writer`, and decode
directly from an `io.Reader`.

A particular `Codec` can work with only one Avro schema. However,
there is no practical limit to how many `Codec`s may be created and
used in a program. Internally a `goavro.codec` is merely a namespace
and two function pointers to decode and encode data. Because `codec`s
maintain no state, the same `Codec` can be concurrently used on
different `io` streams as desired.

```Go
    func (c *codec) Decode(r io.Reader) (interface{}, error)
    func (c *codec) Encode(w io.Writer, datum interface{}) error
```

#### Creating a `Codec`

The below is an example of creating a `Codec` from a provided JSON
schema. `Codec`s do not maintain any internal state, and may be used
multiple times on multiple `io.Reader`s, `io.Writer`s, concurrently if
desired.

```Go
    someRecordSchemaJson := `{"type":"record","name":"Foo","fields":[{"name":"field1","type":"int"},{"name":"field2","type":"string","default":"happy"}]}`
    codec, err := goavro.NewCodec(someRecordSchemaJson)
    if err != nil {
        return nil, err
    }
```

#### Decoding data

The below is a simplified example of decoding binary data to be read
from an `io.Reader` into a single datum using a previously compiled
`Codec`. The `Decode` method of the `Codec` interface may be called
multiple times, each time on the same or on different `io.Reader`
objects.

```Go
    // uses codec created above, and an io.Reader, definition not shown
    datum, err := codec.Decode(r)
    if err != nil {
        return nil, err
    }
```

#### Encoding data

The below is a simplified example of encoding a single datum into the
Avro binary format using a previously compiled `Codec`. The `Encode`
method of the `Codec` interface may be called multiple times, each
time on the same or on different `io.Writer` objects.

```Go
    // uses codec created above, an io.Writer, definition not shown,
    // and some data
    err := codec.Encode(w, datum)
    if err != nil {
        return nil, err
    }
```

Another example, this time leveraging `bufio.Writer`:

```Go
    // Encoding data using bufio.Writer to buffer the writes
    // during data encoding:

    func encodeWithBufferedWriter(c Codec, w io.Writer, datum interface{}) error {
        bw := bufio.NewWriter(w)
        err := c.Encode(bw, datum)
        if err != nil {
            return err
        }
        return bw.Flush()
    }

    err := encodeWithBufferedWriter(codec, w, datum)
    if err != nil {
        return nil, err
    }
```

### Reader and Writer helper types

The `Codec` interface provides means to encode and decode any Avro
data, but a number of additional helper types are provided to handle
streaming of Avro data.

See the example programs `examples/file/reader.go` and
`examples/file/writer.go` for more context:

This example wraps the provided `io.Reader` in a `bufio.Reader` and
dumps the data to standard output.

```Go
func dumpReader(r io.Reader) {
	fr, err := goavro.NewReader(goavro.BufferFromReader(r))
	if err != nil {
		log.Fatal("cannot create Reader: ", err)
	}
	defer func() {
		if err := fr.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	for fr.Scan() {
		datum, err := fr.Read()
		if err != nil {
			log.Println("cannot read datum: ", err)
			continue
		}
		fmt.Println(datum)
	}
}
```

This example buffers the provided `io.Writer` in a `bufio.Writer`, and
writes some data to the stream.

```Go
func makeSomeData(w io.Writer) error {
    recordSchema := `
    {
      "type": "record",
      "name": "example",
      "fields": [
        {
          "type": "string",
          "name": "username"
        },
        {
          "type": "string",
          "name": "comment"
        },
        {
          "type": "long",
          "name": "timestamp"
        }
      ]
    }
    `
	fw, err := goavro.NewWriter(
		goavro.BlockSize(13), // example; default is 10
		goavro.Compression(goavro.CompressionSnappy), // default is CompressionNull
		goavro.WriterSchema(recordSchema),
		goavro.ToWriter(w))
	if err != nil {
		log.Fatal("cannot create Writer: ", err)
	}
	defer fw.Close()

    // make a record instance using the same schema
	someRecord, err := goavro.NewRecord(goavro.RecordSchema(recordSchema))
	if err != nil {
		log.Fatal(err)
	}
	// identify field name to set datum for
	someRecord.Set("username", "Aquaman")
	someRecord.Set("comment", "The Atlantic is oddly cold this morning!")
	// you can fully qualify the field name
	someRecord.Set("com.example.timestamp", int64(1082196484))
    fw.Write(someRecord)

    // make another record
	someRecord, err = goavro.NewRecord(goavro.RecordSchema(recordSchema))
	if err != nil {
		log.Fatal(err)
	}
	someRecord.Set("username", "Batman")
	someRecord.Set("comment", "Who are all of these crazies?")
	someRecord.Set("com.example.timestamp", int64(1427383430))
    fw.Write(someRecord)
}
```

## Limitations

Goavro is a fully featured encoder and decoder of binary Avro data. It
fully supports recursive data structures, unions, and namespacing. It
does have a few limitations that have yet to be implemented.

### Aliases

The Avro specification allows an implementation to optionally map a
writer's schema to a reader's schema using aliases. Although goavro
can compile schemas with aliases, it does not yet implement this
feature.

### JSON Encoding

The Avro Data Serialization format describes two encodings: binary and
JSON. Goavro only implements binary encoding of data streams, because
that is what most applications need.

> Most applications will use the binary encoding, as it is smaller and
> faster. But, for debugging and web-based applications, the JSON
> encoding may sometimes be appropriate.

Note that data schemas are always encoded using JSON, as per the
specification.

### Kafka Streams

[Kakfa](http://kafka.apache.org) is the reason goavro was
written. Similar to Avro Object Container Files being a layer of
abstraction above Avro Data Serialization format, Kafka's use of Avro
is a layer of abstraction that also sits above Avro Data Serialization
format, but has its own schema. Like Avro Object Container Files, this
has been implemented but removed until the API can be improved.

### Default maximum length of `String` and `Bytes` fields

Because the way we currently decode String and Bytes fields is entirely
stateless an Avro file could specify that a String or Bytes field is
extremely large and there would be no way for the decode function to know
anything was wrong. Instead of checking the available system memory on
every decode operation, we've instead decided to opt for what we believe
to be a sane default (`math.MaxInt32` or ~2.2GB) but leave that variable exported so that a user
can change the variable if they need to exceed this limit.

## License

### Goavro license

Copyright 2015 LinkedIn Corp. Licensed under the Apache License,
Version 2.0 (the "License"); you may not use this file except in
compliance with the License.  You may obtain a copy of the License at
[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).

Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied.Copyright [201X] LinkedIn Corp. Licensed under the Apache
License, Version 2.0 (the "License"); you may not use this file except
in compliance with the License.  You may obtain a copy of the License
at
[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0).

Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied.

### Google Snappy license

Copyright (c) 2011 The Snappy-Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## Third Party Dependencies

### Google Snappy

Goavro links with [Google Snappy](https://code.google.com/p/snappy/)
to provide Snappy compression and decompression support.
