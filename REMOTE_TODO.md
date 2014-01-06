```
**GET**                                                                 
                                                                        send objects            deprecate       multi-stream
TODO    "/events":                              getEvents,              N
ok      "/info":                                getInfo,                1
ok      "/version":                             getVersion,             1
...     "/images/json":                         getImagesJSON,          N
TODO    "/images/viz":                          getImagesViz,           0                       yes
TODO    "/images/search":                       getImagesSearch,        N
TODO    "/images/{name:.*}/get":                getImagesGet,           0
TODO    "/images/{name:.*}/history":            getImagesHistory,       1
TODO    "/images/{name:.*}/json":               getImagesByName,        1
TODO    "/containers/ps":                       getContainersJSON,      N
TODO    "/containers/json":                     getContainersJSON,      1
ok      "/containers/{name:.*}/export":         getContainersExport,    0
TODO    "/containers/{name:.*}/changes":        getContainersChanges,   1
TODO    "/containers/{name:.*}/json":           getContainersByName,    1
TODO    "/containers/{name:.*}/top":            getContainersTop,       N
TODO    "/containers/{name:.*}/attach/ws":      wsContainersAttach,     0                                       yes

**POST**
TODO    "/auth":                                postAuth,               0                       yes
ok      "/commit":                              postCommit,             0
TODO    "/build":                               postBuild,              0                       yes
TODO    "/images/create":                       postImagesCreate,       N                       yes             yes (pull)
TODO    "/images/{name:.*}/insert":             postImagesInsert,       N                       yes             yes
TODO    "/images/load":                         postImagesLoad,         1                                       yes (stdin)
TODO    "/images/{name:.*}/push":               postImagesPush,         N                                       yes
ok      "/images/{name:.*}/tag":                postImagesTag,          0
ok      "/containers/create":                   postContainersCreate,   0
ok      "/containers/{name:.*}/kill":           postContainersKill,     0
TODO    "/containers/{name:.*}/restart":        postContainersRestart,  0
ok      "/containers/{name:.*}/start":          postContainersStart,    0
ok      "/containers/{name:.*}/stop":           postContainersStop,     0
ok      "/containers/{name:.*}/wait":           postContainersWait,     0
ok      "/containers/{name:.*}/resize":         postContainersResize,   0
TODO    "/containers/{name:.*}/attach":         postContainersAttach,   0                                       yes
TODO    "/containers/{name:.*}/copy":           postContainersCopy,     0                       yes

**DELETE**
#3180   "/containers/{name:.*}":                deleteContainers,       0
TODO    "/images/{name:.*}":                    deleteImages,           N

**OPTIONS**
ok      "":                                     optionsHandler,         0
```