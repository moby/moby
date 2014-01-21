```
**GET**                                                                 
                                                                        send objects            deprecate       multi-stream
TODO    "/events":                              getEvents,              N
ok      "/info":                                getInfo,                1
ok      "/version":                             getVersion,             1
ok      "/images/json":                         getImagesJSON,          N
ok      "/images/viz":                          getImagesViz,           0                       yes
ok      "/images/search":                       getImagesSearch,        N
ok      "/images/{name:.*}/get":                getImagesGet,           0
ok      "/images/{name:.*}/history":            getImagesHistory,       N
#3621   "/images/{name:.*}/json":               getImagesByName,        1
TODO    "/containers/ps":                       getContainersJSON,      N
TODO    "/containers/json":                     getContainersJSON,      1
ok      "/containers/{name:.*}/export":         getContainersExport,    0
ok      "/containers/{name:.*}/changes":        getContainersChanges,   N
#3621   "/containers/{name:.*}/json":           getContainersByName,    1
ok      "/containers/{name:.*}/top":            getContainersTop,       N
ok      "/containers/{name:.*}/attach/ws":      wsContainersAttach,     0                                       yes

**POST**
TODO    "/auth":                                postAuth,               0                       yes
ok      "/commit":                              postCommit,             0
TODO    "/build":                               postBuild,              0                       yes
TODO    "/images/create":                       postImagesCreate,       N                       yes             yes (pull)
ok      "/images/{name:.*}/insert":             postImagesInsert,       N                       yes             yes
ok      "/images/load":                         postImagesLoad,         1                                       yes (stdin)
TODO    "/images/{name:.*}/push":               postImagesPush,         N                                       yes
ok      "/images/{name:.*}/tag":                postImagesTag,          0
ok      "/containers/create":                   postContainersCreate,   0
ok      "/containers/{name:.*}/kill":           postContainersKill,     0
ok      "/containers/{name:.*}/restart":        postContainersRestart,  0
ok      "/containers/{name:.*}/start":          postContainersStart,    0
ok      "/containers/{name:.*}/stop":           postContainersStop,     0
ok      "/containers/{name:.*}/wait":           postContainersWait,     0
ok      "/containers/{name:.*}/resize":         postContainersResize,   0
ok      "/containers/{name:.*}/attach":         postContainersAttach,   0                                       yes
ok   "/containers/{name:.*}/copy":           postContainersCopy,     0                       yes

**DELETE**
ok      "/containers/{name:.*}":                deleteContainers,       0
#3645   "/images/{name:.*}":                    deleteImages,           N

**OPTIONS**
ok      "":                                     optionsHandler,         0
```
