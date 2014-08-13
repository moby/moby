FROM	busybox

RUN	mkdir /src
WORKDIR	/src

ADD	add1	./
RUN	ls ./add1 && rm ./add1

ADD	add1	/src/
RUN	ls ./add1 && rm ./add1

ADD	add1	add2
RUN	ls /src/add2 && rm /src/add2
