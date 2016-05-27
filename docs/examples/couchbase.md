<!--[metadata]>
+++
title = "Dockerizing a Couchbase service"
description = "Dockerizing a Couchbase service"
keywords = ["docker, example, package installation, networking, couchbase"]
[menu.main]
parent = "engine_dockerize"
+++
<![end-metadata]-->

# Dockerizing a Couchbase service

This example shows how to start a [Couchbase](http://couchbase.com) server using Docker Compose, configure it using its [REST API](http://developer.couchbase.com/documentation/server/4.0/rest-api/rest-endpoints-all.html), and query it.

Couchbase is an open source, document-oriented NoSQL database for modern web, mobile, and IoT applications. It is designed for ease of development and Internet-scale performance.

## Start Couchbase server

Couchbase Docker images are published at [Docker Hub](https://hub.docker.com/_/couchbase/).

Start Couchbase server as:

```
docker run -d --name db -p 8091-8093:8091-8093 -p 11210:11210 couchbase
```

The purpose of each port exposed is explained at [Couchbase Developer Portal - Network Configuration](http://developer.couchbase.com/documentation/server/4.1/install/install-ports.html).

Logs can be seen as:

```
docker logs db
Starting Couchbase Server -- Web UI available at http://<ip>:8091
```

> **Note**: The examples on this page assume that the Docker Host
> is reachable on `192.168.99.100`. Substitute `192.168.99.100` with
> the actual IP address of your Docker Host.  If you're running
> Docker using Docker machine, you can obtain the IP address
> of the Docker host using `docker-machine ip <MACHINE-NAME>`.

The logs show that Couchbase console can be accessed at `http://192.168.99.100:8091`. The default username is `Administrator` and the password is `password`.

## Configure Couchbase Docker container

By default, Couchbase server needs to be configured using the console before it can be used. This can be simplified by configuring it using the REST API.

### Configure memory for Data and Index service

Data, Query and Index are three different services that can be configured on a Couchbase instance. Each service has different operating needs. For example, Query is CPU intensive operation and so requires a faster processor. Index is disk heavy and so requires a faster solid state drive. Data needs to be read/written fast and so requires more memory.

Memory needs to be configured for Data and Index service only.

```
curl -v -X POST http://192.168.99.100:8091/pools/default -d memoryQuota=300 -d indexMemoryQuota=300
* Hostname was NOT found in DNS cache
*   Trying 192.168.99.100...
* Connected to 192.168.99.100 (192.168.99.100) port 8091 (#0)
> POST /pools/default HTTP/1.1
> User-Agent: curl/7.37.1
> Host: 192.168.99.100:8091
> Accept: */*
> Content-Length: 36
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 36 out of 36 bytes
< HTTP/1.1 401 Unauthorized
< WWW-Authenticate: Basic realm="Couchbase Server Admin / REST"
* Server Couchbase Server is not blacklisted
< Server: Couchbase Server
< Pragma: no-cache
< Date: Wed, 25 Nov 2015 22:48:16 GMT
< Content-Length: 0
< Cache-Control: no-cache
<
* Connection #0 to host 192.168.99.100 left intact
```

The command shows an HTTP POST request to the REST endpoint `/pools/default`. The host is the IP address of the Docker machine. The port is the exposed port of Couchbase server. The memory and index quota for the server are passed in the request.

### Configure Data, Query, and Index services

All three services, or only one of them, can be configured on each instance. This allows different Couchbase instances to use affinities and setup services accordingly. For example, if Docker host is running a machine with solid-state drive then only Data service can be started.

```
curl -v http://192.168.99.100:8091/node/controller/setupServices -d 'services=kv%2Cn1ql%2Cindex'
* Hostname was NOT found in DNS cache
*   Trying 192.168.99.100...
* Connected to 192.168.99.100 (192.168.99.100) port 8091 (#0)
> POST /node/controller/setupServices HTTP/1.1
> User-Agent: curl/7.37.1
> Host: 192.168.99.100:8091
> Accept: */*
> Content-Length: 26
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 26 out of 26 bytes
< HTTP/1.1 200 OK
* Server Couchbase Server is not blacklisted
< Server: Couchbase Server
< Pragma: no-cache
< Date: Wed, 25 Nov 2015 22:49:51 GMT
< Content-Length: 0
< Cache-Control: no-cache
<
* Connection #0 to host 192.168.99.100 left intact
```

The command shows an HTTP POST request to the REST endpoint `/node/controller/setupServices`. The command shows that all three services are configured for the Couchbase server. The Data service is identified by `kv`, Query service is identified by `n1ql` and Index service identified by `index`.

### Setup credentials for the Couchbase server

Sets the username and password credentials that will subsequently be used for managing the Couchbase server.

```
curl -v -X POST http://192.168.99.100:8091/settings/web -d port=8091 -d username=Administrator -d password=password
* Hostname was NOT found in DNS cache
*   Trying 192.168.99.100...
* Connected to 192.168.99.100 (192.168.99.100) port 8091 (#0)
> POST /settings/web HTTP/1.1
> User-Agent: curl/7.37.1
> Host: 192.168.99.100:8091
> Accept: */*
> Content-Length: 50
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 50 out of 50 bytes
< HTTP/1.1 200 OK
* Server Couchbase Server is not blacklisted
< Server: Couchbase Server
< Pragma: no-cache
< Date: Wed, 25 Nov 2015 22:50:43 GMT
< Content-Type: application/json
< Content-Length: 44
< Cache-Control: no-cache
<
* Connection #0 to host 192.168.99.100 left intact
{"newBaseUri":"http://192.168.99.100:8091/"}
```

The command shows an HTTP POST request to the REST endpoint `/settings/web`. The user name and password credentials are passed in the request.

### Install sample data

The Couchbase server can be easily load some sample data in the Couchbase instance.

```
curl -v -u Administrator:password -X POST http://192.168.99.100:8091/sampleBuckets/install -d '["travel-sample"]'
* Hostname was NOT found in DNS cache
*   Trying 192.168.99.100...
* Connected to 192.168.99.100 (192.168.99.100) port 8091 (#0)
* Server auth using Basic with user 'Administrator'
> POST /sampleBuckets/install HTTP/1.1
> Authorization: Basic QWRtaW5pc3RyYXRvcjpwYXNzd29yZA==
> User-Agent: curl/7.37.1
> Host: 192.168.99.100:8091
> Accept: */*
> Content-Length: 17
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 17 out of 17 bytes
< HTTP/1.1 202 Accepted
* Server Couchbase Server is not blacklisted
< Server: Couchbase Server
< Pragma: no-cache
< Date: Wed, 25 Nov 2015 22:51:51 GMT
< Content-Type: application/json
< Content-Length: 2
< Cache-Control: no-cache
<
* Connection #0 to host 192.168.99.100 left intact
[]
```

The command shows an HTTP POST request to the REST endpoint `/sampleBuckets/install`. The name of the sample bucket is passed in the request.

Congratulations, you are now running a Couchbase container, fully configured using the REST API.

## Query Couchbase using CBQ

[CBQ](http://developer.couchbase.com/documentation/server/4.1/cli/cbq-tool.html), short for Couchbase Query, is a CLI tool that allows to create, read, update, and delete JSON documents on a Couchbase server. This tool is installed as part of the Couchbase Docker image.

Run CBQ tool:

```
docker run -it --link db:db couchbase cbq --engine http://db:8093
Couchbase query shell connected to http://db:8093/ . Type Ctrl-D to exit.
cbq>
```

`--engine` parameter to CBQ allows to specify the Couchbase server host and port running on the Docker host. For host, typically the host name or IP address of the host where Couchbase server is running is provided. In this case, the container name used when starting the container, `db`, can be used. `8093` port listens for all incoming queries.

Couchbase allows to query JSON documents using [N1QL](http://developer.couchbase.com/documentation/server/4.1/n1ql/n1ql-language-reference/index.html). N1QL is a comprehensive, declarative query language that brings SQL-like query capabilities to JSON documents.

Query the database by running a N1QL query:

```
cbq> select * from `travel-sample` limit 1;
{
    "requestID": "97816771-3c25-4a1d-9ea8-eb6ad8a51919",
    "signature": {
        "*": "*"
    },
    "results": [
        {
            "travel-sample": {
                "callsign": "MILE-AIR",
                "country": "United States",
                "iata": "Q5",
                "icao": "MLA",
                "id": 10,
                "name": "40-Mile Air",
                "type": "airline"
            }
        }
    ],
    "status": "success",
    "metrics": {
        "elapsedTime": "60.872423ms",
        "executionTime": "60.792258ms",
        "resultCount": 1,
        "resultSize": 300
    }
}
```

## Couchbase Web Console

[Couchbase Web Console](http://developer.couchbase.com/documentation/server/4.1/admin/ui-intro.html) is a console that allows to manage a Couchbase instance. It can be seen at:

`http://192.168.99.100:8091/`

Make sure to replace the IP address with the IP address of your Docker Machine or `localhost` if Docker is running locally.

![Couchbase Web Console](couchbase/web-console.png)
