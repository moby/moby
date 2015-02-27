# libnetwork
The Go package to manage Linux network namespaces

# Next steps
Following discussions with Solomon, next goals are:
- Provide a package which exposes functionality over a JSON API
- Provide a binary which serves that API over HTTP
- Provide a package which implements the interfaces by calling out to the HTTP
API
