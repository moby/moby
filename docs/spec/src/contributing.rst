###########################
 Contributing to the specs
###########################

This document is for anyone who changes behavior that a spec states, or who writes, edits or reviews
a spec. It gives the workflow and the conventions that the maintainers apply. The model is in
:doc:`README.md <README>`, the conventions of a subsystem's specs in that subsystem's README, the
principles in :doc:`principles.md <principles>`, and the forms of IDs and titles in
:doc:`id-style-guide.md <id-style-guide>`. Where this document disagrees with them, they hold, and
the change that finds the disagreement fixes this document.

.. _contributing-when-a-code-change-alters-specified-behavior:

**********************************************
 When a code change alters specified behavior
**********************************************

A pull request that changes behavior that a spec states updates the spec in the same pull request.
To find the expectations that a change affects, search the sources for the API operations, fields,
options and settings that the change touches. What the change needs depends on the category of each
expectation that it makes false (:ref:`Categories <readme-categories>` in :doc:`README.md
<README>`):

- **A contract.** The pull request says how the change meets the rule for a contract. A change that
  only a new version of the Engine API makes changes no contract at the older versions: the contract
  is narrowed to the versions at which it holds, and a new expectation states the new behavior. A
  change at every version needs a major release, or an option that restores the behavior, unless a
  principle that the contract refines asks for more, as ``PRIN-SECURITY-NOT-WEAKENED`` does. Raising
  the daemon's minimum API version removes interfaces, so it needs a major release. A change to the
  Engine API also has an entry in :repo-path:`api/docs/CHANGELOG.md`. A contract is not loosened to
  make it pass: a contract that fails is how the specs show such a change.
- **A defect.** The pull request says whether the change is a fix, or a break for the users who met
  the old behavior. The defect is replaced by the contract that the daemon now meets, or removed. A
  fix that applies only from a new API version leaves the defect at the older versions.
- **A provisional expectation or a detector.** The change is weighed against the caveat of the
  source or against the rationale, and the expectation is updated.
- **An expectation that pins a host requirement,** because the daemon no longer needs the
  requirement. The requirement is updated too.

The ID of a merged item that the change removes goes into ``retired-ids.txt``.

.. _contributing-adding-or-changing-an-expectation:

***********************************
 Adding or changing an expectation
***********************************

1. **Establish the behavior** (:ref:`contributing-establishing-what-the-daemon-does`). Note the
   version that you ran.
2. **Find its place.** Choose the spec and the section whose scope covers the behavior.
3. **Decide what backs it.** Look for a requirement or a principle that covers the behavior first.
   Add a requirement only with a source that can be cited, and a rationale only for a detector that
   is not an exact form and pins nothing. If neither a requirement nor a rationale can honestly be
   written, the behavior does not belong in a spec. Before treating host state that the daemon
   creates, such as a rule and its position, as unpromised, look for users who rely on it.
4. **Write the expectation** (:ref:`contributing-wording-an-expectation`).
5. **Fill in its fields:** ``observed_from``, ``depends_on`` and ``platforms``, ``key`` in a list of
   options, and the links: ``needs``, ``within``, ``assumes`` and ``cli_for``, and ``narrows`` or
   ``concerns`` for a defect.
6. **Place it** by the rules of the README and of the subsystem's README. A defect sits next to an
   expectation that it narrows or concerns. An expectation about a :cli:`docker` command sits next
   to the expectation about its API request. In the networking specs, an expectation whose case
   needs a Swarm sits in the "In a Swarm" subsection of its subject's section.
7. **Give it an ID** by the style guide. A merged ID never changes and is never used again.
8. **Build it** with ``make -C docs/spec fmt md validate``. The build finishes without warnings.
   Commit the generated Markdown with the sources.
9. **Write the commit message** (:ref:`contributing-commit-messages`).

.. _contributing-wording-an-expectation:

************************
 Wording an expectation
************************

The README's :ref:`readme-expectations` section comes first: what the *Applies to* paragraph holds,
what a statement says, what an outcome covers, and which cases an expectation names. These rules
apply within it.

Register
========

- An expectation is in the present tense, about what the daemon does now. So is a defect.
- An expectation uses the exact name of an API field, flag, :file:`daemon.json` key or option, with
  the role that says what it names, such as ``:api-field:``, ``:dockerd-flag:``, ``:daemon-json:``
  or ``:drvopt:``. ``ROLES`` in ``ext/spec_roles.py`` lists the roles. A bare value or a placeholder
  is a plain literal.
- Every term that an expectation relies on, such as "auto-assigned port" or "resolves", is defined
  in the introduction of its section or in a README. Within a spec, a term has one meaning.
- A term means what the Engine API presents as that thing. An expectation that leaves out something
  that the API presents as one, such as a network that a lookup returns, names what it leaves out.
- An abbreviation is written as its domain writes it, such as TTL, PTR, NAT, VXLAN or XFRM.
- A sentence of 25 words or fewer is the aim. A longer sentence is better than a dropped
  precondition or an invented shorthand term.

*Applies to*
============

- The *Applies to* paragraph is a noun phrase: "A Swarm with a service that publishes a port in
  ingress mode".
- A state that holds through the observation, such as "while X", goes in *Applies to*. An event that
  the expectation's own request causes goes in the statement.
- An absence is attached to something that exists: "A Swarm with an ingress network, where no
  service publishes a port". It is not an item of its own.
- *Applies to* holds the state of the system, never a bare value. A value that the request supplies
  is stated in the request. A value that something holds is stated through its holder: "A daemon
  configured with an ``mtu`` of …".
- Every object that the expectation's request names has a precondition, in an expectation about a
  :cli:`docker` command too. The object that the request creates has none: the request is the
  subject.
- A trailing "this holds when X" that narrows a statement, or adds a case with another starting
  state, is a precondition. X moves to *Applies to*, or the case becomes an expectation of its own.
  A quantifier that says that a condition does not matter, such as "whether or not" or "for TCP and
  UDP", stays in the statement.

A case that tells the outcomes apart
====================================

Every statement is checked the same way: would a daemon that does the opposite, or nothing, still
pass? The case that the expectation names rules that daemon out. Examples from the DNS spec:

- An expectation about forwarding names an upstream nameserver that has the record.
- An expectation that the resolver skips a nameserver names another nameserver after it.
- An expectation about a rename names a new name that differs from the old one in more than the case
  of its letters, since the resolver compares names regardless of case.

Requests and outcomes
=====================

- The object and the parameters come with the request, before the status: "``<request>`` for
  <object>, with <parameter>, responds <status>".
- A request about a container that is not running names "a created or stopped container", and "a
  created container" only when the outcome says that the container stays created. Where the request
  behaves differently on a running container, that case is an expectation of its own.
- A request about a container runs on the node that has the container. "On any node" goes on the
  container in *Applies to*, not on the request.
- An expectation about a :cli:`docker` command stays in the command's terms: the command, what it
  prints and its exit status.
- A defect whose outcome depends on an order that the daemon picks says that repeated attempts of
  the same request give both outcomes. It gives no number of attempts.

One expectation, one observation
================================

- Separate checks of the same state, especially from different vantage points, are separate
  expectations with the same *Applies to* paragraph and their own ``observed_from`` and
  ``depends_on``.
- A request and the checks of its effect stay together, as do alternative vantage points, such as a
  task or a container, and a claim whose input comes from another vantage point.

Level, edge and interval
========================

- An expectation about what holds while a condition persists takes the condition as its *Applies
  to*. An expectation timed from the start of the condition keeps the start as its "when". A
  contract about the level and a detector about the edge can both state the same observable.
- An expectation that something holds throughout an interval, or that nothing interrupts it across
  events, says, for each event, from when and until when it holds, and links each convergence
  ceiling that it uses. A subsystem's README can set that bound.

.. _contributing-establishing-what-the-daemon-does:

***********************************
 Establishing what the daemon does
***********************************

An expectation states behavior that has been observed. It is established from the code and the
documentation, and what they leave open is settled by running a daemon.

Reading
=======

- The code on the main branch is what counts. A maintenance branch or a release tag can differ.
  Trace a request from its router in :repo-path:`daemon/server/router` to the daemon and the
  component code, and back out to the response.
- The status of an error response follows from how the error is typed, with the types of
  ``github.com/containerd/errdefs``, and from how :repo-path:`daemon/server/httpstatus` maps the
  type to a status. An error that matches no type responds with 500.
- The documented behavior is in the Engine API reference (:repo-path:`api/openapi.yaml`, and the
  versioned pages on docs.docker.com), the API's version history
  (:repo-path:`api/docs/CHANGELOG.md`), the release notes, and the guides on docs.docker.com, whose
  source is the docker/docs repository. The commit that introduced a behavior, and its pull request,
  often state the intent.
- A component outside the daemon, such as swarmkit, a kernel subsystem or the Windows Host
  Networking Service, is part of the behavior. Its code is read in the vendored copy, or in its own
  repository at the version that the main branch uses.

Running
=======

A daemon is run whenever reading leaves the outcome open, and whenever a run is cheap. A run settles
a claim that a reading of the code makes look symmetric. Use a throwaway daemon, and remove it and
its volumes afterwards:

- A released version:

  .. code-block:: sh

      docker run -d --privileged --name spec-probe docker:<version>-dind
      docker exec spec-probe docker <command>
      docker rm -fv spec-probe

- The code of a branch: build it in the development container (``make shell``, then ``hack/make.sh
  binary``), and start :program:`dockerd` from the bundle in that container.
- A Swarm: start several such containers on one user-defined network, run ``docker swarm init`` in
  one and ``docker swarm join`` in the others.
- An API request that the CLI does not make: install curl in the container (``docker exec spec-probe
  apk add -q curl``) and send ``curl --unix-socket /var/run/docker.sock http://localhost/v<API
  version>/<path>``. A daemon started with :dockerd-flag:`--debug` logs each request that it
  handles, which shows the requests that a CLI command makes.

A container that runs the daemon this way boots no init system and has no package of the daemon. It
shows what the daemon's code does. Where the package, its service units or the host's services can
change the outcome, the behavior is checked on a host that meets the subsystem's definition of a
node.

The version that was run goes with the claim: in the commit message, or in the pull request when the
run settles a question in review.

A run shows today's behavior on one setup. It does not show that the behavior is promised: the
sources and the reliance test decide that. Where the platform, a daemon setting such as the firewall
backend, or the API version could change the outcome, those cases are run too, or the expectation is
narrowed to the cases that were run.

Facilities that a run lacks
===========================

A facility that a test or a run needs can usually be built:

- A program is hidden with an overlay mount or with ``PATH``.
- A kernel facility is blocked with a Linux security module, a seccomp filter or a module blacklist.
- A condition is brought about on demand with a failpoint in the daemon, or with a new one
  (:ref:`readme-who-the-specs-serve` in :doc:`README.md <README>`).
- A service, such as firewalld, runs in an image that has it installed, as CI does.

That an expectation needs such a facility is work to do, not a reason to leave it out.

*************************
 Reviewing a spec change
*************************

A spec change is reviewed against the code and the README's rules, not against how the expectations
read. ``make -C docs/spec diff`` lists the items that changed since the merge base with the main
branch, a word diff of each changed field, and the linked items that each change can affect.
``DIFF_FLAGS=--summary`` prints only the list of items. CI shows the same report on a pull request.

For each new or changed item:

- **Behavior.** Does the code on the main branch do what the expectation says, in every case of its
  scope, under every daemon setting that it holds under and on every platform in its ``platforms``?
- **Category.** Do its links give the category that the sources and the reliance test call for? Does
  a contract refine every requirement and principle that covers it directly? Does a defect violate
  the most specific target that it contradicts directly?
- **Sources.** Does a new requirement say what its sources say, no more and no less? Does some
  expectation refine each of its clauses?
- **Scope.** Does the *Applies to* paragraph state the whole precondition? Does a defect narrow each
  expectation that it contradicts in its case? Would a daemon that does the opposite, or nothing,
  still pass?
- **Fields and wording.** Are ``observed_from``, ``depends_on``, ``platforms``, ``needs`` and
  ``within`` right? Is every sentence falsifiable, and does the wording follow
  :ref:`contributing-wording-an-expectation`?
- **IDs.** Do new IDs follow the style guide? Are merged IDs unchanged, and is every removed merged
  ID in ``retired-ids.txt``?
- **Build.** Does ``make -C docs/spec validate`` pass, with the generated Markdown committed?

A reader who knows the reasoning behind an expectation fills its gaps without noticing them. For a
large change, ask someone who did not write it to read only the generated Markdown, as the author of
a test would, and to report each expectation whose verdict they cannot decide, or could decide two
ways.

Findings
========

- A finding states its consequence first: what is wrong, and for which reader or test. That an
  expectation differs from another driver, from an earlier version of itself or from a code comment
  is not a finding by itself, and the other side may be the wrong one.
- A claim about ordering, races or teardown is checked against the locks and the order in the code,
  and quotes them. A claim that reading leaves open is run before it is filed.
- A finding that has no consequence is withdrawn, not filed with a hedge.
- Some findings are already settled by the README, such as a missing "unless it is refused" clause.
  A finding is checked against the README before it is filed.

Deciding
========

- A finding is decided by the README's rules, the principles and the reliance test. A question that
  they settle once the facts are known does not go to the maintainers.
- When findings of one class keep coming back because they conflict with a rule of the README, the
  rule is wrong or incomplete. The rule is amended, with its reason, in a change to the README or to
  :doc:`principles.md <principles>`, and the class settles at once, by a definition or a refinement
  of a principle, not by a disclaimer in each expectation. What the amended rule allows is checked
  against the behavior that users rely on.
- A question goes to the maintainers when its answer is a new commitment of the product, such as
  promising an undocumented behavior by documenting it, or when no rule or principle reaches it.

*******************
 Outside the specs
*******************

- An unfixed security vulnerability is never described in a spec, a commit message, an issue or a
  pull request. Report it as :repo-path:`SECURITY.md` says. An expectation can state the behavior
  once the issue is public.
- A bug in the daemon becomes a defect in the spec and an issue, filed in the terms of the
  repository that owns the code: an issue about swarmkit reproduces with swarmkit and uses its
  terms.
- A page of the documentation that a later deliberate change made stale gets a fix in the
  docker/docs repository.
- A behavior that users need, and that the daemon has no supported way to provide, gets a feature
  request.
- A behavior that a component outside the daemon decides, and that is known only from reading its
  code, gets an issue, not an expectation.

.. _contributing-starting-a-spec-for-another-subsystem:

***************************************
 Starting a spec for another subsystem
***************************************

A spec for another part of the daemon, such as volumes, images or the container lifecycle, joins
this project and shares its model, schema and principles. Its sources go in a directory of their own
under ``src/``, as the sources of the networking specs are in ``src/network/``.

1. **Pick a spec code and areas.** Add the code to ``SPEC_CODES`` in ``conf.py``, which builds the
   pattern of IDs from it, and add the code and the area codes of its sections to the style guide. A
   code is short and differs from the existing ones.
2. **Write the subsystem's README,** ``src/<subsystem>/README.rst``. It states the scope of its
   specs and what they do not cover. It adds only the conventions that the subsystem needs beyond
   :doc:`README.md <README>`: its vantage points, settings and scope, timing, the settings that its
   host requirements use and the documents that each spec's host requirements cover, and any
   subsection for a kind of case that the README's placement rule lets it name. Its table of
   documents marks each spec "Not yet complete." until the spec's sections cover the subsystem, and
   the spec's introduction says so too. Add the new README to the table of documents and the toctree
   of :doc:`README.md <README>`.
3. **Choose the vocabularies.** The vantage points and the ``when`` settings that the subsystem's
   specs use come from ``OBSERVED_FROM`` and ``WHEN`` in ``conf.py``. Where none fits, define a new
   one there, worded for every subsystem, instead of stretching an existing one. List the
   subsystem's choices in ``SUBSYSTEM_VOCABULARIES``, under the directory of its specs, in the order
   that its README gives them, and render its tables as the networking README does: with entries in
   ``needs_render_context``, and with ``<subsystem>-observed-from.rst.j2`` and
   ``<subsystem>-when.rst.j2`` templates in ``templates/``.
4. **Add the documents** to the subsystem README's toctree, and to ``LAX_DOCS`` in ``conf.py``, so
   that the build reports what the rules for linked specs find without failing (the end of
   :ref:`readme-requirements-principles-and-rationales` in :doc:`README.md <README>`). A document
   leaves ``LAX_DOCS`` once its items meet those rules, and ``LAX_DOCS`` is empty again when the
   pull requests that add the spec merge.
5. **Reuse the principles.** Add a principle only for a property that the maintainers hold the
   daemon to and that no principle covers. A principle is about the daemon, not about one subsystem,
   and refines a broader principle. A new root also gets a row in the README's table of roots.
6. **Source the requirements.** A requirement or a rationale goes at the start of the first document
   of its spec, in the order of the subsystem README's toctree, that links or names it. Each
   requirement cites a source. A requirement of another spec covers an expectation only where its
   statement and its sources cover the case: a networking requirement about refusal statuses does
   not by itself reach the requests of another subsystem.
7. **List the host requirements** that the subsystem needs, in full, by the rules of the README's
   :ref:`readme-general-host-requirements` section.
8. **Add roles** to ``ROLES`` in ``ext/spec_roles.py`` for inline code that no role fits. Where the
   repository holds the source of truth for the names, add a check of them to that file too.
9. **Write the expectations** section by section, as
   :ref:`contributing-adding-or-changing-an-expectation` says.

.. _contributing-commit-messages:

*****************
 Commit messages
*****************

The subject of a commit that changes the specs starts with ``docs/spec:``. The body says what the
diff cannot: the kinds of change, the rules that they apply and why, in the terms of the specs. It
does not describe each item, since ``make diff`` lists the changed items for a reviewer. A large
change to the content of the specs keeps its message to a few paragraphs. The daemon version that
settled a claim goes in the message.
