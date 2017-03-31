# Docker's remote API authorization with macaroons

Docker uses [Macaroons](http://research.google.com/pubs/pub41892.html) for authorize requests to the remote API. This article explains how to grant privileges for a remote client to interact with the API.

## Initializing the credentials in the server

The first command to run is `docker id --init`. This creates the root macaroon in the server and it will be shared with the client.

```
$ docker id --init --secret "Super secret token"
```

From that point forward, any request to the api that doesn't include the macaroon will be unauthorized.

The macaroon is stored in the client in its serialized format. It can look like this:

```
eyJjYXZlYXRzIjpbXSwibG9jYXRpb24iOiJ0aGUgY2xvdWQiLCJpZGVudGlmaWVyIjoiZG9ja2VyIiwic2lnbmF0dXJlIjoiNzE4ZGJkZTQwZDliZGFhNjM4MTUzOTQxZTA3MjY4YjlhYTkzYzk1ODIyMzVlMGI4ZWM0NDgzZGQ4ZmVkNTMyZiJ9
```

Deserializing the macaroon we can see more details about the current authorization credentials:

```json
{
  "caveats":[],
  "location":"the cloud",
  "identifier":"docker",
  "signature":"718dbde40d9bdaa638153941e07268b9aa93c9582235e0b8ec4483dd8fed532f"
}
```

Any client in possession of this macaroon will have full read/write access, since we're not including any caveat.

## Sharing credentials

You can always see a serialized version of your macaroon by running `docker id --show`.

```
$ docker id --show
eyJjYXZlYXRzIjpbXSwibG9jYXRpb24iOiJ0aGUgY2xvdWQiLCJpZGVudGlmaWVyIjoiZG9ja2VyIiwic2lnbmF0dXJlIjoiNzE4ZGJkZTQwZDliZGFhNjM4MTUzOTQxZTA3MjY4YjlhYTkzYzk1ODIyMzVlMGI4ZWM0NDgzZGQ4ZmVkNTMyZiJ9
```

You can share your serialized macaroon with any other client that needs access to the docker api. There are two ways to tell the docker client to use a specific macaroon.

1. Set the macaroon globally for every request using `docker id --set`:

```
$ docker id --set "Serialized macaroon"
```

2. Set the macaroon via an environment variable for specific usage:

```
$ DOCKER_REMOTE_AUTH="Serialized macaroon" \
> docker run --rm -it ubuntu bash
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
$ docker id --create --secret "Super secret token" --op read --container-id ff7da5edfaba
```

This command gives you the serialized version of a new macaroon:

```
eyJjYXZlYXRzIjpbeyJjaWQiOiJvcD1yZWFkIn0seyJjaWQiOiJjb250YWluZXJfaWQ9ZmY3ZGE1ZWRmYWJhIn1dLCJsb2NhdGlvbiI6InRoZSBjbG91ZCIsImlkZW50aWZpZXIiOiJkb2NrZXIiLCJzaWduYXR1cmUiOiJlZTZmZDkyYzJjNDc1ZmM3Mzg1NjNkNjdmZmY1N2NkNzVlOTQyMDlmYTA4ZTdlMmQxOTM5ZDA1MWYxNzFlZTUyIn0=
```

If we deserialize it, we can see that the macaroon includes two caveats:

```json
{
  "caveats":[{"cid":"op=read"},{"cid":"container_id=ff7da5edfaba"}],
  "location":"the cloud",
  "identifier":"docker",
  "signature":"ee6fd92c2c475fc738563d67fff57cd75e94209fa08e7e2d1939d051f171ee52"
}
```

Any client using it will only be able to perform read operations in that container, the rest of them will be unauthorized.

## Removing authorizations

If for some reason you want to stop using the API's authorization, you can run the destroy command. This command requires the root macaroon to be executed:

```
$ docker id --destroy
```
