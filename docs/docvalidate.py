#!/usr/bin/env python

""" I honestly don't even know how the hell this works, just use it. """
__author__ = "Scott Stamp <scott@hypermine.com>"

from HTMLParser import HTMLParser
from urlparse import urljoin
from sys import setrecursionlimit
import re
import requests

setrecursionlimit(10000)
root = 'http://localhost:8000'


class DataHolder:

    def __init__(self, value=None, attr_name='value'):
        self._attr_name = attr_name
        self.set(value)

    def __call__(self, value):
        return self.set(value)

    def set(self, value):
        setattr(self, self._attr_name, value)
        return value

    def get(self):
        return getattr(self, self._attr_name)


class Parser(HTMLParser):
    global root

    ids = set()
    crawled = set()
    anchors = {}
    pages = set()
    save_match = DataHolder(attr_name='match')

    def __init__(self, origin):
        self.origin = origin
        HTMLParser.__init__(self)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if 'href' in attrs:
            href = attrs['href']

            if re.match('^{0}|\/|\#[\S]{{1,}}'.format(root), href):
                if self.save_match(re.search('.*\#(.*?)$', href)):
                    if self.origin not in self.anchors:
                        self.anchors[self.origin] = set()
                    self.anchors[self.origin].add(
                        self.save_match.match.groups(1)[0])

                url = urljoin(root, href)

                if url not in self.crawled and not re.match('^\#', href):
                    self.crawled.add(url)
                    Parser(url).feed(requests.get(url).content)

        if 'id' in attrs:
            self.ids.add(attrs['id'])
	# explicit <a name=""></a> references
        if 'name' in attrs:
            self.ids.add(attrs['name'])


r = requests.get(root)
parser = Parser(root)
parser.feed(r.content)
for anchor in sorted(parser.anchors):
    if not re.match('.*/\#.*', anchor):
        for anchor_name in parser.anchors[anchor]:
            if anchor_name not in parser.ids:
                print 'Missing - ({0}): #{1}'.format(
                    anchor.replace(root, ''), anchor_name)
