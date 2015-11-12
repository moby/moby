# Docker Authorization Plug-in API

## Introduction

Docker authorization plug-in infrastructure enables extending the functionality of the Docker daemon with respect to user authorization. The infrastructure enables registering a set of external authorization plug-in. Each plug-in receives information about the user and the request and decides whether to allow or deny the request. Only in case all plug-ins allow accessing the resource the access is granted. 

Each plug-in operates as a separate service, and registers with Docker through general (plug-ins API) [https://blog.docker.com/2015/06/extending-docker-with-plugins/]. No Docker daemon recompilation is required in order to add / remove an authentication plug-in. Each plug-in is notified twice for each operation: 1) before the operation is performed and, 2) before the response is returned to the client. The plug-ins can modify the response that is returned to the client. 

The authorization depends on the authorization effort that takes place in parallel [https://github.com/docker/docker/issues/13697]. 

This is the official issue of the authorization effort: https://github.com/docker/docker/issues/14674

(Here)[https://github.com/rhatdan/docker-rbac] you can find an open document that discusses a default RBAC plug-in for Docker. 

## Docker daemon configuration 

In order to add a single authentication plug-in or a set of such, please use the following command line argument:

``` docker -d authz-plugin=authZPlugin1,authZPlugin2 ```

## API

The skeleton code for a typical plug-in can be found here [ADD LINK]. The plug-in must implement two AP methods:

1. */AuthzPlugin.AuthZReq* - this is the _authorize request_ method that is called before executing the Docker operation. 
1. */AuthzPlugin.AuthZRes* - this is the _authorize response_ method that is called before returning the response to the client. 

#### /AuthzPlugin.AuthZReq

**Request**:

```
{    
    "User":              "The user identification"
    "UserAuthNMethod":   "The authentication method used"
    "RequestMethod":     "The HTTP method"
    "RequestUri":        "The HTTP request URI"
    "RequestBody":       "Byte array containing the raw HTTP request body"
    "RequestHeader":     "Byte array containing the raw HTTP request header as a map[string][]string "
    "RequestStatusCode": "Request status code"
}
```

**Response**:

```
{    
    "Allow" : "Determined whether the user is allowed or not"
    "Msg":    "The authorization message"
}
```

#### /AuthzPlugin.AuthZRes

**Request**:
```
{
    "User":              "The user identification"
    "UserAuthNMethod":   "The authentication method used"
    "RequestMethod":     "The HTTP method"
    "RequestUri":        "The HTTP request URI"
    "RequestBody":       "Byte array containing the raw HTTP request body"
    "RequestHeader":     "Byte array containing the raw HTTP request header as a map[string][]string"
    "RequestStatusCode": "Request status code"
    "ResponseBody":      "Byte array containing the raw HTTP response body"
    "ResponseHeader":    "Byte array containing the raw HTTP response header as a map[string][]string"
    "ResponseStatusCode":"Response status code"
}
```

**Response**:
```
{
   "Allow" :               "Determined whether the user is allowed or not"
   "Msg":                  "The authorization message"
   "ModifiedBody":         "Byte array containing a modified body of the raw HTTP body (or nil if no changes required)"
   "ModifiedHeader":       "Byte array containing a modified header of the HTTP response (or nil if no changes required)"
   "ModifiedStatusCode":   "int containing the modified version of the status code (or 0 if not change is required)"
}
```

The modified response enables the authorization plug-in to manipulate the content of the HTTP response.
In case of more than one plug-in, each subsequent plug-in will received a response (optionally) modified by a previous plug-in. 