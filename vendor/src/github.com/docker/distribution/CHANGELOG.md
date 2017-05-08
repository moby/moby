# Changelog

## 2.5.0 (2016-06-14)

### Storage
- Ensure uploads directory is cleaned after upload is commited
- Add ability to cap concurrent operations in filesystem driver
- S3: Add 'us-gov-west-1' to the valid region list
- Swift: Handle ceph not returning Last-Modified header for HEAD requests
- Add redirect middleware

#### Registry
- Add support for blobAccessController middleware
- Add support for layers from foreign sources
- Remove signature store
- Add support for Let's Encrypt
- Correct yaml key names in configuration

#### Client
- Add option to get content digest from manifest get

#### Spec
- Update the auth spec scope grammar to reflect the fact that hostnames are optionally supported
- Clarify API documentation around catalog fetch behavior

### API
- Support returning HTTP 429 (Too Many Requests)

### Documentation
- Update auth documentation examples to show "expires in" as int

### Docker Image
- Use Alpine Linux as base image


