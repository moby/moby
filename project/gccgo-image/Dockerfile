FROM  ubuntu:14.04

RUN apt-get -y update
RUN apt-get -y install build-essential libgmp-dev libgmp3-dev libmpfr-dev libmpc-dev flex subversion curl

WORKDIR /work
RUN svn co -q --revision 219661 svn://gcc.gnu.org/svn/gcc/trunk src

WORKDIR /work/bld
RUN ../src/configure --enable-threads=posix --enable-shared --enable-__cxa_atexit --enable-languages=c,c++,go --enable-secureplt --enable-checking=yes --with-long-double-128 --enable-decimal-float --disable-bootstrap --disable-alsa --disable-multilib --prefix=/usr/local/gccgo
RUN make
RUN make install
RUN mkdir /usr/local/gccgo/src
RUN cp -R ../src/libgo/go /usr/local/gccgo/src/pkg
ENV PATH /usr/local/gccgo/bin:$PATH
RUN echo /usr/local/gccgo/lib64 > /etc/ld.so.conf.d/gccgo.conf
RUN ldconfig

WORKDIR /
