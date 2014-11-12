# Docker Trust
Docker uses a trust system based on public key cryptography and a global
federated namespace to link users to keys and resources.

## Login
The `docker login` command can be used to register public keys with the trust
system. By default every client and daemon instance of Docker will generate a
public key and using `docker login` will register that public key with the
credentials provided on login. By default `docker login` registers against
Docker Hub using Docker Hub credentials. An authentication server URL may also
be provided to register with.

After logging in, the key identifier registered with authentication server will
be stored in the local Docker configuration file (~/.dockercfg). If the
authentication server does not support public key registration, the credentials
will be stored in the configuration file.

## Trust Graph

### Terminology
- **delegation** Extension of identity between two entities which would
otherwise have no association
- **global namespace** A hierachical set of entities capable of referencing
each other by a '/' separated path string. Top level namespace entities may
represent users or groups which parent its sub namespace.
- **graph provider** Provides graph links to clients to populate the client's
local graph.
- **graph link** Connects two entities in the namespace by a directed edge
containing a verb string. Since links are directed edges, a link may only be
walked in one direction.
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
- **trust server** A graph provider which returns graph links given signed
content.
- **verb** A string representing an action allowed (or disallowed for
revocation) by a graph link. The verb can be used to translate a link to
"subject verb object" with the object representing the element being directed
to and subject the element being directed from (e.g. "user-1 delegates to
public-key-1" would be represented by a link with the verb "delegate" and
direction from "/user1" to "public-key-1").

### Description
The trust graph is collection of links between elements in the global
namespace. Every node in the graph represents a namespace element which may be
a public key, user, group, or resources. Every link between two nodes is
directed and contains a verb signifying what permission is being extended.
When a key is registered to credentials at login, a link is inserted into the
trust graph associating the public key with the user owning the credentials.
Additionally, users can be linked to linked to other resources or groups.

### Delegation
The "delegate" verb is used to indicate identity extension between nodes, most
commonly used to link a public key to user. Once a user has delegated to a
public key, that public key may be used with any permission given to the user.
In order to use a public key in the system, a signature or challenege is used
to prove possession of the associated private key.

### Revocation
A revocation removes a link in the graph and creates a temporary link
indicating the revocation. These revocation links are cleaned up after time.
These revocations are not cleaned up immediately to allow propogation to graph
caches.

### Verification
Any content may be verified against the trust graph by extracting the public
key, the relevant element, and a target verb from the content. The graph will
be walked from public key to the relevant element, only using links that allow
the needed verb. When the target element is part of a child namespace of a
reached node, that parent node will be used. If a path is found between the
elements, the content is considered verified. By requiring signed content  the
public key will always be the starting point for verification and will disallow
arbitrarily walking the graph without permission originating from the private
key. For verifying build manifests, the public key is the signer, the verb
will be "build", and the relevant content is the image name.

### Local caching
While the trust graph can be considered global, its size is too large propagate
to every client wishing to check permissions against the graph. Likewise not
all permissions are intended to be shared publicly and should only be shared
with an audience verifying content which requires those permissions. Trust
clients will be able to send signed content to a graph provider (called a Trust
Server) which will verify the content and return the links in the graph which
are needed to verify the content. The links will be loaded into a local trust
graph and verification will be done directly against the local graph. These
links can be cached for a period of time allowing for subsequent queries for
the same content or content with the same permissions to skip calling the graph
provider.

### Sub Graphs
The global graph may be a collection of public top level elements as well as
private sub graphs which handle permissions for a subset of a namespace. These
subgraphs may contain elements unknown to the public graph providers and
therefore unable to be walked by a single graph provider, however these private
graph providers will be able to provide links to local trust clients such that
local clients can do verification. Graph providers must allow for a single
graph traversal to span multiple providers.

## Trust Management
Management of links within the global graph will be provided by a separate web
service (e.g. Docker Hub) or in the future a tool (e.g. CLI) which communicates
to a graph management server using a defined API. Sub graph providers may use
their own databases as the source of the links which would need to provide its
own management system.

### Graph Management API
*FIXME needs definition*

## Trust Server
The trust server is a service used to extend a local trust graph to a larger
shared graph. Signed content is sent to the trust server and it returns graph
links which can be used locally for verification. The trust server can
understand multiple content types (e.g. build manifest, JWT) and extract the
graph information needed to provide the links. The returned links are signed
by the trust server as proof of authenticity and prevent transport tampering
(even through an unsecure proxy).  The statements are signed by an X509
certificate which changes to a root CA configured into Docker, allowing the
statements to be shared and cached safely. Trust servers which are graph
providers for a sub graph will sign with an X509 certificate defining the sub
graph namespace, allowing for querying any trust server without the possibility
of the trust server providing graph links which do not belong to its namespace.

The trust server is not a database not does it provide a key management API.
Rather the trust server connects to a graph data backend through a trusted
connection to find graph links. This data source must be manage separately and
will be pluggable into the trust server.

### Design

#### Statement Queries

The Trust Server can be queried for statements by sending signed content. The
signed content must be a JSON Web Signature with the content type understood by
the Trust Server. The Trust Server will use the public key of the valid
signature, identifier associated with the content, and action from the content
to query. The action may be derived from the content (image manifest means
builds) or explicitly stated such as a “scope” field in a JWT. Requiring a
valid signature ensures that queries cannot be made for public keys and
identifiers which have no association. This prevents potential abuse of the
system by making it impossible to scrape the system for information which is
not already public. This also allows querying without additional authorization
since having possession of the content grants access to the links to verify it.


#### Statement

The Trust Server returns a signed statement containing a list of relevant graph
edges needed to traverse the graph for a given query. The statement contains
both a list of additive and revocation permissions. Every statement has an
expiration which is expected to be cached for repeat queries of the same
content. Revocations should be included for up to the expiration time to
ensure any valid cached data may be overridden. The signature on the
statements should chain to the root Docker CA configured for the engine.

Example
~~~~
{
   "revocations": [
      {
         "subject": "/other",
         "scope": ["build"],
         "grantee": "LYRA:YAG2:QQKS:376F:QQXY:3UNK:SXH7:K6ES:Y5AU:XUN5:ZLVY:KBYL"
      }
   ],
   "grants": [
      {
         "subject": "/library",
         "scope": ["build"],
         "grantee": "LYRA:YAG2:QQKS:376F:QQXY:3UNK:SXH7:K6ES:Y5AU:XUN5:ZLVY:KBYL"
      }
   ],
   "expiration": "2014-12-29T00:08:20.565183779Z",
   "issuedAt": "2014-09-30T00:08:20.565183976Z",
   "signatures": [
      {
         "header": {
            "alg": "ES256",
            "x5c": [
               "MIIBnTCCA...",
               "MIIBnjC..."
            ]
         },
         "signature": "uYMwXO86...",
         "protected": "eyJmb3JtY..."
      }
   ]
}
~~~~

#### Scopes
The scope used in grants and revocations are an array strings referring to
verbs. Grants can be chained together through delegation verbs.  When the
subject ends with a “/” the scope applies to all child elements.

##### Example Scopes
build - scope used for verifying build manifests
connect - scope used for allowing connections to host or service

##### Reserved Scopes
any - Allows any scope except delegate
delegate - Allows delegation of any scope (implies “any” on entire subtree)
delegate_* - Allows delegation only on a specific scope (applies to entire
subtree)

#### Subject
The subject is the namespace in which the scope may be applied. When chaining
the subject must correspond to a grantee in the next link. When the subject
ends with a “/”, the scope is considered to be applicable only on children, not
directly on the named subject.

#### Grantee
The grantee is the namespace which is permitted to perform the scope verb on
the subject. The first grantee in a chain of grants will usually be a public
key since a public key is easily authenticated through a signature. The first
grantee should always be authenticated by the service consuming the trust graph.

#### API
The Trust Server can have one or multiple ways to make queries. The socket
must be over TLS to prevent MITM sniffing of sent content,  a false statement
cannot be generated through MITM.

##### via REST
The Trust Server has a single endpoint which accepts signed content via the
request body and returns an array of statements via the response body.
`POST /graph/`

#### Namespacing
Any given trust server may be responsible for the entire or subset of the
namespace. When receiving queries for sections of the namespace which are
unowned, the trust server should be able to proxy requests other trust servers.
 This means the trust server will need to be configured to know about other
trust servers and which namespace those trust servers are responsible for.

#### Top Level Namespaces
The top level namespace will not only be important for determining whether a
X509 certificate has authority, but to categorize elements in the namespace.
For example keys and users have a specific relationship where users delegate to
keys. By proving ownership of a key, a delegation can give access to user.
However delegations should never go in the reverse direction, in which case a
key delegates to a user, giving that user access to anyone who delegates to
that key. Since keys should never be the subject of a grant, no certificate
should have authority over keys, meaning keys are in a separate top level
namespace. The keys top level namespace will be “/keys”, while users will be
under a separate top level namespace such as “/object” or “/docker”. The same
could be true of emails which are granted authority through ownership of a DNS
domain, not by the x509 certificate.

#### Caching
Statements may be cached to disk and reloaded until the statement is expired.
After expiration the statement may still be used while offline, but once online
should requery to get an updated statement which may account for any
revocations. The caching/requerying interval will balance security and
performance.

#### Chain of Trust
Statements returned by the trust server will be signed by a leaf certificate in
an x509 chain. All intermediate certificates will be listed in the “x5c”
section of the JSON web signature to allow chaining to the root. The libtrust
client will need to have the x509 root certificate to verify statements. Once
a statement has been received it may be cached or exported/imported since it
will remain verifiable up until the statement expires.

#### Backends
The trust server should have a pluggable interface to the graph data source.
The implementation of the graph queries is up to implementation of the
interface. The query to the backend is expected to be pre-authorized, no
credentials will be provided to plugins.

#### Backend Interface
The backend interface will be communicated over libchan using a job model and
libchan objects as returned values. Using libchan will allow the interface to
be pluggable either in process or by a subprocess/container.

### Concerns
- Authentication is avoided by the trust server to allow anonymous querying to
validate signed content. This makes any signed content which does not contain
an expiration forever vulnerable to replay by anyone in possession of the
signed content .

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
statements concerning the subject have expired.


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

- Do revocations need to stay around longer to act as limiters on permissive
scopes (grant all to subtree except a specific element)?

### Open source implementation
Currently no open source implementation of the Trust Server exists.  Once a
pluggable backend interface is designed it may be easily built using libtrust.

## libtrust
Libtrust provides a Go implementation of the trust graph for use as a local
trust client. It also provides the necessary functionality for extracting
information from the trust server supported content types and creating the
statements used by the client. All the cryptography used by libtrust uses the
standard libraries provided in Go. Non-Go implementation of libtrust are
planned but none exist today.

- [libtrust (Go)](https://github.com/docker/libtrust)
