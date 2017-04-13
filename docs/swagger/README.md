


```
docker build -t docker-swagger-docs .

docker run -ti \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -p 8003:8003
    docker-swagger-docs
```

Open http://localhost:8003/
