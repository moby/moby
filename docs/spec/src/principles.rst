#####################
 Behavior principles
#####################

A principle holds across the specs. An expectation refines every principle and requirement whose
statement covers it directly, whichever spec the requirement belongs to.

.. principle:: The daemon leaves nothing behind
    :id: PRIN-NO-LEAKS
    :refines: PRIN-OWNS-ONLY-ITS-OWN

    What the daemon creates for a network, an endpoint, a container, a sandbox or a service does not
    outlive that object. Examples are network namespaces, interfaces, firewall rules, kernel state
    for encryption and the daemon's records of the object. Nothing of it remains once the object is
    gone. After the daemon restarts following a crash, or after an upgrade, nothing that the earlier
    daemon created remains unless the daemon still uses it for an object that exists. An operation
    on the object that fails leaves nothing of what the operation created.

.. principle:: Bad configuration fails closed
    :id: PRIN-FAILS-CLOSED
    :refines: PRIN-FAITHFUL

    A request or a daemon setting whose configuration the daemon or a driver cannot apply fails: the
    request is refused, or the daemon does not start with that setting. The daemon does not go on
    with that configuration ignored, or with another value in its place.

.. principle:: A request by an ambiguous name or ID prefix is refused
    :id: PRIN-AMBIGUITY-NOT-DESTRUCTIVE
    :refines: PRIN-FAITHFUL

    A request names an object by its full ID, then by its name, then by a prefix of its ID, and the
    first of these that matches decides. A request whose string matches more than one object at that
    rank is refused. The request changes and removes none of the objects that the string matches.

.. principle:: A request that cannot succeed is refused as early as it can be
    :id: PRIN-FAIL-FAST
    :refines: PRIN-USER-INFORMED

    When a request cannot succeed because of what it asks, given the properties that the objects it
    names keep for as long as they exist, the daemon refuses it with an error response, at the
    earliest request at which the daemon can detect the problem. A request that the daemon accepts
    does not leave such a failure to a later request, to the tasks of a service, or to the daemon
    log. A failure that depends on state that can change before the later request, such as whether
    an object exists, what holds a resource, or the configuration of the daemon or the host, is not
    one of these: :need:`PRIN-WARN-AHEAD` covers it.

.. principle:: The daemon keeps the user informed
    :id: PRIN-USER-INFORMED

    The daemon keeps the user informed. It reports a problem with a request at the earliest point it
    can, in terms the user can act on, and never leaves the user to discover from later behavior
    that a request did not do what was asked.

.. principle:: A request does what it asks, or nothing
    :id: PRIN-FAITHFUL

    The daemon does what a request asks, or nothing. It does not apply something other than what was
    asked, act on objects the request did not name unambiguously, or let anything the request does
    not control decide the outcome.

.. principle:: The daemon manages only what it creates
    :id: PRIN-OWNS-ONLY-ITS-OWN

    The daemon manages only what it creates. It removes everything it creates once the object it
    belongs to is gone, and it leaves alone what it did not create: it does not remove or change it.
    Where the daemon's own state sits relative to what others add, such as the order of rules in a
    chain that both use, is not promised, except at an anchor or where users observably rely on it.

.. principle:: A node-local object is a resource of its node
    :id: PRIN-NODE-LOCAL

    A network of local scope, like a volume, is a resource of the node that has it. On that node,
    whether something uses it depends only on the objects that exist on that node: a Swarm object
    that names it, such as a Swarm-scoped network created with :api-field:`ConfigFrom`, uses it on a
    node only while that object has a local attachment there. Whether a Swarm object can use a
    node's local resources is for the orchestrator to decide, as it decides where tasks run.

.. principle:: The daemon grants no access the user did not ask for
    :id: PRIN-LEAST-ACCESS

    The daemon grants no access the user did not ask for.

.. principle:: What a release does, later minor releases keep doing
    :id: PRIN-STABLE

    What a release does at the daemon's interfaces, later minor releases keep doing; a change needs
    a major release, or an option that restores the behavior. A change that only a new version of
    the Engine API makes adds an interface and changes none, and removing a version of the Engine
    API needs a major release. The daemon's interfaces are each version of the Engine API that the
    daemon supports, the CLI, the daemon's configuration, what these make observable in containers,
    networks and traffic, the anchors that a spec names, and behavior on a host that users
    observably rely on, which an observable requirement records. Everything else that the daemon
    creates on a host, such as its firewall rules, interfaces, namespaces and files, and their
    names, content and order, is how it brings that behavior about, and a minor release can change
    it.

.. principle:: Security that a user enables is not weakened
    :id: PRIN-SECURITY-NOT-WEAKENED
    :refines: PRIN-STABLE

    When a user enables a security feature, such as encryption of a network, a later minor release
    does not give weaker protection. That includes the parameters that a security review of a
    deployment records, such as the algorithms, the key lengths and the longest time that a key is
    used. An option that restores the earlier protection does not make a weakening a minor-release
    change. A major release that weakens one of them states it in its release notes.

.. principle:: The outcome of a request does not depend on internal ordering
    :id: PRIN-REPEATABLE
    :refines: PRIN-FAITHFUL

    The outcome of a request depends only on the request and the state the daemon reports. The
    daemon does not let an order it chooses internally, and that the request does not control,
    decide whether the request succeeds.

.. principle:: Errors say what to change
    :id: PRIN-ERRORS-ACTIONABLE
    :refines: PRIN-USER-INFORMED

    An error response, a task error or a log entry for a refused request, or for a failure that a
    setting, option or object of the user causes, tells the user what to change: it names the
    setting, option or object at fault and why, in the terms the user used. It does not report
    internal state, such as a file path or a context deadline, in place of the cause.

.. principle:: Foreseeable failures are reported when the request is accepted
    :id: PRIN-WARN-AHEAD
    :refines: PRIN-USER-INFORMED

    When a request is accepted with configuration that state the daemon holds will make a later
    request, or the tasks of a service, fail, the daemon says so in that request's response, in its
    warnings, not only when the later request or the tasks fail. That state can change before the
    later request: whether an object exists, what holds a resource that the configuration asks for,
    and the configuration of the daemon and the host. The user can then change the configuration or
    the state. State that the daemon does not hold, such as the node-local networks of other nodes,
    is not covered.
