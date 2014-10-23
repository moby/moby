FROM busybox
RUN echo 'dockerio:x:1001:1001::/bin:/bin/false' >> /etc/passwd
RUN echo 'dockerio:x:1001:' >> /etc/group
RUN mkdir /exists
RUN touch /exists/exists_file
RUN chown -R dockerio.dockerio /exists
COPY test_file1 test_file2 /exists/
ADD test_file3 test_file4 https://docker.com/robots.txt /exists/
RUN [ $(ls -l / | grep exists | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
RUN [ $(ls -l /exists/test_file1 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/test_file2 | awk '{print $3":"$4}') = 'root:root' ]

RUN [ $(ls -l /exists/test_file3 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/test_file4 | awk '{print $3":"$4}') = 'root:root' ]
RUN [ $(ls -l /exists/robots.txt | awk '{print $3":"$4}') = 'root:root' ]

RUN [ $(ls -l /exists/exists_file | awk '{print $3":"$4}') = 'dockerio:dockerio' ]
