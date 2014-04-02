FROM		busybox
ADD		add1	.
RUN		ls /add1 && rm /add1
WORKINDIR	/tmp
ADD		add1	.
RUN		ls ./add1 && rm ./add1
ADD		add1 	add2
RUN		ls /tmp/add2 && rm /tmp/add2
