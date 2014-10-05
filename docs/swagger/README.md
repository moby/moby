

```
docker build -t docker-sagger-docs .

docker run -ti \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -p 8003:8003
    docker-swagger-docs
```
