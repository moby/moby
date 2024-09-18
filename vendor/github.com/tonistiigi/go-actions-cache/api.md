# Github Actions Cache service API

User docs: https://docs.github.com/en/actions/guides/caching-dependencies-to-speed-up-workflows

API captured from: https://github.com/actions/toolkit/tree/main/packages/cache

## Authentication

Actions have access to two special environment variables `ACTIONS_CACHE_URL` and `ACTIONS_RUNTIME_TOKEN`. Inline step scripts in workflows do not see these variables. [`crazy-max/ghaction-github-runtime@v1`](https://github.com/crazy-max/ghaction-github-runtime) action can be used as a workaround if needed to expose them.

The base URL for cache API is `$ACTIONS_CACHE_URL/_apis/artifactcache/`.

`ACTIONS_RUNTIME_TOKEN` is a JWT token valid for 6h. Token is associated with repository scopes that can be readwrite or readonly. Eg. a PR has write access to its own scope but readonly access to the target branch scope.

All requests need to be authenticated with `Authorization: Bearer $ACTIONS_RUNTIME_TOKEN` .

## Query cache

### `GET /cache`

#### Query parameters:

- `keys` - comma-separated list of keys to query. Keys can be queried by prefix and do not need to match exactly. The newest record matching a prefix is returned.
- `version` - unique value that provides namespacing. The same value needs to be used on saving cache. The actual value does not seem to be significant.


#### Response

On success returns JSON object with following properties:

- `cacheKey` - full cache key used on saving (not prefix that was used in request)
- `scope` - which scope cache object belongs to
- `archiveLocation` - URL to download blob. This URL is already authenticated and does not need extra authentication with the token.

## Save cache

### `POST /caches`

Reserves a cache key and returns ID (incrementing number) that can be used for uploading cache. Once a key has been reserved, there is no way to save any other data to the same key. Subsequent requests with the same key/version will receive "already exists" error. There does not seem to be a way to discard partial save on error as well that may be problematic with crashes.

#### Request JSON object:

- `key` - Key to reserve. A prefix of this is used on query.
- `version` - Namespace that needs to match version on cache query.

#### Response JSON object:

- `cacheID` - Numeric unique ID used in next requests.


### `PATCH /caches/[cacheID]`

Uploads a chunk of data to the specified cache record. `Content-Range` headers are used to specify what range of data is being uploaded.

Request body is `application/octet-stream` raw data. Successful response is empty.

### `POST /caches/[cacheID]`

Finalizes the cache record after all data has been uploaded with `PATCH` requests. After calling this method, data becomes available for loading. 

#### Request JSON object:

- `size` - Total size of the object. Needs to match with the data that was uploaded.

Successful respone is empty.

