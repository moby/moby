# Docker's remote API authorization with macaroons

Docker uses [Macaroons](http://research.google.com/pubs/pub41892.html) for authorize requests to the remote API. This article explains how to grant privileges for a remote client to interact with the API.

## Initializing the credentials in the server

The first command to run is `docker id --init`. This will create the root macaroon in the server and it will be shared with the client.

```
$ docker id --init --secret "Super secret token"
```

From that point forward, any request to the api that doesn't include the macaroon will be unauthorized. Any request that uses the root macaroon has full read/write access to the server.

## Sharing credentials

You can always see a serialized version of your macaroon by running `id --show`.

```
$ docker id --show
```

The output can be shared with anyone and they'll get access to the server according to the caveats of your macaroon.

They can use the macaroon setting the parameter `--id` in any docker command:

```
$ docker run --rm -i --id "Serialized macaroon" -t ubuntu bash
```

They can also set the macaroon as default authorization id:

```
$ docker id --set "Serialized macaroon"
```

## Creating caveats

Using the root macaroon, you can create new macaroons with caveats to share with other people. Caveats are used to restrict access to certain parts of the server.

These are the current allowed caveats:

- op: Restrict access only to certain request operations. The only allowed values are `read` and `write`.
- image_id: Restrict access only to one image. The value must be an image id present in the server.
- container_id: Restrict access only to one container. The value must be a container id present in the server.
- ip: Restrict access from only one IP. The value must be a value IPv4 or IPv6.
- expires: Restrict access for a period of time. The value must be the allowed interval in seconds. For instance, `60` to allow access for only 60 seconds.

All these caveats can be layered.

This is the command to allow only read access to only one container:

```
$ docker id --create --secret "Super secret token" --op read --container-id "container id"
```

## Removing authorizations

If for some reason you want to stop using the API's authorization, you can run the destroy command. This command requires the root macaroon to be executed:

```
$ docker id --destroy
```
