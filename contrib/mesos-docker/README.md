Mesos Docker
============

So the title here is a bit of a lie.   This framework *can* run dockers but it
pretty much just runs arbitrary commands according to the redis
configuration.  The current "data" runs a sample web application which
sort of works like docker in that it's an entirely self contained
application but doesn't actually use docker because the cloud I was
testing the current version of this on is RHEL only which docker currently doesn't support.

The "stack" for this should be:
*Mesos 0.13
*Hipache
*Redis
*Zookeeper
*WebUI to manage app configuration 

Eventually it should be pretty easy to cut out Redis from the equation
and stick to zookeeper as the configuration data store.  A good plan of
attack would be to go with LUA-Zookeeper and openresty instead of
hipache.

I have gotten all this deployed and configured with chef but the
framework itself is pretty much what I am allowed to opensource at the
moment.  
