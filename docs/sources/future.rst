=================
Future Directions
=================

|docker| is still a work in progress and while it's quite usable for certain
use cases, there are some that are not implemented yet.

.. warning::

    The use cases listed here are not currently supported by |docker|.

    They are listed here to steer direction for the community and hint
    potential users of what's to come.

.. contents:: Use Cases
   :local:

Docker Builder
==============

(Copied verbatim from irc channel)

::

    APPNAME=helloflask; REV=master; docker build shykes/$APPNAME:$REV --from shykes/pybuilder buildapp http://github.com/shykes/webapp/archive/$REV.tar.gz

::

    docker build sa2ajj/postgres --from base:ubuntu --cmd apt-get install -y postgresql-server

.. |docker| replace:: ``docker``
