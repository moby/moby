"""Sphinx configuration for the behavior specifications.

Each spec item is a Sphinx-Needs need. The need types, their fields, the link
types between them and the schema rules that check them are declared here. The
vocabularies below also fill the tables in README.rst and network/README.rst.
"""

import os
import posixpath
import re
import sys

sys.path.insert(0, os.path.abspath("ext"))

project = "Moby behavior specifications"
extensions = ["sphinx_needs", "spec_roles", "md_needs", "observed_from"]
exclude_patterns = ["_build", "templates"]
root_doc = "README"
html_theme = "alabaster"
html_show_sourcelink = False
# Keep the source's straight quotes in the output.
smartquotes = False

# The root of the moby repository, for the checks of the semantic roles: this
# file is docs/spec/src/conf.py.
spec_repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
# The codes that the IDs of each spec's items use; a new spec appends its code.
SPEC_CODES = ["NET", "BR", "IPVL", "DNS", "SWARM"]
# iptables chains of earlier releases, which cleanup expectations name and no
# current reference ruleset shows.
spec_legacy_chains = ["DOCKER-ISOLATION", "DOCKER-ISOLATION-STAGE-1", "DOCKER-ISOLATION-STAGE-2"]

# --- Vocabularies ----------------------------------------------------------

#: What an expectation's verdict depends on (field ``depends_on``).
DEPENDS_ON = {
    "none": (
        "Nothing outside the daemon build",
        "The verdict does not differ between OS releases that run the same daemon build.",
    ),
    "os-linux": (
        "What a Linux distribution supplies: the kernel and first-party userland",
        "The kernel's features, the programs and services that the distribution ships, and their "
        "defaults, such as sysctl values. Which features each release offers, and how they behave.",
    ),
    "os-windows": (
        "What Windows supplies",
        "The services and features of Windows, such as the Host Networking Service. Which features "
        "each release offers, and how they behave.",
    ),
    "upgrade": (
        "A package's upgrade path from the previous release",
        "Its service units and maintainer scripts, which stop, start or restart the daemon across the "
        "upgrade.",
    ),
    "cli": (
        "The ``docker`` CLI a packager ships with the daemon",
        "Its version, how it turns commands and flags into API requests, and how it prints the "
        "responses.",
    ),
}

#: The platforms of nodes (fields ``platforms`` and ``os-<platform>`` in ``depends_on``).
PLATFORMS = {
    "linux": "Linux nodes",
    "windows": "Windows nodes",
}

#: The vantage points (field ``observed_from``).
OBSERVED_FROM = {
    "task": "Inside a Swarm service task's container",
    "ctr": "Inside a standalone container, one that is not a Swarm service task. A spec can narrow "
    "which networks the container is attached to.",
    "node": "The host network namespace of a node running the daemon",
    "underlay": "The physical network between nodes. Its traffic can be observed at any host on it, "
    "including at a node's own network interfaces. Traffic on it can come from any host on it other "
    "than the node under test, such as another node, or a container outside the node under test that "
    "is attached to the physical network.",
    "client": "A host other than the node under test that can reach one of the node's addresses. It "
    "can be another node of the same Swarm.",
    "api:mgr": "Engine API / CLI against a Swarm manager",
    "api:wkr": "Engine API / CLI against a Swarm worker",
    "api:active": "Engine API / CLI against a Swarm node, manager or worker, whose availability is "
    "``active``. Only such a node can run a Swarm service task or attach a standalone container to a "
    "Swarm-scoped network.",
    "api:solo": "Engine API / CLI against a daemon that is not in a Swarm",
    "api": "Engine API / CLI against a daemon, whether or not it is in a Swarm",
    "log": "The daemon log on a node. For some failures, it is the only channel that reports them.",
}

#: The kinds of host requirement (field ``kind``).
HOSTREQ_KINDS = {
    "os": "A facility of the operating system that the daemon uses: a feature of the Linux kernel, "
    "such as VXLAN devices or IPVS, or a service of Windows, such as the Host Networking Service.",
    "program": "A command that the daemon runs, such as ``iptables`` or ``nft``, and what it must "
    "accept.",
    "config": "State of the host that the daemon relies on and does not set, such as a sysctl that "
    "must have a given value, or traffic that the host's firewall must accept.",
}

#: The kinds of source of a requirement (field ``source_kind``).
SOURCE_KINDS = {
    "docs": "The current documentation, its release notes included",
    "docs-history": "A revision of the documentation that is no longer current, whose statement was "
    "dropped without being withdrawn",
    "api": "The Engine API reference, ``api/openapi.yaml``",
    "issue": "An issue or pull request in which the maintainers agreed that a behavior must hold: a "
    "maintainer's comment, a merged change's commit message or code comment, or a maintainer's triage "
    "of the issue as valid",
    "observable": "Behavior that users rely on without a document, because a change would break "
    "deployments that a reasonable user could build. The requirement names who relies on it and how",
}

#: The item types (README).
TYPES = {
    "expectation": "Behavior of the daemon. Its links decide its category (below).",
    "hostreq": "Something that a node must provide for the expectations that need it to apply.",
    "ceiling": "The time within which a class of change in a Swarm takes effect.",
    "req": "A promise that a cited source makes. Expectations refine it, or are provisional on it.",
    "principle": "A property that the maintainers hold the daemon to across the specs. A principle "
    "refines broader principles; one that refines none is a root.",
    "rationale": "Why a change to a detector's behavior should be deliberate, when users do not rely "
    "on that behavior.",
}

#: The categories of an expectation (field ``category``), computed from its own links, in this
#: order (README).
CATEGORIES = {
    "defect": "It ``violates`` a requirement or principle: known to be wrong. Whether a change that "
    "ends it is a fix or a break is weighed when the change is made.",
    "contract": "It ``refines`` a requirement or principle: promised. A change needs a major release, "
    "or an option that restores the behavior, unless a principle that it refines asks for more.",
    "provisional": "It is ``provisional_on`` a requirement whose own source says that it may change: "
    "promised with that caveat. It becomes a contract when the source drops the caveat.",
    "detector": "None of those: today's behavior, which should not change by accident. It is the "
    "exact form of another expectation, pins a host requirement or an expectation, or links a "
    "rationale.",
}

#: The settings under which a host requirement applies (field ``when``). A requirement applies when
#: all of its settings hold; without ``when`` it always applies.
WHEN = {
    "linux": "A Linux node",
    "windows": "A Windows node",
    "iptables-backend": "The daemon option ``firewall-backend`` is ``iptables``",
    "nftables-backend": "The daemon option ``firewall-backend`` is ``nftables``",
    "iptables-legacy": "The ``iptables`` program is the legacy implementation",
    "iptables-nft": "The ``iptables`` program is the nftables-based implementation",
    "iptables": "The daemon option ``iptables`` is true",
    "ip6tables": "The daemon option ``ip6tables`` is true",
    "ip-forward": "The daemon option ``ip-forward`` is true",
    "no-ip-forward": "The daemon option ``ip-forward`` is false",
    "userland-proxy": "The daemon option ``userland-proxy`` is true",
    "no-userland-proxy": "The daemon option ``userland-proxy`` is false",
}

#: The vantage points and ``when`` settings that each subsystem's specs use, in the order that its
#: README lists them, by the directory of its specs. The vocabularies above are shared by the specs
#: of every subsystem. An expectation's ``observed_from`` names only its subsystem's vantage points.
SUBSYSTEM_VOCABULARIES = {
    "network": {
        "observed_from": ["task", "ctr", "node", "underlay", "client", "api:mgr", "api:wkr", "api:active",
                          "api:solo", "api", "log"],
        "when": ["linux", "windows", "iptables-backend", "nftables-backend", "iptables-legacy", "iptables-nft",
                 "iptables", "ip6tables", "ip-forward", "no-ip-forward", "userland-proxy", "no-userland-proxy"],
    },
}

# --- Need types ----------------------------------------------------------

_SPECS = "|".join(SPEC_CODES)
needs_id_regex = rf"^(({_SPECS})-[A-Z0-9]+|REQ-({_SPECS})|RAT-({_SPECS}))(-[A-Z0-9]+){{1,5}}(-(CLI|HTTP[0-9]{{3}}|MSG|WINDOWS|LINUX|V4|V6|NFT|IPT))*$|^PRIN(-[A-Z0-9]+){{1,4}}$"
needs_id_required = True
needs_role_need_template = "{{ id }}"

_EXPECTATION_FIELDS = {
    "observed_from": {"nullable": False},
    "depends_on": {"nullable": False},
    "platforms": {"nullable": False},
}

_PRINCIPLE_REFINES = {
    "description": "A requirement or principle that serves a broader principle",
    "outgoing": "refines",
    "incoming": "refined by",
    "target_types": ["principle"],
}

needs_types = [
    dict(
        directive="expectation",
        title="Expectation",
        prefix="",
        color="#BFD8D2",
        style="node",
        fields=dict(_EXPECTATION_FIELDS),
        links={
            "refines": {
                "description": "An expectation that a requirement or principle promises",
                "outgoing": "refines",
                "incoming": "refined by",
                "target_types": ["req", "principle"],
            },
            "provisional_on": {
                "description": "An expectation that a requirement promises, whose own source says it may change",
                "outgoing": "provisional on",
                "incoming": "provisional basis of",
                "target_types": ["req"],
            },
            "violates": {
                "description": "An expectation that contradicts a requirement or principle",
                "outgoing": "violates",
                "incoming": "violated by",
                "target_types": ["req", "principle"],
            },
            "narrows": {
                "description": "A defect that contradicts another expectation in some case",
                "outgoing": "narrows",
                "incoming": "does not cover",
                "target_types": ["expectation"],
            },
            "concerns": {
                "description": "A defect about another expectation's case that does not contradict it",
                "outgoing": "concerns",
                "incoming": "concerned by",
                "target_types": ["expectation"],
            },
            "affects": {
                "description": "Every expectation a defect narrows or concerns",
                "outgoing": "affects",
                "incoming": "affected by",
                "derive": {"kind": "union", "fields": ["narrows", "concerns"]},
            },
            "exact_form_of": {
                "description": "The exact form of another expectation's observable",
                "outgoing": "exact form of",
                "incoming": "exact form",
                "schema": {"maxItems": 1},
                "target_types": ["expectation"],
            },
            "pins": {
                "description": "An expectation that pins what the daemon does without a host requirement, or another expectation",
                "outgoing": "pins",
                "incoming": "pinned by",
                "target_types": ["hostreq", "expectation"],
            },
            "rationale": {
                "description": "Why a change to a detector should be deliberate",
                "outgoing": "rationale",
                "incoming": "rationale of",
                "target_types": ["rationale"],
            },
            "detects": {
                "description": "What a detector is for",
                "outgoing": "detects",
                "incoming": "detected by",
                "derive": {"kind": "union", "fields": ["exact_form_of", "pins", "rationale"]},
            },
            "needs": {
                "description": "An expectation whose verdict needs a host requirement, under the requirement's settings",
                "outgoing": "needs",
                "incoming": "needed by",
                "target_types": ["hostreq"],
            },
            "within": {
                "description": "An expectation that holds within a convergence ceiling",
                "outgoing": "within",
                "incoming": "bounds",
                "target_types": ["ceiling"],
            },
            "assumes": {
                "description": "An expectation that relies on a property that another expectation states",
                "outgoing": "assumes",
                "incoming": "assumed by",
                "target_types": ["expectation"],
            },
            "cli_for": {
                "description": "An expectation about a docker command, and the one about the API request that the command makes",
                "outgoing": "CLI form of",
                "incoming": "CLI form",
                "schema": {"maxItems": 1},
                "target_types": ["expectation"],
            },
        },
    ),
    dict(
        directive="hostreq",
        title="Host requirement",
        prefix="",
        color="#D9D2E9",
        style="node",
        fields={"kind": {"nullable": False}, "platforms": {"nullable": False}},
    ),
    dict(
        directive="ceiling",
        title="Convergence ceiling",
        prefix="",
        color="#FFF2CC",
        style="node",
        fields={"seconds": {"nullable": False}},
    ),
    dict(
        directive="req",
        title="Requirement",
        prefix="",
        color="#C9DAF8",
        style="node",
        fields={"source_kind": {"nullable": False}},
        links={
            "refines": _PRINCIPLE_REFINES,
            "violates": {
                "description": "A requirement that conflicts with a principle",
                "outgoing": "violates",
                "incoming": "violated by",
                "target_types": ["principle"],
            },
        },
    ),
    dict(
        directive="principle",
        title="Principle",
        prefix="",
        color="#F4CCCC",
        style="node",
        links={"refines": _PRINCIPLE_REFINES},
    ),
    dict(
        directive="rationale",
        title="Rationale",
        prefix="",
        color="#EFEFEF",
        style="node",
    ),
]

needs_fields = {
    # Expectations
    "category": {
        "description": "Computed from the expectation's own links; never written",
        "schema": {"type": "string", "enum": list(CATEGORIES)},
        # only an expectation has a category, which spec_roles computes; other types leave it empty
        "nullable": True,
    },
    "observed_from": {"description": "Vantage point", "schema": {"type": "string", "minLength": 1}, "nullable": True},
    **{
        field: {
            "description": f"Computed from observed_from: {what}; never written",
            "schema": {"type": "array", "items": {"type": "string", "enum": list(OBSERVED_FROM)}},
            "nullable": True,
        }
        for field, what in (
            ("vantage", "every vantage point that it names"),
            ("observed_at", "the vantage points at which the expectation is observed"),
            ("sent_from", "the vantage points that send what is observed"),
        )
    },
    "depends_on": {
        "description": "What the verdict depends on",
        "schema": {"type": "array", "items": {"type": "string", "enum": list(DEPENDS_ON)}, "minItems": 1},
        "nullable": True,
    },
    "platforms": {
        "description": "The platforms where the item's case occurs",
        "schema": {"type": "array", "items": {"type": "string", "enum": list(PLATFORMS)}, "minItems": 1},
        "nullable": True,
    },
    "key": {
        "description": "The driver options that an expectation of an option list is about",
        "schema": {"type": "array", "items": {"type": "string"}},
        "nullable": True,
    },
    # Host requirements
    "kind": {
        "description": "The kind of host requirement",
        "schema": {"type": "string", "enum": list(HOSTREQ_KINDS)},
        "nullable": True,
    },
    "when": {
        "description": "The settings under which the host requirement applies; all must hold",
        "schema": {"type": "array", "items": {"type": "string", "enum": list(WHEN)}},
        "nullable": True,
    },
    "kernel_options": {
        "description": "The mainline Linux kernel options that provide the facility, without CONFIG_",
        "schema": {"type": "array", "items": {"type": "string", "pattern": "^[A-Z0-9_]+$"}},
        "nullable": True,
    },
    # Ceilings
    "seconds": {"description": "The ceiling in seconds", "schema": {"type": "integer", "minimum": 1}, "nullable": True},
    # Requirements
    "source": {
        "description": "Where the promise is made, one citation per entry",
        "schema": {"type": "array", "items": {"type": "string"}},
        "nullable": True,
    },
    "source_kind": {
        "description": "Kind of source",
        "schema": {"type": "string", "enum": list(SOURCE_KINDS)},
        "nullable": True,
    },
}


def _local(props, required=()):
    return {"local": {"properties": props, "required": list(required)}}


# Rules that hold from the start are violations. Rules that the requirement
# layer satisfies are "expected" rules: they are violations for each document
# except those in LAX_DOCS, for which they are info, reported without failing
# the build. A document leaves LAX_DOCS once its items satisfy them.
LAX_DOCS = []


def _docnames():
    """Every document of the specs, so that a new document is held to every rule unless it is
    listed in LAX_DOCS."""
    src = os.path.dirname(os.path.abspath(__file__))
    docs = []
    for d, dirs, files in os.walk(src):
        if d == src:  # exclude_patterns and ext name directories of src itself
            dirs[:] = [x for x in dirs if x not in exclude_patterns and x != "ext"]
        rel = os.path.relpath(d, src)
        docs += [posixpath.join(*rel.split(os.sep), f[: -len(".rst")]) if rel != "." else f[: -len(".rst")]
                 for f in files if f.endswith(".rst")]
    return sorted(docs)


DOCS = _docnames()


def _expected(rule):
    rules = []
    strict = [d for d in DOCS if d not in LAX_DOCS]
    for docs, severity in ((LAX_DOCS, "info"), (strict, "violation")):
        if not docs:
            continue
        r = dict(rule, id=f"{rule['id']}-{severity}", severity=severity)
        r["select"] = dict(rule["select"])
        r["select"]["properties"] = dict(rule["select"].get("properties", {}), docname={"enum": docs})
        rules.append(r)
    return rules


#: The principle that a requirement violates when its own source says it may change.
STABLE = "PRIN-STABLE"
_IDS = {"type": "array", "items": {"type": "string"}}


def _cat(c):
    return {"type": {"const": "expectation"}, "category": {"const": c}}


#: The ID prefix of each type whose IDs have one; the IDs of the other types start with a spec's code.
ID_PREFIXES = {"req": "REQ-", "rationale": "RAT-", "principle": "PRIN-"}

needs_schema_definitions = {
    "schemas": [
        *[
            {
                "id": f"{t}-id-prefix",
                "message": f"the ID of a {t} starts with {prefix}",
                "select": {"properties": {"type": {"const": t}}},
                "validate": {"local": {"properties": {"id": {"pattern": f"^{prefix}"}}}},
            }
            for t, prefix in ID_PREFIXES.items()
        ],
        {
            "id": "spec-code-id-prefix",
            "message": "the ID of an item of another type starts with the code of its spec",
            "select": {"properties": {"type": {"enum": [t["directive"] for t in needs_types if t["directive"] not in ID_PREFIXES]}}},
            "validate": {"local": {"properties": {"id": {"pattern": f"^({_SPECS})-"}}}},
        },
        {
            "id": "depends-on-none-alone",
            "message": "an item that depends on none names nothing else in depends_on",
            "select": {"properties": {"depends_on": {"contains": {"const": "none"}}}, "required": ["depends_on"]},
            "validate": {"local": {"properties": {"depends_on": {"maxItems": 1}}}},
        },
        {
            "id": "cli-form-of-api-expectation",
            "message": "cli_for links an expectation that depends on cli to one that does not",
            "select": {"properties": {"cli_for": {"type": "array", "minItems": 1}}, "required": ["cli_for"]},
            "validate": {
                "local": {"properties": {"depends_on": {"contains": {"const": "cli"}}}, "required": ["depends_on"]},
                "network": {"cli_for": {"items": _local({"depends_on": {"items": {"enum": [d for d in DEPENDS_ON if d != "cli"]}}})}},
            },
        },
        *[
            {
                "id": f"os-{p}-on-{p}",
                "message": f"an item that depends on os-{p} lists {p} in its platforms",
                "select": {"properties": {"depends_on": {"contains": {"const": f"os-{p}"}}}, "required": ["depends_on"]},
                "validate": {"local": {"properties": {"platforms": {"contains": {"const": p}}}, "required": ["platforms"]}},
            }
            for p in PLATFORMS
        ],
        {
            "id": "requirement-cites-a-source",
            "message": "a requirement cites a source",
            "select": {"properties": {"type": {"const": "req"}}},
            "validate": {"local": {"properties": {"source": {"type": "array", "minItems": 1}}, "required": ["source"]}},
        },
        {
            "id": "refines-a-stable-promise",
            "message": f"an expectation refines no requirement that violates {STABLE} (link it with provisional_on)",
            "select": {"properties": {"type": {"const": "expectation"}, "refines": {"type": "array", "minItems": 1}}, "required": ["refines"]},
            "validate": {"network": {"refines": {"items": _local({"violates": dict(_IDS, contains={"type": "string", "const": STABLE}, minContains=0, maxContains=0)})}}},
        },
        {
            "id": "provisional-on-an-unstable-promise",
            "message": f"provisional_on links only requirements that violate {STABLE}",
            "select": {"properties": {"type": {"const": "expectation"}, "provisional_on": {"type": "array", "minItems": 1}}, "required": ["provisional_on"]},
            "validate": {"network": {"provisional_on": {"items": _local({"violates": dict(_IDS, contains={"type": "string", "const": STABLE})}, ["violates"])}}},
        },
        *_expected(
            {
                "id": "host-requirement-is-needed",
                "message": "a host requirement is needed by an expectation",
                "select": {"properties": {"type": {"const": "hostreq"}}},
                "validate": {
                    "network_back": {"needs": {"type": "array", "contains": _local({"type": {"const": "expectation"}}), "minContains": 1}}
                },
            }
        ),
        *_expected(
            {
                "id": "detector-says-what-it-detects",
                "message": "a detector is an exact form, pins something, or links a rationale",
                "select": {"properties": _cat("detector")},
                "validate": {
                    "network": {
                        "detects": {
                            "contains": _local({"type": {"enum": ["expectation", "hostreq", "rationale"]}}),
                            "minContains": 1,
                        }
                    }
                },
            }
        ),
        *[
            r
            for _l in ("narrows", "concerns")
            for r in _expected(
                {
                    "id": f"{_l}-is-a-defect-link",
                    "message": "narrows and concerns are a defect's links",
                    "select": {
                        "properties": {"type": {"const": "expectation"}, _l: {"type": "array", "minItems": 1}},
                        "required": [_l],
                    },
                    "validate": {"local": {"properties": {"violates": {"type": "array", "minItems": 1}}, "required": ["violates"]}},
                }
            )
        ],
        *_expected(
            {
                "id": "defect-affects-an-expectation",
                "message": "a defect narrows or concerns at least one expectation",
                "select": {"properties": _cat("defect")},
                "validate": {
                    "network": {"affects": {"contains": _local({"type": {"const": "expectation"}}), "minContains": 1}}
                },
            }
        ),
    ]
}

_STABLE_VIOLATION = dict(_IDS, contains={"type": "string", "const": STABLE})
needs_schema_definitions["schemas"] += [
    {
        "id": "requirement-is-refined",
        "message": "a requirement is refined by an expectation",
        "severity": "info",
        "select": {"properties": {"type": {"const": "req"},
                                  "violates": dict(_STABLE_VIOLATION, minContains=0, maxContains=0)}},
        "validate": {"network_back": {"refines": {"type": "array", "contains": _local({"type": {"const": "expectation"}}), "minContains": 1}}},
    },
    {
        "id": "provisional-requirement-is-used",
        "message": f"a requirement that violates {STABLE} is the provisional basis of an expectation",
        "severity": "info",
        "select": {"properties": {"type": {"const": "req"}, "violates": _STABLE_VIOLATION}, "required": ["violates"]},
        "validate": {"network_back": {"provisional_on": {"type": "array", "contains": _local({"type": {"const": "expectation"}}), "minContains": 1}}},
    },
]

for _t in needs_types:
    if _t["directive"] == "ceiling":
        _t["back_links"] = {
            "within": {"schema": {"minItems": 1}, "severity": "violation", "description": "Every ceiling bounds an expectation"}
        }
    if _t["directive"] == "rationale":
        _t["back_links"] = {
            "rationale": {"schema": {"minItems": 1}, "severity": "violation", "description": "Every rationale explains an expectation"}
        }

needs_table_style = "table"
needs_table_columns = "ID;TITLE;TYPE;DEPENDS_ON"

# Findings of the expected rules at severity info are reported in
# _build/needs/schema_violations.json, and do not fail a build with -W.
suppress_warnings = ["sn_schema_info"]

# The tables of README.rst and network/README.rst are rendered from the
# vocabularies above.
needs_render_context = {
    "network_observed_from": {k: OBSERVED_FROM[k] for k in SUBSYSTEM_VOCABULARIES["network"]["observed_from"]},
    "network_when": {k: WHEN[k] for k in SUBSYSTEM_VOCABULARIES["network"]["when"]},
    "type_docs": TYPES,
    "depends_on": DEPENDS_ON,
    "platforms": PLATFORMS,
    "hostreq_kinds": HOSTREQ_KINDS,
    "source_kinds": SOURCE_KINDS,
    "categories": CATEGORIES,
}

# An item's ID never changes once it is merged, and the ID of a removed item
# that was merged is retired: it is listed in retired-ids.txt and never used
# again, so that a test or issue that cites it cannot come to mean another item.
# A line holds one ID; "#" starts a comment. A line that holds anything else would never match an
# ID, and so would leave the ID it means usable: the build refuses it.
with open("retired-ids.txt") as _f:
    RETIRED_IDS = {_i for _l in _f for _i in [_l.split("#", 1)[0].strip()] if _i}
for _i in RETIRED_IDS:
    if not re.fullmatch(needs_id_regex, _i):
        raise ValueError(f"retired-ids.txt: {_i!r} is not an ID")

needs_warnings = {"retired-id": f"id in {sorted(RETIRED_IDS)!r}"}
# Name the failing item in the warning itself, which -q does not hide.
needs_warnings_always_warn = True

# The vantage points that the observed_from of each subsystem's expectations names.
spec_subsystem_vocabularies = SUBSYSTEM_VOCABULARIES
