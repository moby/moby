
FROM    ubuntu:14.04

RUN     apt-get update && apt-get install -y nginx
RUN     useradd docker

ADD     swagger-ui.tar.gz   /docs
ADD     index.html   /docs/index.html
ADD     nginx.conf  /etc/nginx.conf
ADD     specs       /docs/specs
EXPOSE  8003
CMD     nginx -c /etc/nginx.conf
