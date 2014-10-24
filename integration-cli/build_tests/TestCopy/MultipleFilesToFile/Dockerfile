FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN chown -R dockerio.dockerio /exists
COPY test_file1 /exists/
ADD test_file2 test_file3 /exists/test_file1
