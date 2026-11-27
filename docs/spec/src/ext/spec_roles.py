"""Semantic roles for the behavior specs, and the checks of what they name.

Each role marks what a piece of inline code is: an API operation, a daemon
flag, a Kconfig symbol, an iptables chain, and so on. It renders as inline code.
Readers of the source, of needs.json and of the output all see the kind of
thing that the text names.

Where a source of truth exists in the repository, the build checks the text of
a role against it, and warns about text that it does not find there.
"""

import fnmatch
import glob
import os
import posixpath
import re
import subprocess

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

#: The names of errnos and signals that Linux defines, aliases included, as Python's errno and signal
#: modules give them in the toolchain container. They are listed here, not read from those modules,
#: so that the build gives the same result on every platform. A test checks them in the container.
ERRNOS = frozenset({
    "E2BIG", "EACCES", "EADDRINUSE", "EADDRNOTAVAIL", "EADV", "EAFNOSUPPORT", "EAGAIN", "EALREADY",
    "EBADE", "EBADF", "EBADFD", "EBADMSG", "EBADR", "EBADRQC", "EBADSLT", "EBFONT", "EBUSY",
    "ECANCELED", "ECHILD", "ECHRNG", "ECOMM", "ECONNABORTED", "ECONNREFUSED", "ECONNRESET",
    "EDEADLK", "EDEADLOCK", "EDESTADDRREQ", "EDOM", "EDOTDOT", "EDQUOT", "EEXIST", "EFAULT",
    "EFBIG", "EHOSTDOWN", "EHOSTUNREACH", "EIDRM", "EILSEQ", "EINPROGRESS", "EINTR", "EINVAL",
    "EIO", "EISCONN", "EISDIR", "EISNAM", "EKEYEXPIRED", "EKEYREJECTED", "EKEYREVOKED", "EL2HLT",
    "EL2NSYNC", "EL3HLT", "EL3RST", "ELIBACC", "ELIBBAD", "ELIBEXEC", "ELIBMAX", "ELIBSCN",
    "ELNRNG", "ELOOP", "EMEDIUMTYPE", "EMFILE", "EMLINK", "EMSGSIZE", "EMULTIHOP", "ENAMETOOLONG",
    "ENAVAIL", "ENETDOWN", "ENETRESET", "ENETUNREACH", "ENFILE", "ENOANO", "ENOBUFS", "ENOCSI",
    "ENODATA", "ENODEV", "ENOENT", "ENOEXEC", "ENOKEY", "ENOLCK", "ENOLINK", "ENOMEDIUM", "ENOMEM",
    "ENOMSG", "ENONET", "ENOPKG", "ENOPROTOOPT", "ENOSPC", "ENOSR", "ENOSTR", "ENOSYS", "ENOTBLK",
    "ENOTCONN", "ENOTDIR", "ENOTEMPTY", "ENOTNAM", "ENOTRECOVERABLE", "ENOTSOCK", "ENOTSUP",
    "ENOTTY", "ENOTUNIQ", "ENXIO", "EOPNOTSUPP", "EOVERFLOW", "EOWNERDEAD", "EPERM", "EPFNOSUPPORT",
    "EPIPE", "EPROTO", "EPROTONOSUPPORT", "EPROTOTYPE", "ERANGE", "EREMCHG", "EREMOTE", "EREMOTEIO",
    "ERESTART", "ERFKILL", "EROFS", "ESHUTDOWN", "ESOCKTNOSUPPORT", "ESPIPE", "ESRCH", "ESRMNT",
    "ESTALE", "ESTRPIPE", "ETIME", "ETIMEDOUT", "ETOOMANYREFS", "ETXTBSY", "EUCLEAN", "EUNATCH",
    "EUSERS", "EWOULDBLOCK", "EXDEV", "EXFULL",
})
SIGNALS = frozenset({
    "SIGABRT", "SIGALRM", "SIGBUS", "SIGCHLD", "SIGCLD", "SIGCONT", "SIGFPE", "SIGHUP", "SIGILL",
    "SIGINT", "SIGIO", "SIGIOT", "SIGKILL", "SIGPIPE", "SIGPOLL", "SIGPROF", "SIGPWR", "SIGQUIT",
    "SIGRTMAX", "SIGRTMIN", "SIGSEGV", "SIGSTKFLT", "SIGSTOP", "SIGSYS", "SIGTERM", "SIGTRAP",
    "SIGTSTP", "SIGTTIN", "SIGTTOU", "SIGURG", "SIGUSR1", "SIGUSR2", "SIGVTALRM", "SIGWINCH",
    "SIGXCPU", "SIGXFSZ",
})
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
        spec = yaml.safe_load(f) or {}
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


#: A call of a pflag FlagSet that defines a flag, or marks one, by its name: on a FlagSet named
#: like ``flags``, or on the one that a cobra command's ``Flags()`` or ``PersistentFlags()`` gives.
FLAG_CALL = re.compile(
    r"(?:\b[A-Za-z_]*[Ff]lags|Flags\(\))\."
    r"(?P<method>[A-Z][A-Za-z0-9]*Var|Var|Bool|String|Int|Int64|Uint|Uint64|Float64|Duration|IP|Count|StringSlice"
    r"|StringArray|IntSlice)(?P<p>P?)\(|(?:\b[A-Za-z_]*[Ff]lags|Flags\(\))\.(?P<mark>MarkDeprecated|MarkHidden)\("
)
GO_STRING = re.compile(r'"(?:[^"\\\n]|\\.)*"|`[^`]*`')
#: A Go constant or variable set to a string literal: ``Name = "value"``, also in a const block.
GO_STRING_DEF = re.compile(r'(?<![\w.])([A-Za-z_]\w*)\s*(?:string\s*)?=\s*"((?:[^"\\\n]|\\.)*)"')
#: A concatenation of an identifier and a string literal, ``netlabel.Prefix + ".driver"``.
GO_CONCAT = re.compile(r'(?:(?<![\w.])([A-Za-z_]\w*)\s*(?:string\s*)?=\s*)?(?<![\w.])(?:[A-Za-z_]\w*\.)?([A-Za-z_]\w*)\s*\+\s*"(\.[^"\\\n]*)"')
FLAG_NAME = re.compile(r"[a-z0-9][a-z0-9-]*")


def _go_files(repo, *dirs):
    for d in dirs:
        for f in sorted(glob.glob(os.path.join(repo, d, "**", "*.go"), recursive=True)):
            if not f.endswith("_test.go"):
                with open(f, encoding="utf-8", errors="replace") as fh:
                    yield fh.read()


def _call_args(text, start):
    """The arguments of the call whose "(" ends at start, as source text, split at the commas that
    are not inside a nested call, literal or composite."""
    args, depth, i, begin = [], 1, start, start
    while i < len(text) and depth:
        c = text[i]
        if c in "\"`":
            lit = GO_STRING.match(text, i)
            i = lit.end() if lit else i + 1
            continue
        if c in "([{":
            depth += 1
        elif c in ")]}":
            depth -= 1
            if not depth:
                break
        elif c == "," and depth == 1:
            args.append(text[begin:i].strip())
            begin = i + 1
        i += 1
    args.append(text[begin:i].strip())
    return args


def _dockerd_flags(repo, *dirs):
    """The names of the flags that dockerd defines, and their shorthands, such as "-D". The name is
    the argument after the variable of an ``XVar`` call and the first argument of any other call of
    FLAG_CALL, and the shorthand of an ``XP`` call follows the name: a string literal, or a constant
    set to one, such as FlagTLS."""
    texts = list(_go_files(repo, *dirs))
    consts = {name: value for text in texts for name, value in GO_STRING_DEF.findall(text)}
    def value(arg):
        return arg[1:-1] if GO_STRING.fullmatch(arg) else consts.get(arg.rsplit(".", 1)[-1], "")

    flags = set()
    for text in texts:
        for m in FLAG_CALL.finditer(text):
            args = _call_args(text, m.end())
            i = 1 if m.group("method") and m.group("method").endswith("Var") else 0
            if len(args) > i and FLAG_NAME.fullmatch(value(args[i])):
                flags.add(value(args[i]))
            if m.group("p") and len(args) > i + 1 and re.fullmatch(r"[A-Za-z]", value(args[i + 1])):
                flags.add("-" + value(args[i + 1]))
    return flags


def _go_option_names(repo, *dirs):
    """The string literals of the Go sources; the values of their concatenations of a named string
    and a literal that starts with ".", such as DriverPrefix + ".mtu", where DriverPrefix is Prefix +
    ".driver" and Prefix is "com.docker.network"; and the suffixes of those concatenations, such as
    ".overlay.vxlanid_list". A name has each value that some package gives it."""
    texts = list(_go_files(repo, *dirs))
    literals = {m.group(0)[1:-1] for text in texts for m in GO_STRING.finditer(text) if m.group(0)[0] == '"'}
    values = {}
    for text in texts:
        for name, value in GO_STRING_DEF.findall(text):
            values.setdefault(name, set()).add(value)
    concats = [m.groups() for text in texts for m in GO_CONCAT.finditer(text)]
    names, suffixes = set(), set()
    for _ in range(4):  # a concatenation of a concatenation, a few levels deep
        for target, prefix, suffix in concats:
            for value in list(values.get(prefix, ())):
                if target:
                    values.setdefault(target, set()).add(value + suffix)
                # only the concatenations that make Docker's own options, not file names such as
                # path + ".hash"
                if value.startswith("com.docker."):
                    names.add(value + suffix)
                    suffixes.add(suffix)
    return literals | names, suffixes


def _known_option(opt, options, bare):
    """Whether opt is an option that the Go sources name: in full, or as the end of a driver's
    suffix of a concatenation, such as vxlanid_list, the end of ".overlay.vxlanid_list". A name without a dot,
    such as parent, is one of bare, libnetwork's string literals: the daemon's other literals
    include words such as sock, which name no option. A word that is a literal of libnetwork,
    such as json, passes."""
    names, suffixes = options
    if not opt or opt.startswith(".") or opt.endswith("."):
        return False
    # the end of a driver's own suffix, such as ".overlay.vxlanid_list": not "mtu", the end of ".mtu"
    return (opt in names if "." in opt else opt in bare) or any(
        s.endswith("." + opt) and s.count(".") > 1 for s in suffixes)


def _docker_chains(strings):
    """The names among strings that have the form of Docker's own iptables chains, DOCKER-INGRESS,
    and of its nftables chains and tables, docker-overlay."""
    ipt = {s for s in strings if re.fullmatch(r"DOCKER(?:-[A-Z0-9]+)*", s)}
    nft = {s for s in strings if re.fullmatch(r"docker(?:-[a-z0-9]+)+", s)}
    return ipt, nft


def _tracked_paths(repo):
    """The files that git tracks in repo, and their directories; None where git cannot tell, as
    for a linked worktree whose Git directory is outside the container."""
    r = subprocess.run(["git", "-c", "safe.directory=*", "-C", repo, "ls-files", "-z"], capture_output=True)
    if r.returncode != 0:
        return None
    paths = {"."}
    for f in r.stdout.decode("utf-8", "replace").split("\0"):
        while f and f not in paths:
            paths.add(f)
            f = posixpath.dirname(f)
    return paths


def _repo_path_exists(repo, text, tracked=None):
    """Whether text names a path in the repository, compared case by case, as on Linux, and one
    that git tracks where tracked, from _tracked_paths, is given: not a build output or a file of
    the developer's own. A path that ends with "/" names a directory."""
    if text.startswith("/"):
        return False
    is_dir = text.endswith("/")
    rel = posixpath.normpath(text)
    if rel == ".." or rel.startswith("../"):
        return False
    if glob.has_magic(rel):
        # glob compares a part without wildcards as the file system does: case-insensitively on macOS
        return any(_repo_path_exists(repo, os.path.relpath(m, repo) + ("/" if is_dir else ""), tracked)
                   for m in glob.glob(os.path.join(repo, rel)))
    if tracked is not None and rel not in tracked:
        return False
    path = repo
    for part in rel.split("/"):
        if part == ".":
            continue
        try:
            if part not in os.listdir(path):
                return False
        except NotADirectoryError:
            return False
        path = os.path.join(path, part)
    return os.path.isdir(path) if is_dir else True


def _golden_names(repo, subdir):
    """The tables and the chains that the reference rulesets in subdir show."""
    tables, chains = set(), set()
    for f in glob.glob(os.path.join(repo, "integration", "network", "bridge", subdir, "generated", "*.md")):
        with open(f) as fh:
            text = fh.read()
        chains.update(re.findall(r"(?m)^\s*Chain ([A-Za-z0-9_-]+)", text))
        chains.update(re.findall(r"(?m)^\s*-N ([A-Za-z0-9_-]+)", text))
        chains.update(re.findall(r"chain ([A-Za-z0-9_-]+) \{", text))
        tables.update(re.findall(r"table [a-z0-9]+ ([A-Za-z0-9_-]+) \{", text))
    return tables, chains


def check(app, env):
    uses = getattr(env, "spec_role_uses", [])
    repo = app.config.spec_repo_root
    if not repo or not os.path.isdir(repo):
        return
    ops = _openapi_ops(repo)
    kconfig = set()
    try:
        from sphinx_needs.data import SphinxNeedsData

        for need in SphinxNeedsData(env).get_needs_view().values():
            kconfig.update(need.get("kernel_options") or [])
    except Exception:
        pass
    drvopts = _go_option_names(repo, "daemon/libnetwork", "daemon", "api/types")
    dockerd = _dockerd_flags(repo, "cmd/dockerd", "daemon/command", "daemon/config")
    ipt = _golden_names(repo, "iptablesdoc")[1] | BUILTIN_IPT_CHAINS | set(app.config.spec_legacy_chains)
    nft_tables, nft_chains = _golden_names(repo, "nftablesdoc")
    seen = set()
    # Chains and tables that no reference ruleset shows, such as those of overlay networks and
    # ingress, are found among libnetwork's string literals of the form of Docker's own names.
    tracked = _tracked_paths(repo)
    libnet = _go_strings(repo, "daemon/libnetwork")
    libnet_ipt, libnet_nft = _docker_chains(libnet)
    for role, text, docname, lineno in uses:
        text = " ".join(text.split())
        key = (role, text)
        if key in seen:
            continue
        seen.add(key)
        problem = None
        if role == "api-op" and ops is not None:
            m = re.match(r"^([A-Z]+) (/\S*)$", text)
            if not m or not any(meth == m.group(1) and rx.match(m.group(2).split("?")[0]) for meth, rx in ops):
                problem = "no such operation in api/openapi.yaml"
        elif role == "kconfig":
            sym = text[len("CONFIG_"):] if text.startswith("CONFIG_") else text
            # the README names the prefix itself, as :kconfig:`CONFIG_`
            if sym and sym not in kconfig:
                problem = "not in the kernel_options of any host requirement"
        elif role == "drvopt":
            opt = text.split("=", 1)[0]
            if not (any(fnmatch.fnmatchcase(n, opt) for n in drvopts[0]) if "*" in opt
                    else _known_option(opt, drvopts, libnet)):
                problem = "no Go string literal in the daemon names this option"
        elif role == "dockerd-flag":
            flag = text.split("=", 1)[0].split(" ", 1)[0]
            flag = flag[2:] if flag.startswith("--") else flag
            if flag not in dockerd:
                problem = "dockerd defines no such flag"
        elif role == "ipt-chain":
            if text not in ipt and text not in libnet_ipt:
                problem = "no reference ruleset in integration/network/bridge/iptablesdoc has this chain"
        elif role in ("nft-chain", "nft-table") and (nft_tables or nft_chains):
            # libnetwork's names of the form of Docker's own count for both: their form does not
            # say which they are
            if text not in (nft_tables if role == "nft-table" else nft_chains) and text not in libnet_nft:
                problem = "no reference ruleset in integration/network/bridge/nftablesdoc has this name"
        elif role == "repo-path":
            if not _repo_path_exists(repo, text, tracked):
                problem = "no such path in the repository, or one that git does not track"
        elif role == "errno" and text not in ERRNOS:
            problem = "not a known errno name"
        elif role == "signal" and text not in SIGNALS:
            problem = "not a known signal name"
        if problem:
            LOGGER.warning(f":{role}:`{text}`: {problem}", location=(docname, lineno), type="spec", subtype="role")


NEED_REF = re.compile(r":need:`(?:[^`<]*<)?([A-Z0-9-]+)>?`")

#: The category of an expectation, by the first of these links that it has; else detector.
CATEGORY_LINKS = (("violates", "defect"), ("refines", "contract"), ("provisional_on", "provisional"))


def _category(need):
    if need["type"] != "expectation":
        return None
    return next((c for link, c in CATEGORY_LINKS if need.get(link)), "detector")


def set_categories(app, needs):
    """Give each expectation the category that its links give, before the needextend directives,
    whose filters can select by it. A written category is a warning."""
    for need in needs.values():
        if need.get("category"):
            LOGGER.warning(f"{need['id']}: category is written; it is computed from the links, never written",
                           location=(need["docname"], need["lineno"]), type="spec", subtype="category")
        need["category"] = _category(need)


def check_links(app, needs):
    """Rules README states that the schema cannot express:

    - an item's category is still the one that its links give after the needextend directives: a
      needextend neither writes a category nor adds a link that changes it;
    - a host requirement's kernel options are written without their CONFIG_ prefix;
    - an expectation's :need: references to ceilings agree with its ``within`` links (each
      referenced ceiling is linked);
    - an expectation does not need a host requirement of none of the expectation's platforms.
    """
    for need in needs.values():
        loc = (need["docname"], need["lineno"])
        category = _category(need)
        if (need.get("category") or None) != category:
            LOGGER.warning(f"{need['id']}: a needextend makes the category {need.get('category')!r} where its "
                           f"links make it {category!r}: the category is computed before the needextend "
                           f"directives, which neither write it nor change the links it depends on",
                           location=loc, type="spec", subtype="category")
        for option in need.get("kernel_options") or []:
            if option.startswith("CONFIG_"):
                LOGGER.warning(f"{need['id']}: kernel option {option} is written with its CONFIG_ prefix",
                               location=loc, type="spec", subtype="kconfig")
        if need["type"] != "expectation":
            continue
        within = set(need.get("within") or [])
        for ref in NEED_REF.findall(need.get("content") or ""):
            target = needs.get(ref)
            if target is not None and target["type"] == "ceiling" and ref not in within:
                LOGGER.warning(f"{need['id']}: names the ceiling {ref} but does not link it with within",
                               location=loc, type="spec", subtype="ceiling")
        platforms = set(need.get("platforms") or [])
        for req in need.get("needs") or []:
            target = needs.get(req)
            if target is not None and platforms and not platforms & set(target.get("platforms") or []):
                LOGGER.warning(f"{need['id']}: needs {req}, a host requirement of none of its platforms",
                               location=loc, type="spec", subtype="platform")


def setup(app):
    app.connect("needs-before-post-processing", set_categories)
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
