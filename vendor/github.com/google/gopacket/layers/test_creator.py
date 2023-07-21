#!/usr/bin/python
# Copyright 2012 Google, Inc. All rights reserved.

"""TestCreator creates test templates from pcap files."""

import argparse
import base64
import glob
import re
import string
import subprocess
import sys


class Packet(object):
  """Helper class encapsulating packet from a pcap file."""

  def __init__(self, packet_lines):
    self.packet_lines = packet_lines
    self.data = self._DecodeText(packet_lines)

  @classmethod
  def _DecodeText(cls, packet_lines):
    packet_bytes = []
    # First line is timestamp and stuff, skip it.
    # Format: 0x0010:  0000 0020 3aff 3ffe 0000 0000 0000 0000  ....:.?.........

    for line in packet_lines[1:]:
      m = re.match(r'\s+0x[a-f\d]+:\s+((?:[\da-f]{2,4}\s)*)', line, re.IGNORECASE)
      if m is None: continue
      for hexpart in m.group(1).split():
        packet_bytes.append(base64.b16decode(hexpart.upper()))
    return ''.join(packet_bytes)

  def Test(self, name, link_type):
    """Yields a test using this packet, as a set of lines."""
    yield '// testPacket%s is the packet:' % name
    for line in self.packet_lines:
      yield '//   ' + line
    yield 'var testPacket%s = []byte{' % name
    data = list(self.data)
    while data:
      linebytes, data = data[:16], data[16:]
      yield ''.join(['\t'] + ['0x%02x, ' % ord(c) for c in linebytes])
    yield '}'
    yield 'func TestPacket%s(t *testing.T) {' % name
    yield '\tp := gopacket.NewPacket(testPacket%s, LinkType%s, gopacket.Default)' % (name, link_type)
    yield '\tif p.ErrorLayer() != nil {'
    yield '\t\tt.Error("Failed to decode packet:", p.ErrorLayer().Error())'
    yield '\t}'
    yield '\tcheckLayers(p, []gopacket.LayerType{LayerType%s, FILL_ME_IN_WITH_ACTUAL_LAYERS}, t)' % link_type
    yield '}'
    yield 'func BenchmarkDecodePacket%s(b *testing.B) {' % name
    yield '\tfor i := 0; i < b.N; i++ {'
    yield '\t\tgopacket.NewPacket(testPacket%s, LinkType%s, gopacket.NoCopy)' % (name, link_type)
    yield '\t}'
    yield '}'



def GetTcpdumpOutput(filename):
  """Runs tcpdump on the given file, returning output as string."""
  return subprocess.check_output(
      ['tcpdump', '-XX', '-s', '0', '-n', '-r', filename])


def TcpdumpOutputToPackets(output):
  """Reads a pcap file with TCPDump, yielding Packet objects."""
  pdata = []
  for line in output.splitlines():
    if line[0] not in string.whitespace and pdata:
      yield Packet(pdata)
      pdata = []
    pdata.append(line)
  if pdata:
    yield Packet(pdata)


def main():
  class CustomHelpFormatter(argparse.ArgumentDefaultsHelpFormatter):
    def _format_usage(self, usage, actions, groups, prefix=None):
      header =('TestCreator creates gopacket tests using a pcap file.\n\n'
               'Tests are written to standard out... they can then be \n'
               'copied into the file of your choice and modified as \n'
               'you see.\n\n')
      return header + argparse.ArgumentDefaultsHelpFormatter._format_usage(
        self, usage, actions, groups, prefix)

  parser = argparse.ArgumentParser(formatter_class=CustomHelpFormatter)
  parser.add_argument('--link_type', default='Ethernet', help='the link type (default: %(default)s)')
  parser.add_argument('--name', default='Packet%d', help='the layer type, must have "%d" inside it')
  parser.add_argument('files', metavar='file.pcap', type=str, nargs='+', help='the files to process')

  args = parser.parse_args()

  for arg in args.files:
    for path in glob.glob(arg):
      for i, packet in enumerate(TcpdumpOutputToPackets(GetTcpdumpOutput(path))):
        print '\n'.join(packet.Test(
          args.name % i, args.link_type))

if __name__ == '__main__':
    main()
