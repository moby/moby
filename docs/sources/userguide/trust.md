# Docker Trust
Docker uses a trust system based on public key cryptography and a global
federated namespace to link users to keys and resources. Users never need
to share their private keys and retain full control of extended permissions.

## Trust CLI
To manage the trust system on the command line, a tool called `trust` may be
used. Trust has subcommands to manage the various life cycles of keys and ACLs.
The tool will be able to interface with remote key servers to manage a local
copy of a globally distributed set of keys and ACLs in the namespace. The cli
is configurable to work with application specific trust and should be able to
directly manage trust information used by an application using *libtrust*.

### Key Registration
The `trust register` command can be used to register public keys with the trust
system. By default every client and daemon instance of Docker will generate a
public key and using `trust register` will register that public key with the
credentials provided on login. With no additional arguments `trust register`
will register against a default trust server using basic authentication. 
Docker Hub is the built in default using Docker Hub credentials, but a trust server
URL may also be provided to register with.  The register command also supports
setting a limited set of permission to the key, to keep the key from being able
to perform any action on the users behalf.  The default permission is to be
able to delegate any action, which includes extending permissions to other
users.

After registering, the key identifier registered with trust server will be stored
in the local Docker configuration file (`~/.dockercfg`). The trust server will
return the grant created and added to the global trust graph by the server . The
grant will contain the key id, user namespace, and the actions delegated to the
key.

### Grant
The `trust grant` command can be used to extend permission in the trust
graph.  The `--delegate` flag can be used to allow the grant to be used to
create further grants.  The `--grantee` flag is used to set the grantee which
will be given permission.  The `--subject` flag is used to specify what 
element within the namespace the grantee is given permission to.

~~~~
# Grants user jlhawn access to push to any repository under dmcgowan
# as well as extend that permission to others
trust grant --delegate --grantee jlhawn --subject dmcgowan push

# Grants user jlhawn access to push to the repository name
# dmcgowan/my-app
trust grant --grantee jlhawn --subject dmcgowan/my-app push

# Grants user jlhawn access to push and pull to the repository name
# dmcgowan/my-app
trust grant --grantee jlhawn --subject dmcgowan/my-app push pull

# Effectively giving jlhawn full access to the dmcgowan namespace
trust grant --delegate --grantee jlhawn --subject dmcgowan any
~~~~

## Trust Graph

### Terminology
- **delegation** Extension of identity between two entities which would
otherwise have no association.
- **global namespace** A hierachical set of entities capable of referencing
each other by a '/' separated path string. Top level namespace entities may
represent users or groups which parent its sub namespace.
- **grant** An extension of permission between two entities along with
a set of verbs and a delegation flag.  When the delegation flag is given,
the grant may be used to create further grants.  A grant contains a 
subject namespace, grantee, set of verbs, and delegation flag.
- **graph provider** Provides graph links to clients to populate the client's
local graph.  Links are provided as an array of grants.
- **graph link** Connects two entities in the namespace by a directed edge
containing a verb string. Since links are directed edges, a link may only be
walked in one direction.  Every link is associated with at least one grant.
- **namespace element** An entity identified by a path with '/' separated
elements. The path is hierarchical with the left-most elements representing
parents of elements on its right (e.g. `/someuser/some-app` may represent an
image owned by someuser).
- **revocation** The removal of a link in the graph, which may be replaced
temporarily by a link marking its deletion.
- **trust client** An application which uses the trust graph to verify content.
- **trust graph** A collection of graph links which may be walked between any
two nodes. A trust graph will not contain dangling nodes since a node's
existance is marked by the existance of a link referencing it.
- **trust server** A graph provider which returns grants given signed
content.  The trust server also accepts new grants from clients and can
generate grants associating keys to a namespace element. 
- **verb** A string representing an action allowed (or disallowed for
revocation) by a graph link. The verb can be used to translate a link to
"subject verb object" with the object representing the element being directed
to and subject the element being directed from (e.g. "user-1 delegates pull to
public-key-1" would be represented by a link with the verb "pull" with the
delegate flag set and direction from "/user1" to "public-key-1").

### Description
The trust graph is collection of grants used to form links between elements in
the global namespace. Every node in the graph represents a namespace element
which may be a public key, user, group, or resources. Every link between two
nodes is directed and contains a verb signifying what permission is being
extended and a flag to determine whether further extension is permitted.
When a key is registered to credentials at login, a link is inserted into the
trust graph associating the public key with the user owning the credentials.
Additionally, users can be linked to other resources or groups.

### Delegation
The "delegate" flag is used to indicate grant extension between nodes, most
commonly used to link a public key to user. Once a user has delegated to a
public key, that public key may be used to grant permission to other namespace
elements.

### Revocation
A revocation removes a link in the graph and creates a temporary link
indicating the revocation. These revocation links may be cleaned up after time.
These revocations are not cleaned up immediately to allow propagation to graph
caches.

### Verification
Any content may be verified against the trust graph by extracting the public
key, the relevant element, and a target verb from the content. The graph will
be walked from public key to the relevant element, only using links that allow
the needed verb. When the target element is part of a child namespace of a
reached node, that parent node will be used. If a path is found between the
elements, the content is considered verified. By requiring signed content the
public key will always be the starting point for verification and will disallow
arbitrarily walking the graph without permission originating from the private
key. For verifying build manifests, the public key is the signer, the verb
will be "build", and the relevant content is the image name.

### Local caching
While the trust graph can be considered global, its size is too large to
propagate to every client wishing to check permissions against the graph.
Likewise not all permissions are intended to be shared publicly and should only
be shared with an audience verifying content which requires those permissions.
Trust clients will be able to send signed content to a graph provider which will
verify the content and return the links (grants) in the graph which are needed
to verify the content. The grants will be loaded into a local trust graph and
verification will be done directly against the local graph. These grants can be
cached for a period of time allowing for subsequent queries for the same content
or content with the same permissions to skip calling the graph provider.

### Sub Graphs
The global graph may be a collection of public top level elements as well as
private sub graphs which handle permissions for a subset of a namespace. These
subgraphs may contain elements unknown to the public graph providers and
therefore unable to be walked by a single graph provider, however these private
graph providers will be able to provide links to local trust clients such that
local clients can do verification. Graph providers must allow for a single
graph traversal to span multiple providers.

## Trust Server
The trust server is a service used to extend a local trust graph to a larger
shared graph. Signed content is sent to the trust server and it returns graph
links which can be used locally for verification. The trust server can
understand multiple content types (e.g. build manifest, JWT) and extract the
graph information needed to provide the grants.  The trust server accepts new
grants which are signed by the client and sent the trust server. The trust
server verifieds grants, and updates its trust graph. The client may also send
up a set a credentials to link to its public key, in which case the trust server
will create a grant, add it to its trust server, and returns the grant to the
client. These grants are signed by an x509 certificate which chains to a root
CA configured into Docker.  This certificate ensures that the trust server is
only able to create grants within a specific namespace. Since these grants are
the starting point for trust verifications and easily verified, they may be
shared and cached safely.  A trust server will also be able to use its
certificate to generate a request for additional grants and send them to other
trust servers. Each trust server will be able to verify the trust server is
requesting grants related to a section of the namespace their certificate has
limited them to. The graph backends to the trust server as well as credential
store used to check before grant generation will be pluggable.

### Design

#### Statement Queries

The Trust Server can be queried for statements by sending signed content. The
signed content must be a JSON Web Signature with the content type understood by
the Trust Server. The Trust Server will use the public key of the valid
signature, identifier associated with the content, and action from the content
to query. The action may be derived from the content (image manifest means
builds) or explicitly stated such as a “actions” field in a JWT. Requiring a
valid signature ensures that queries cannot be made for public keys and
identifiers which have no association. This prevents potential abuse of the
system by making it impossible to scrape the system for information which is
not already public. This also allows querying without additional authorization
since having possession of the content grants access to the links to verify it.

#### Query Result

The Trust Server returns an array of grants representing the list of relevant
graph edges needed to traverse the graph for a given query. The array contains
both a list of revocation and non-revocation grants. Every grant may have an
expiration which is expected to be cached for repeat queries of the same
content. Revocations should be included for up to the expiration time of the
grant is revoking to ensure any valid cached data may be overridden. The
signature on the key grants should chain to the root Docker CA configured for
the engine.  

### Grant

A grant is represented by a JSON Web Signature with the content being a 
grant JSON object and signature by a key authorized to make the grant.  The
key may be a key which has been delegated to the subject namespace or an x509
certificate which has authority over the subject namespace.

Grant object

user1 allows user2 to build and pull
~~~~
{
   "subject": "user1",
   "actions": ["build", "pull"],
   "delegated": false,
   "revoked": false,
   "grantee": "user2",
   "expiration": "2014-12-29T00:08:20.565183779Z",
   "issuedAt": "2014-09-30T00:08:20.565183976Z"
}
~~~~

user2 delegates to key
~~~~
{
   "subject": "user2",
   "actions": ["any"],
   "delegated": true,
   "revoked": false,
   "grantee": "LYRA:YAG2:QQKS:376F:QQXY:3UNK:SXH7:K6ES:Y5AU:XUN5:ZLVY:KBYL",
   "expiration": "2014-12-29T00:08:20.565183779Z",
   "issuedAt": "2014-09-30T00:08:20.565183976Z"
}
~~~~

User created grant
~~~~
{
   "payload": "ew0KICAgInN1YmplY3QiOiAidXNlcjIiLA0KICAgInNjb3BlIjogWyJhbnki...",
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kty": "EC",
               "x": "Cu_UyxwLgHzE9rvlYSmvVdqYCXY42E9eNhBb0xNv0SQ",
               "y": "zUsjWJkeKQ5tv7S-hl1Tg71cd-CqnrtiiLxSi6N_yc8"
            },
            "alg": "ES256",
            "cty": "json/trust+grant"
         },
         "signature": "sgbJD2AJ-2Lu6KpbthnUyB6vSr9uVUAV9Cx8QtfA-0Gw...",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjEzMDM4LCJmb3JtYXRUYWls..."
      }
   ]
}
~~~~

Trust Server created key grant
~~~~
{
   "payload": "ew0KICAgInN1YmplY3QiOiAidXNlcjIiLA0KICAgInNjb3BlIjogWyJhbnki...",
   "signatures": [
      {
         "header": {
            "alg": "ES256",
            "x5c": [
               "MIIBnTCCA...",
               "MIIBnjC..."
            ],
            "cty": "json/trust+grant"
         },
         "signature": "uYMwXO86...",
         "protected": "eyJmb3JtY..."
      }
   ]
}
~~~~

#### Actions
The action used in grants and revocations are an array strings referring to
verbs. Grants can be chained together through delegation verbs.

##### Example Scopes
build - action used for verifying build manifests
connect - action used for allowing connections to host or service
pull - action used to allow pulling of a private image

##### Reserved Scopes
any - Allows any action

#### Subject
The subject is the namespace in which the action may be applied. When chaining
the subject must correspond to a grantee in the next link. When the subject
ends with a “/”, the action is considered to be applicable only on children, not
directly on the named subject.

#### Grantee
The grantee is the namespace which is permitted to perform the action verb on
the subject. The first grantee in a chain of grants will usually be a public
key since a public key is easily authenticated through a signature. The first
grantee should always be authenticated by the service consuming the trust graph.

#### API
The Trust Server can have one or multiple ways to make queries. The socket
must be over TLS to prevent MITM sniffing of sent content.  Because grants are
by definition shareable and self verifiable, a MITM will be unable to create
or alter statements.  However TLS should still be used to ensure that a MITM
cannot remove revocation statements.

##### via REST


`POST /graph/` - Query the graph for grants

The Trust Server has a single endpoint for graph queries which accepts signed
content via the request body and returns an array of statements via the response
body.

`POST /grants/` - Add grant to trust graph

Request body contains a signed grant, no response body is expected.

`POST /keys/<namespace>` - Create grant to associate public key with user

Request body is public key to associate with account. Returns grant created.

#### Namespacing
Any given trust server may be responsible for the entire or subset of the
namespace. When receiving queries for sections of the namespace which are
unowned, the trust server should be able to proxy requests other trust servers.
 This means the trust server will need to be configured to know about other
trust servers and which namespace those trust servers are responsible for.

#### Caching
Grants may be cached to disk and reloaded until the statement is expired.
After expiration the statement may still be used while offline, but once online
should requery to get any revocations. The caching/requerying interval will
balance security and performance.

#### Chain of Trust
Grants generated by the trust server will be signed by a leaf certificate in
an x509 chain. All intermediate certificates will be listed in the “x5c”
section of the JSON web signature to allow chaining to the root. The libtrust
client will need to have the x509 root certificate to verify statements.
Grants may be cached or exported/imported and remain verifiable against the
chain of trust.

#### x509 Certificate Authorities
Certificate Authorities in the trust system will be used to sign leaf
certificates with specific roles and namespace scope.  A top level
CA is able to create leaf certificates over the entire namespace. A
scoped CA is only able to generate leaf certificates for a sub-section
of the namespace. The sub-section is determined by the Common name of the
certificate. Certificate authorities may also have a limited role in which
types of leaf certificates may be issued. The different roles are the
key linkage (which allows linking a public key fingerprint into the namespace)
or name linkage (which allows linking between two section of the namespace).
The key link role is used to limit a certificates ability to manage grants
between the namespace and only allow the linked keys to handle that role.
The name link role is used to manage grants between namespace elements which
includes the key link role. A trust server which supports the associate
public key endpoint must have a leaf certificate signed by one of these
Certificate Authorities.

#### Self-signed Grants
A self signed grant is a grant in which the key used to signed is also
represented as the subject by the public key fingerprint. This can be used
by entities which use a key pair for identity and grant permission to its
resources directly. These grants may be verified and distributed by the
trust server without needing to link directly into the x509 chain.

#### Backends
The trust server should have a pluggable interface to the graph datastore.
The implementation of the graph queries and updates are up to implementation of
the interface. The query to the backend is expected to be pre-authorized, no
credentials will be provided to plugins. The credential store to check username
and password will also be pluggable and have an interface separate from the
graph datastore.

#### Backend Interface
The backend interface will be communicated over libchan using libchan supported
objects as returned values. Using libchan will allow the interface to
be pluggable either in process or by a subprocess/container.

### Concerns
- Authentication is avoided by the trust server to allow anonymous querying to
validate signed content. This makes any signed content which does not contain
an expiration forever vulnerable to replay by anyone in possession of the
signed content.

- Leaking a “private” manifest allows making queries against the trust server
which may expose a grant showing the user who build the image (if not part of
the image name) and membership in the image name’s ownership group. For
example if a manifest for “privategroup/image1” was leaked and a query to the
trust server returned grants for (secret user (delegate)-> key1, private group
(build)-> secret user), then secret user’s identity and membership is leaked
along with the manifest. This can be mitigated by collapsing grants on the
Trust Server, which is only possible if the returning trust server has
authority over the subject which would get collapsed (may not be possible with
proxying to multiple trust servers).

- Caching can keep revocations from getting picked up by clients. The
expiration time should be balanced to ensure that revocations take effect.
Revocations only need to be kept long enough to ensure that any previous
grants concerning the subject have expired.


### Open Design Questions
- How should keys and other objects be separated at the top level namespace?

- Does the trust server proxy the content or create a new request (JWT) just
for the subgraph that is needed? In which case the trust server will need to
verify the signature as part of the x509 chain and derive the public key from
the content instead of signing key. Would this just be a different content
type or different endpoint? Also once receiving statements, should the
statements be passed on as is or combined. Combining may not be possible if a
trust server own only a subset and is not authorized to sign on the content it
received.

- Do revocations need to stay around longer to ensure that any grant, even
ones unknown by the trust server, are overrode?

## libtrust
Libtrust provides a Go implementation of the trust graph for use as a local
trust client. It also provides the necessary functionality for extracting
information from the trust server supported content types and creating the
statements used by the client. All the cryptography used by libtrust uses the
standard libraries provided in Go. Non-Go implementation of libtrust are
planned but none exist today.

- [libtrust (Go)](https://github.com/docker/libtrust)
