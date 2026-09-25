Wireshark Dissectors for NetworkDB
==================================

`memberlist.lua` is a [Wireshark][] plugin
which registers a `memberlist` protocol
that can dissect the [memberlist][] TCP and UDP protocols.
The `memberlist` protocol can be configured to dissect user data
as the protocol named in the `memberlist.userdata_dissector` preference.

`moby-networkdb.lua` is a Wireshark plugin which registers
a protocol named `networkdbgossip`
that dissects NetworkDB gossip messages.
As node-to-node communications for NetworkDB
are transported as memberlist user messages,
the memberlist protocol dissector must be configured
to invoke the networkdbgossip protocol as a sub-dissector.
(Read: set the preference `memberlist.userdata_dissector:networkdbgossip`)

Installation
------------

### Install Wireshark 4.5 or newer.
Wireshark 4.4 has an incomplete msgpack protocol dissector
that is unable to properly decode memberlist messages.
As of 2025-06-30 Wireshark 4.5 has yet to be released.
A [nightly build][ws.dl] may be required.

### Install the plugins
Configure Wireshark/Tshark to load `memberlist.lua` and `moby-networkdb.lua`.
Refer to [the Wireshark Lua manual][ws.lua.intro] for further instruction.

### Configure the ProtoBuf protocol

NetworkDB messages are serialized to protobuf,
which is not self-describing.
The ProtoBuf Wireshark protocol needs to be supplied with
the protobuf IDL definitions of the messages
in order to dissect them.

1. Clone [moby/moby][] for the NetworkDB IDL definitions.
2. Clone [protocolbuffers/protobuf][] for the protobuf "standard library" IDL.
3. Configure the ProtoBuf protocol (Preferences -> Protocols -> ProtoBuf)
   as follows:
     - ✅ Load .proto files on startup. (`protobuf.reload_protos`)
     - ✅ Dissect Protobuf fields as Wireshark fields. (`protobuf.pbf_as_hf`)
     - Add entries to the Protobuf Search Paths table (`uat:protobuf.search_paths`):
         - path/to/protocolbuffers/protobuf/src
         - path/to/moby/moby/vendor
	 - path/to/moby/moby
	 - path/to/moby/moby/libnetwork/networkdb (✅ Load all files)
	 - path/to/moby/moby/libnetwork/drivers/overlay (✅ Load all files)

Note that it is not sufficient to just grab the .proto files from the repos.
The directory structure is necessary for the definitions to load properly.

### Configure the Memberlist protocol

Configure memberlist to dissect user data messages as NetworkDB gossip.
- In Preferences -> Protocols -> MEMBERLIST,
  set the User Data Dissector to `networkdbgossip`.
  (`memberlist.userdata_dissector`)
- Optional: set Memberlist TCP+UDP port(s) (`memberlist.ports`) as needed.
  E.g. a value such as `7946,10000-10999` would be useful
  for analyzing packet captures from NetworkDB unit tests.

Usage Notes
-----------

### Encryption

The memberlist protocol dissector supports decryption
of encrypted memberlist messages
when provided with a file containing the encryption keys used.
In Preferences -> Protocols -> MEMBERLIST,
set the Encryption Key Logfile Path
(`memberlist.keylog`)
to a file containing the encryption keys.

The logfile should list the encryption keys
as hexadecimal strings, delimited by newlines.

dockerd may be configured to write the NetworkDB encryption keys to a logfile
by setting the environment variable `NETWORKDBKEYLOGFILE`
to the path where the file should reside.

### Known Issues

The NetworkDB protocol may fail to load with an error when Wireshark is first started:

    moby-networkdb.lua:4: bad argument #1 to 'new'
    (Field_new: a field with this name must exist)

This is due to [a known issue in Wireshark.](https://gitlab.com/wireshark/wireshark/-/issues/20161)

Workaround: reload Lua plugins after Wireshark has been initialized.

- Menu: Analyze -> Reload Lua Plugins, or
- Keyboard shortcut: Ctrl-Shift-L (Windows/Linux), ⇧⌘L (macOS)

### Limitations

- Only memberlist encryption version 1 (AES-GCM 128, no padding) is supported,
  not version 0 (AES-GCM 128 using PKCS#7 padding).
- Labelled messages cannot currently be decrypted.

[memberlist]: https://github.com/hashicorp/memberlist
[Wireshark]: https://wireshark.org
[ws.dl]: https://www.wireshark.org/download/automated/
[ws.lua.intro]: https://www.wireshark.org/docs/wsdg_html_chunked/wsluarm.html#wsluarm_intro
[moby/moby]: https://github.com/moby/moby
[protocolbuffers/protobuf]: https://github.com/protocolbuffers/protobuf
