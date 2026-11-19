"""Semantic roles for the behavior specs, and the checks of what they name.

Each role marks what a piece of inline code is: an API operation, a daemon
flag, a Kconfig symbol, an iptables chain, and so on. It renders as inline code.
Readers of the source, of needs.json and of the output all see the kind of
thing that the text names.

Where a source of truth exists in the repository, the build checks the text of
a role against it, and warns about text that it does not find there.
"""

import glob
import os
import re

from docutils import nodes
from sphinx.util import logging

LOGGER = logging.getLogger(__name__)

#: role name: what it marks. "file" and "program" are Sphinx's own roles.
ROLES = {
    "api-op": "An Engine API operation, METHOD /path",
    "api-field": "A field of an Engine API object",
    "api-query": "A query parameter or filter term of an Engine API request",
    "api-value": "An Engine API field with its value, written field=value",
    "api-event": "An action of an Engine API event",
    "api-error": "Text of the message of an Engine API error response",
    "cli": "A docker CLI command",
    "cli-flag": "A flag of a docker CLI command",
    "dockerd-flag": "A flag of dockerd",
    "daemon-json": "A daemon setting, by its daemon.json key",
    "env": "An environment variable",
    "log": "Text of an entry in the daemon log",
    "driver": "A network or IPAM driver",
    "network": "A network, by name",
    "drvopt": "A driver, network or endpoint option",
    "kconfig": "A Linux kernel configuration option, without CONFIG_",
    "sysctl": "A sysctl",
    "errno": "An errno name",
    "signal": "A signal name",
    "cap": "A Linux capability",
    "netdev": "A network device, by name",
    "link-kind": "A kind of Linux network link",
    "ipt-table": "An iptables or ip6tables table",
    "ipt-chain": "An iptables or ip6tables chain",
    "ipt-target": "An iptables or ip6tables target, or a rule fragment ending in one",
    "nft-table": "An nftables table",
    "nft-chain": "An nftables chain",
    "fwd-zone": "A firewalld zone",
    "fwd-policy": "A firewalld policy",
    "dns-name": "A DNS name, or a pattern of names",
    "rr-type": "A DNS resource record type",
    "rcode": "A DNS response code",
    "resolv-opt": "A resolv.conf directive or option",
    "repo-path": "A path in the moby repository",
}

ERRNOS = {
    "EPERM", "ENOENT", "EINTR", "EIO", "EAGAIN", "ENOMEM", "EACCES", "EBUSY", "EEXIST", "ENODEV",
    "EINVAL", "ENOSPC", "EPIPE", "ENOSYS", "ENOTSUP", "EOPNOTSUPP", "EADDRINUSE", "EADDRNOTAVAIL",
    "ENETDOWN", "ENETUNREACH", "ECONNABORTED", "ECONNRESET", "ENOBUFS", "ETIMEDOUT",
    "ECONNREFUSED", "EHOSTDOWN", "EHOSTUNREACH", "EMSGSIZE", "EPROTONOSUPPORT", "EAFNOSUPPORT",
}
SIGNALS = {
    "SIGHUP", "SIGINT", "SIGQUIT", "SIGKILL", "SIGUSR1", "SIGUSR2", "SIGPIPE", "SIGTERM", "SIGCHLD",
    "SIGCONT", "SIGSTOP",
}
#: Chains that the kernel defines, and chains of earlier releases that cleanup
#: expectations name, which no current reference ruleset shows.
BUILTIN_IPT_CHAINS = {"INPUT", "FORWARD", "OUTPUT", "PREROUTING", "POSTROUTING"}


class _Uses:
    def __init__(self):
        self.items = []  # (role, text, docname, lineno)


def _make_role(name):
    def role(typ, rawtext, text, lineno, inliner, options=None, content=None):
        text = nodes.unescape(text)
        node = nodes.literal(rawtext, text, classes=["spec-role", f"spec-{name}"])
        env = inliner.document.settings.env
        uses = getattr(env, "spec_role_uses", None)
        if uses is None:
            uses = env.spec_role_uses = []
        uses.append((name, text, env.docname, lineno))
        return [node], []

    return role


def _openapi_ops(repo):
    path = os.path.join(repo, "api", "openapi.yaml")
    if not os.path.exists(path):
        return None
    import yaml

    with open(path) as f:
        spec = yaml.safe_load(f)
    ops = []
    for p, methods in spec.get("paths", {}).items():
        rx = "^" + re.sub(r"\\\{[^}]*\\\}", "[^/ ]+", re.escape(p)) + "$"
        for m in methods:
            if m in ("get", "post", "put", "delete", "head", "patch"):
                ops.append((m.upper(), re.compile(rx)))
    return ops


def _go_strings(repo, *dirs):
    strings = set()
    for d in dirs:
        for f in glob.glob(os.path.join(repo, d, "**", "*.go"), recursive=True):
            if f.endswith("_test.go"):
                continue
            with open(f, encoding="utf-8", errors="replace") as fh:
                for m in re.finditer(r'"([^"\\\n]*)"', fh.read()):
                    strings.add(m.group(1))
    return strings


def _golden_chains(repo, subdir):
    chains = set()
    for f in glob.glob(os.path.join(repo, "integration", "network", "bridge", subdir, "generated", "*.md")):
        with open(f) as fh:
            text = fh.read()
        chains.update(re.findall(r"(?m)^\s*Chain ([A-Za-z0-9_-]+)", text))
        chains.update(re.findall(r"(?m)^\s*-N ([A-Za-z0-9_-]+)", text))
        chains.update(re.findall(r"chain ([A-Za-z0-9_-]+) \{", text))
        chains.update(re.findall(r"table [a-z0-9]+ ([A-Za-z0-9_-]+) \{", text))
    return chains


def check(app, env):
    uses = getattr(env, "spec_role_uses", [])
    repo = app.config.spec_repo_root
    if not repo or not os.path.isdir(repo):
        return
    ops = _openapi_ops(repo)
    kconfig = set()
    for need in getattr(env, "_spec_kernel_options", []):
        kconfig.update(need)
    try:
        from sphinx_needs.data import SphinxNeedsData

        for need in SphinxNeedsData(env).get_needs_view().values():
            kconfig.update(need.get("kernel_options") or [])
    except Exception:
        pass
    drvopts = _go_strings(repo, "daemon/libnetwork", "daemon", "api/types")
    dockerd = _go_strings(repo, "cmd/dockerd", "daemon/command", "daemon/config")
    ipt = _golden_chains(repo, "iptablesdoc") | BUILTIN_IPT_CHAINS | set(app.config.spec_legacy_chains)
    nft = _golden_chains(repo, "nftablesdoc")
    seen = set()
    libnet = _go_strings(repo, "daemon/libnetwork")
    for role, text, docname, lineno in uses:
        text = " ".join(text.split())
        key = (role, text)
        if key in seen:
            continue
        seen.add(key)
        problem = None
        if role == "api-op" and ops is not None:
            m = re.match(r"^([A-Z]+) (/\S*)", text)
            if not m or not any(meth == m.group(1) and rx.match(m.group(2).split("?")[0]) for meth, rx in ops):
                problem = "no such operation in api/openapi.yaml"
        elif role == "kconfig":
            sym = text[len("CONFIG_"):] if text.startswith("CONFIG_") else text
            if sym and sym not in kconfig:
                problem = "not in the kernel_options of any host requirement"
        elif role == "drvopt":
            opt = text.split("=", 1)[0]
            if "*" not in opt and opt not in drvopts and not any(
                len(s) >= 6 and (opt.endswith(s) or s.endswith("." + opt)) for s in drvopts
            ):
                problem = "no Go string literal in the daemon names this option"
        elif role == "dockerd-flag":
            flag = text.split("=", 1)[0].split(" ", 1)[0].lstrip("-")
            if flag not in dockerd:
                problem = "dockerd defines no such flag"
        elif role == "ipt-chain":
            if text not in ipt and text not in libnet:
                problem = "no reference ruleset in integration/network/bridge/iptablesdoc has this chain"
        elif role in ("nft-chain", "nft-table") and nft:
            if text not in nft and text not in libnet:
                problem = "no reference ruleset in integration/network/bridge/nftablesdoc has this name"
        elif role == "repo-path":
            path = os.path.join(repo, text.rstrip("/"))
            if not (glob.glob(path) if glob.has_magic(text) else os.path.exists(path)):
                problem = "no such path in the repository"
        elif role == "errno" and text not in ERRNOS:
            problem = "not a known errno name"
        elif role == "signal" and text not in SIGNALS:
            problem = "not a known signal name"
        if problem:
            LOGGER.warning(f":{role}:`{text}`: {problem}", location=(docname, lineno), type="spec", subtype="role")


CEILING_REF = re.compile(r":need:`(?:[^`<]*<)?(SWARM-CEIL-[A-Z0-9-]+)>?`")


def check_links(app, needs):
    """Rules README states that the schema cannot express:

    - an expectation's :need: references to ceilings agree with its ``within`` links (each
      referenced ceiling is linked);
    - an expectation does not need a host requirement of none of the expectation's platforms.
    """
    for need in needs.values():
        if need["type"] != "expectation":
            continue
        loc = (need["docname"], need["lineno"])
        within = set(need.get("within") or [])
        for ceiling in CEILING_REF.findall(need.get("content") or ""):
            if ceiling not in within:
                LOGGER.warning(f"{need['id']}: names the ceiling {ceiling} but does not link it with within",
                               location=loc, type="spec", subtype="ceiling")
        platforms = set(need.get("platforms") or [])
        for req in need.get("needs") or []:
            target = needs.get(req)
            if target is not None and platforms and not platforms & set(target.get("platforms") or []):
                LOGGER.warning(f"{need['id']}: needs {req}, a host requirement of none of its platforms",
                               location=loc, type="spec", subtype="platform")


def setup(app):
    app.connect("needs-before-sealing", check_links)
    app.add_config_value("spec_repo_root", "", "env")
    app.add_config_value("spec_legacy_chains", [], "env")
    for name in ROLES:
        app.add_role(name, _make_role(name))

    def purge(app, env, docname):
        env.spec_role_uses = [u for u in getattr(env, "spec_role_uses", []) if u[2] != docname]

    def merge(app, env, docnames, other):
        env.spec_role_uses = getattr(env, "spec_role_uses", []) + [
            u for u in getattr(other, "spec_role_uses", []) if u[2] in docnames
        ]

    app.connect("env-purge-doc", purge)
    app.connect("env-merge-info", merge)
    app.connect("env-check-consistency", check)
    return {"parallel_read_safe": True, "parallel_write_safe": True}
