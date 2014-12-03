Ceph Graph Driver Documentation
===============================

# Known issues

 - At the moment it is *not* possible to statically compile docker with
   the ceph graph driver enabled because some static libraries are missing
   (Ubuntu 14.04). Please use: ./hack/make.sh dynbinary

 - Using "rbd cache = true" in the "[client]" section of ceph.conf is known
   to segfault the driver. Reference and fix:
   http://tracker.ceph.com/issues/8912

 - Using glibc-2.20 could result in a SIGILL that kills the docker daemon
   process. Reference and fix:
   https://github.com/ceph/ceph/pull/2937
