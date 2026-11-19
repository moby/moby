"""Tests of the parts of the extensions in src/ext that need no build: the grammar of observed_from,
and how the role checks find names in the repository."""

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src" / "ext"))

import md_needs  # noqa: E402
import observed_from  # noqa: E402
import spec_roles  # noqa: E402
from docutils import nodes  # noqa: E402

CODES = ["task", "ctr", "node", "underlay", "client", "api:mgr", "api:wkr", "api:active", "api:solo", "api", "log"]


class ObservedFrom(unittest.TestCase):
    parse = staticmethod(observed_from.parser(CODES))

    def test_parts(self):
        cases = {
            "api:mgr": [([], ["api:mgr"], None)],
            "task, ctr": [([], ["task"], None), ([], ["ctr"], None)],
            "underlay → task or ctr": [(["underlay"], ["task", "ctr"], None)],
            "task or ctr → underlay": [(["task", "ctr"], ["underlay"], None)],
            "task on two nodes": [([], ["task"], "on two nodes")],
            "task or ctr on a task-free node": [([], ["task", "ctr"], "on a task-free node")],
            "api:mgr, api:wkr (on the task's node)": [([], ["api:mgr"], None), ([], ["api:wkr"], "(on the task's node)")],
            "underlay (outer destination address)": [([], ["underlay"], "(outer destination address)")],
            "api, ctr → ctr": [([], ["api"], None), (["ctr"], ["ctr"], None)],
            # a narrowing in parentheses can hold "," and "→"
            "underlay (outer source, destination address)": [([], ["underlay"], "(outer source, destination address)")],
            "underlay (a → b) → task": [(["underlay"], ["task"], "(a → b)")],
        }
        for value, want in cases.items():
            with self.subTest(value=value):
                parts, errors = self.parse(value)
                self.assertEqual(errors, [])
                self.assertEqual([(p["sent_from"], p["at"], p["narrowing"]) for p in parts], want)

    def test_errors(self):
        cases = {
            "nowhere": "does not start with a vantage point",
            "api:manager": "does not start with a vantage point",
            "task ctr": "names two vantage points",
            "ctr,": "empty vantage point",
            "→ ctr": "empty vantage point",
            "task → ctr → node": "more than one '→'",
            "api:mgr (x), apu": "does not start with a vantage point",
            # words that are not a narrowing: a typo of a separator or of a code
            "underlay -> task": "in parentheses or start with a preposition",
            "task or ctrr": "in parentheses or start with a preposition",
            "task  or ctr": "in parentheses or start with a preposition",
            "api (on x) or ctr": "in parentheses or start with a preposition",
            # a word that starts like a code is not a second code
            "ctr node-local": "in parentheses or start with a preposition",
            # an arrow in words, and a vantage point in a narrowing
            "underlay to task": "in parentheses or start with a preposition",
            "task from client": "in parentheses or start with a preposition",
            "api:mgr on api:wkr": "in parentheses or start with a preposition",
            "api:mgr on api:wkr.": "in parentheses or start with a preposition",
            "api:mgr on api:wkr's node": "in parentheses or start with a preposition",
            # a parenthesis that is not closed would hide the separators after it
            "ctr on node (1, api": "unbalanced parentheses",
            "ctr on node 1), api": "unbalanced parentheses",
        }
        for value, want in cases.items():
            with self.subTest(value=value):
                _, errors = self.parse(value)
                self.assertEqual(len(errors), 1, errors)
                self.assertIn(want, errors[0])


def _go_repo(files):
    """A temporary repository holding files, a map of path to content."""
    tmp = tempfile.TemporaryDirectory()
    for path, text in files.items():
        os.makedirs(os.path.join(tmp.name, os.path.dirname(path)), exist_ok=True)
        Path(tmp.name, path).write_text(text)
    return tmp


class DriverOptions(unittest.TestCase):
    FILES = {
        "netlabel/labels.go": """package netlabel
const (
	Prefix = "com.docker.network"
	DriverPrefix = Prefix + ".driver"
	DriverMTU = DriverPrefix + ".mtu"
	OverlayVxlanIDList = DriverPrefix + ".overlay.vxlanid_list"
	EndpointSysctls = Prefix + ".endpoint.sysctls"
)
""",
        # another package's Prefix: each value counts
        "other/other.go": """package other
const Prefix = "com.example"
func f(id string) string { return id + ".json" + "daemon.json" + "parent" + "sock" + "com.docker.swarm." }
func g() string { return ".driver.mtu" }
""",
    }

    def setUp(self):
        self.tmp = _go_repo(self.FILES)
        self.options = spec_roles._go_option_names(self.tmp.name, "netlabel", "other")
        self.bare = {"parent", "dsr"}

    def tearDown(self):
        self.tmp.cleanup()

    def test_known(self):
        for opt in ("com.docker.network", "com.docker.network.driver", "com.docker.network.driver.mtu",
                    "com.docker.network.endpoint.sysctls", "vxlanid_list", "parent"):
            with self.subTest(opt=opt):
                self.assertTrue(spec_roles._known_option(opt, self.options, self.bare))

    def test_unknown(self):
        for opt in ("com.docker.network.bogus.driver", "com.docker.network.driver.mtu.bak", "totally.made.up.mtu",
                    "json", "daemon", "com.docker.network.bridge.host_ipv4", "sysctls.endpoint",
                    # a word that is a literal, but not one of libnetwork's
                    "sock",
                    # the end of a suffix that is not a driver's own
                    "mtu",
                    # a literal that ends or starts with a dot, and the concatenation of another
                    # package's Prefix
                    "com.docker.swarm.", ".driver.mtu", "com.example.driver"):
            with self.subTest(opt=opt):
                self.assertFalse(spec_roles._known_option(opt, self.options, self.bare))


class GoDefinitions(unittest.TestCase):
    def test_a_field_is_not_a_definition(self):
        self.assertEqual(spec_roles.GO_STRING_DEF.findall('t.Prefix = ""\nPrefix = "com.docker.network"'),
                         [("Prefix", "com.docker.network")])
        self.assertEqual(spec_roles.GO_CONCAT.findall('x.y = t.Prefix + ".z"\nA = netlabel.Prefix + ".b"'),
                         [("", "Prefix", ".z"), ("A", "Prefix", ".b")])


class Chains(unittest.TestCase):
    def test_docker_chains(self):
        ipt, nft = spec_roles._docker_chains({"DOCKER", "DOCKER-INGRESS", "docker-overlay", "docker0", "docker",
                                              "docker_gwbridge", "DOCKER_TEST_NO_FIREWALLD", "Docker-x"})
        self.assertEqual((ipt, nft), ({"DOCKER", "DOCKER-INGRESS"}, {"docker-overlay"}))


class RepoPaths(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = self.tmp.name
        os.makedirs(os.path.join(self.repo, "api", "docs"))
        Path(self.repo, "api", "openapi.yaml").touch()

    def tearDown(self):
        self.tmp.cleanup()

    def test_in_the_repository(self):
        for path in ("api", "api/openapi.yaml", "api/docs", "api/docs/", "./api/../api/openapi.yaml", "api/*.yaml",
                     "api/*/", "."):
            with self.subTest(path=path):
                self.assertTrue(spec_roles._repo_path_exists(self.repo, path))

    def test_not_in_the_repository(self):
        for path in ("/etc/passwd", "/", "..", "../..", "../outside", "API/openapi.yaml", "api/OpenAPI.yaml",
                     "API/*.yaml", "api/openapi.yaml/x", "api/openapi.yaml/", "api/missing", "api/*.json"):
            with self.subTest(path=path):
                self.assertFalse(spec_roles._repo_path_exists(self.repo, path))


class RepoPathCaseInsensitive(unittest.TestCase):
    """On a file system that ignores case, as macOS's does, simulated for a repository whose names
    are all lowercase."""

    def test_case(self):
        from unittest import mock

        with tempfile.TemporaryDirectory() as repo:
            os.makedirs(os.path.join(repo, "api"))
            Path(repo, "api", "openapi.yaml").touch()

            def fold(f):
                return lambda p, *a, **kw: f(repo + p[len(repo):].lower() if p.startswith(repo) else p, *a, **kw)

            with mock.patch("os.path.exists", fold(os.path.exists)), mock.patch("os.path.isdir", fold(os.path.isdir)), \
                    mock.patch("os.path.lexists", fold(os.path.lexists)):
                self.assertTrue(os.path.exists(os.path.join(repo, "API", "OpenAPI.yaml")))
                self.assertTrue(spec_roles._repo_path_exists(repo, "api/openapi.yaml"))
                for path in ("API/openapi.yaml", "api/OpenAPI.yaml", "API/"):
                    with self.subTest(path=path):
                        self.assertFalse(spec_roles._repo_path_exists(repo, path))


class RepoPathCase(unittest.TestCase):
    """On the repository itself, which on macOS is on a file system that ignores case."""

    REPO = str(Path(__file__).resolve().parents[3])

    def test_case(self):
        self.assertTrue(spec_roles._repo_path_exists(self.REPO, "api/openapi.yaml"))
        for path in ("API/openapi.yaml", "API/*.yaml", "Api/openapi.*", "DAEMON/libnetwork/*.go"):
            with self.subTest(path=path):
                self.assertFalse(spec_roles._repo_path_exists(self.REPO, path))


class DockerdFlags(unittest.TestCase):
    GO = '''package config

func installFlags(flags *pflag.FlagSet, conf *Config) {
	flags.Var(opts.NewNamedListOptsRef("dns-opts", &conf.DNSOptions, nil), "dns-opt", "DNS options to use")
	flags.StringVar(&conf.Root, "data-root", defaultDataRoot, "Root directory of persistent Docker state")
	flags.BoolVarP(&conf.Debug, "debug", "D", false, "Enable debug mode")
	flags.IntVar(&conf.MTU,
		"mtu", 0, "Set the containers network MTU")
	flags.MarkDeprecated("restart", "Please use a restart policy on docker run")
	flags.Lookup("not-a-definition")
	conf.Name = "default-address-pools"
	flags.BoolVar(&o.TLS, FlagTLS, false, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&o.Verify, opts.FlagTLSVerify, false, "Use TLS and verify the remote")
	flags.Int64Var(&conf.CPURealtimePeriod, "cpu-rt-period", 0, "Limit the CPU real-time period")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	cmd.Flags().StringVar(&conf.Pidfile, "pidfile", "", "Path to use for daemon PID file")
}

const (
	FlagTLS       = "tls"
	FlagTLSVerify = "tlsverify"
)
'''

    def test_flag_names(self):
        with tempfile.TemporaryDirectory() as repo:
            os.makedirs(os.path.join(repo, "daemon", "config"))
            Path(repo, "daemon", "config", "flags.go").write_text(self.GO)
            Path(repo, "daemon", "config", "flags_test.go").write_text('func f() { flags.Bool("from-a-test", false, "") }')
            got = spec_roles._dockerd_flags(repo, "daemon/config")
        self.assertEqual(got, {"dns-opt", "data-root", "debug", "-D", "mtu", "restart", "tls", "tlsverify",
                               "cpu-rt-period", "help", "-h", "pidfile"})


class ExternalLinks(unittest.TestCase):
    def test_html_links_to_github(self):
        from types import SimpleNamespace

        para = nodes.paragraph()
        para += nodes.reference("", "openapi", refuri="../../../../api/openapi.yaml")
        para += nodes.reference("", "README", refuri="../README.html", internal=True)
        md_needs.rewrite_external_links(SimpleNamespace(builder=SimpleNamespace(name="html")), para, "network/x")
        self.assertEqual([r["refuri"] for r in para.findall(nodes.reference)],
                         [md_needs.GITHUB + "api/openapi.yaml", "../README.html"])


class LiteralFence(unittest.TestCase):
    def test_fence(self):
        cases = {"x": ("`", ""), ":ref:`x` y": ("``", ""), "`x`": ("``", " "), "a``b": ("```", "")}
        for text, want in cases.items():
            with self.subTest(text=text):
                self.assertEqual(md_needs._literal_fence(nodes.literal(text=text)), want)


@unittest.skipUnless(sys.platform == "linux", "the names are those that Linux defines")
class ErrnoSignalNames(unittest.TestCase):
    def test_match_python(self):
        import errno
        import re
        import signal

        self.assertEqual(spec_roles.ERRNOS, {n for n in dir(errno) if re.fullmatch(r"E[A-Z0-9]+", n)})
        self.assertEqual(spec_roles.SIGNALS, set(signal.Signals.__members__))


class GoldenNames(unittest.TestCase):
    def test_tables_and_chains(self):
        tmp = _go_repo({"integration/network/bridge/doc/generated/x.md": """
table inet docker-bridges {
    chain filter-FORWARD {
Chain DOCKER-USER (1 references)
-N DOCKER-CT
"""})
        with tmp:
            self.assertEqual(spec_roles._golden_names(tmp.name, "doc"),
                             ({"docker-bridges"}, {"filter-FORWARD", "DOCKER-USER", "DOCKER-CT"}))


class TrackedPaths(unittest.TestCase):
    def test_tracked(self):
        with tempfile.TemporaryDirectory() as repo:
            self.assertIsNone(spec_roles._tracked_paths(repo))
            os.makedirs(os.path.join(repo, "api"))
            Path(repo, "api", "openapi.yaml").touch()
            Path(repo, "api", "draft.yaml").touch()
            subprocess.run(["git", "init", "-q", repo], check=True)
            subprocess.run(["git", "-C", repo, "add", "api/openapi.yaml"], check=True)
            tracked = spec_roles._tracked_paths(repo)
            self.assertEqual(tracked, {".", "api", "api/openapi.yaml"})
            for path, want in (("api/openapi.yaml", True), ("api/", True), ("api/*.yaml", True),
                               ("api/draft.yaml", False), (".git", False)):
                with self.subTest(path=path):
                    self.assertEqual(spec_roles._repo_path_exists(repo, path, tracked), want)


class CheckTracked(unittest.TestCase):
    def test_check_refuses_an_untracked_path(self):
        """check() itself holds a :repo-path: to the files that git tracks."""
        from types import SimpleNamespace
        from unittest import mock

        with tempfile.TemporaryDirectory() as repo:
            os.makedirs(os.path.join(repo, "api"))
            Path(repo, "api", "openapi.yaml").touch()
            Path(repo, "api", "draft.yaml").touch()
            subprocess.run(["git", "init", "-q", repo], check=True)
            subprocess.run(["git", "-C", repo, "add", "api/openapi.yaml"], check=True)
            app = SimpleNamespace(config=SimpleNamespace(spec_repo_root=repo, spec_legacy_chains=[]))
            env = SimpleNamespace(spec_role_uses=[("repo-path", "api/openapi.yaml", "doc", 1),
                                                  ("repo-path", "api/draft.yaml", "doc", 2)])
            with mock.patch.object(spec_roles, "LOGGER") as logger:
                spec_roles.check(app, env)
        warned = [c.args[0] for c in logger.warning.call_args_list]
        self.assertEqual(len(warned), 1, warned)
        self.assertIn(":repo-path:`api/draft.yaml`", warned[0])


class Categories(unittest.TestCase):
    def test_first_link_that_fits(self):
        cases = [
            (dict(violates=["P"], refines=["R"]), "defect"),
            (dict(refines=["R"], provisional_on=["Q"]), "contract"),
            (dict(provisional_on=["Q"]), "provisional"),
            ({}, "detector"),
        ]
        for links, want in cases:
            with self.subTest(links=links):
                self.assertEqual(spec_roles._category(dict(type="expectation", **links)), want)
        self.assertIsNone(spec_roles._category(dict(type="req", refines=["P"])))


if __name__ == "__main__":
    unittest.main()
