"""Tests of tools/needs_diff.py: what counts as a change, rename matching, and the escaping of the
report."""

import importlib.util
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "tools" / "needs_diff.py"
_spec = importlib.util.spec_from_file_location("needs_diff", SCRIPT)
nd = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(nd)


def doc(needs, schema=None):
    return {"current_version": "", "versions": {"": {"needs": needs, "needs_schema": {"properties": schema or {}}}}}


def need(need_id, title="Title", content="", **fields):
    return dict(id=need_id, type="expectation", title=title, content=content, **fields)


def section(rep, name):
    return next(items for n, items in rep["sections"] if n == name)


class Unwrap(unittest.TestCase):
    def test_reformat_is_not_a_change(self):
        cases = [
            ("foo bar\nbaz", "foo\nbar baz"),
            ("- foo bar\n  baz\n- qux", "- foo\n  bar baz\n- qux"),
            ("#. one two\n   three", "#. one\n   two three"),
            ("text with :ref:`x`\nand more", "text with\n:ref:`x` and more"),
            ("Intro::\n\n    a  b\n    c\n\n    d\n\nAfter\npara", "Intro::\n\n    a  b\n    c\n\n    d\n\nAfter para"),
            ("p\n\n\n\nq", "p\n\nq"),
            # a number at the start of a line of a paragraph is text, not an enumerated item
            ("The request responds 403. Then x.", "The request responds\n403. Then x."),
            ("- a request responds 403. Then x.", "- a request responds\n  403. Then x."),
            ("- - a nested item\n    wraps", "- - a nested\n    item wraps"),
            # a paragraph of a list item after its literal block
            ("- Run::\n\n      cmd\n\n  Then the daemon\n  does a thing.", "- Run::\n\n      cmd\n\n  Then the daemon does\n  a thing."),
        ]
        for a, b in cases:
            with self.subTest(a=a):
                self.assertEqual(nd.unwrap(a), nd.unwrap(b))

    def test_structure_is_a_change(self):
        cases = [
            ("- bridge\n- overlay", "- bridge - overlay"),
            ("- a\n\n  - b", "- a\n\n- b"),
            ("- a\n  - b", "- a\n- b"),
            # an item that starts with a list of its own
            ("- - x\n  - y", "- - x - y"),
            ("1. - x\n   - y", "1. - x - y"),
            ("- a\n\n  para", "- a\n\npara"),
            ("invalid address", "invalid  address"),
            ("p\n\nq", "p q"),
            ("Intro::\n\n    a\n    b", "Intro::\n\n    a b"),
            ("1. x\n2. y", "1. x 2. y"),
            ("=== ===\na   b\nc   d\n=== ===", "=== ===\na   b c   d\n=== ==="),
            (":a: x\n:b: y", ":a: x :b: y"),
            # a definition list, and a line indented past the text of the line before
            ("term\n   definition", "term definition"),
            ("A paragraph\n    quoted?", "A paragraph quoted?"),
            ("| a\n| b", "| a | b"),
            (".. code-block:: sh\n\n    a\n    b", ".. code-block:: sh\n\n    a b"),
        ]
        for a, b in cases:
            with self.subTest(a=a):
                self.assertNotEqual(nd.unwrap(a), nd.unwrap(b))


class Compare(unittest.TestCase):
    def test_renames_pair_the_most_similar_items(self):
        # A-V4 is similar enough to B-A6 to pair with it, but less than A-V6 is, and (A-V4, B-A6) is the
        # first pair in the order of the IDs: pairing in that order, or the least similar pair first,
        # crosses the twins.
        base = doc({
            "A-V4": need("A-V4", "IPv4 thing works", "The IPv4 thing works well on a node with a default address."),
            "A-V6": need("A-V6", "IPv6 thing works", "The IPv6 thing works well on a node with a default address."),
        })
        head = doc({
            "B-V4": need("B-V4", "IPv4 thing works", "The IPv4 thing works well on any node with a default address, too."),
            "B-A6": need("B-A6", "IPv6 thing works", "The IPv6 thing works well on a node with the default address."),
        })
        b, h = base["versions"][""]["needs"], head["versions"][""]["needs"]
        self.assertGreaterEqual(nd.similarity(b["A-V4"], h["B-A6"]), 0.8)
        self.assertGreater(nd.similarity(b["A-V6"], h["B-A6"]), nd.similarity(b["A-V4"], h["B-A6"]))
        rep = nd.compare(base, head, "a", "b", set(), 0.8)
        self.assertEqual([(e["old_id"], e["id"]) for e in section(rep, "Renamed")], [("A-V4", "B-V4"), ("A-V6", "B-A6")])
        self.assertEqual((section(rep, "Added"), section(rep, "Removed")), ([], []))

    def test_threshold_is_inclusive(self):
        words = "one two three four five six seven eight nine ten".split()
        base = doc({"A": need("A", "", " ".join(words))})
        head = doc({"B": need("B", "", " ".join(words[:8] + ["eleven", "twelve"]))})
        self.assertEqual(nd.similarity(base["versions"][""]["needs"]["A"], head["versions"][""]["needs"]["B"]), 0.8)
        self.assertEqual(len(section(nd.compare(base, head, "a", "b", set(), 0.8), "Renamed")), 1)

    def test_recheck(self):
        schema = {
            "refines": {"field_type": "links"},
            "refines_back": {"field_type": "backlinks"},
            # a derived link type repeats its members, and is neither compared nor followed
            "detects": {"field_type": "links", "readOnly": True},
            "detects_back": {"field_type": "backlinks"},
        }
        items = {i: need(i, f"Title {i}") for i in ("P", "Q", "R", "Y", "Z")}
        base = doc({**items, "X": need("X", refines=["P", "Q"], detects=["R"])}, schema)
        head = doc({**items, "X": need("X", refines=["P"], refines_back=["Y"], detects_back=["Z"])}, schema)
        [changed] = section(nd.compare(base, head, "a", "b", set(), 0.8), "Changed")
        self.assertEqual(changed["rows"], [("refines", "−<code>Q</code>")])
        self.assertEqual(changed["recheck"], [("P", "Title P", "refines", "out", False),
                                              ("Q", "Title Q", "refines", "out", True),
                                              ("Y", "Title Y", "refines", "in", False)])

    def test_order_and_location_are_not_changes(self):
        schema = {"refines": {"field_type": "links"}, "lineno": {"field_type": "core"}}
        base = doc({"P": need("P"), "Q": need("Q"), "X": need("X", refines=["P", "Q"], lineno=10)}, schema)
        head = doc({"P": need("P"), "Q": need("Q"), "X": need("X", refines=["Q", "P"], lineno=12)}, schema)
        self.assertEqual(section(nd.compare(base, head, "a", "b", set(), 0.8), "Changed"), [])

    def test_added_link_is_shown(self):
        schema = {"refines": {"field_type": "links"}}
        base = doc({"P": need("P"), "X": need("X")}, schema)
        head = doc({"P": need("P"), "X": need("X", refines=["P"])}, schema)
        [changed] = section(nd.compare(base, head, "a", "b", set(), 0.8), "Changed")
        self.assertEqual(changed["rows"], [("refines", "+<code>P</code>")])

    def test_removed_items_that_link_each_other(self):
        schema = {"refines": {"field_type": "links"}, "refines_back": {"field_type": "backlinks"}}
        base = doc({"P": need("P", "First thing", "One.", refines_back=["X"]),
                    "X": need("X", "Second", "Two entirely different words here.", refines=["P"])}, schema)
        rep = nd.compare(base, doc({}, schema), "a", "b", set(), 0.8)
        self.assertEqual([e["id"] for e in section(rep, "Removed")], ["P", "X"])

    def test_unchanged_text_is_a_rename(self):
        rep = nd.compare(doc({"A": need("A", "Same", "Same text.")}), doc({"B": need("B", "Same", "Same text.")}),
                         "a", "b", set(), 0.8)
        self.assertEqual([(e["old_id"], e["id"]) for e in section(rep, "Renamed")], [("A", "B")])

    def test_dissimilar_items_are_not_renames(self):
        base = doc({"A": need("A", "One thing", "The daemon does one thing.")})
        head = doc({"B": need("B", "Another", "A container sees something else entirely.")})
        rep = nd.compare(base, head, "a", "b", set(), 0.8)
        self.assertEqual((section(rep, "Renamed"), [e["id"] for e in section(rep, "Added")],
                          [e["id"] for e in section(rep, "Removed")]), ([], ["B"], ["A"]))

    def test_items_of_different_types_are_not_renames(self):
        base = doc({"A": need("A", "Same", "Same text.")})
        head = doc({"B": dict(need("B", "Same", "Same text."), type="req")})
        rep = nd.compare(base, head, "a", "b", set(), 0.8)
        self.assertEqual(section(rep, "Renamed"), [])

    def test_reflow_is_not_a_change(self):
        schema = {"content": {"field_type": "core"}}
        base = doc({"X": need("X", content="The daemon does\na thing.")}, schema)
        head = doc({"X": need("X", content="The daemon\ndoes a thing.")}, schema)
        self.assertEqual(section(nd.compare(base, head, "a", "b", set(), 0.8), "Changed"), [])
        head = doc({"X": need("X", content="The daemon does\n\na thing.")}, schema)
        self.assertEqual(len(section(nd.compare(base, head, "a", "b", set(), 0.8), "Changed")), 1)

    def test_section_and_derived_fields_are_not_changes(self):
        schema = {f: {"field_type": "core" if f == "sections" else "extra"}
                  for f in ("sections", "vantage", "observed_from")}
        base = doc({"X": need("X", sections=["Old heading"], vantage=["api"], observed_from="api")}, schema)
        head = doc({"X": need("X", sections=["New heading"], vantage=["node"], observed_from="node")}, schema)
        rep = nd.compare(base, head, "a", "b", set(), 0.8)
        [changed] = section(rep, "Changed")
        self.assertEqual([f for f, _ in changed["rows"]], ["observed_from"])

    def test_a_revision_without_specs_has_no_items(self):
        rep = nd.compare(nd.EMPTY, doc({"X": need("X")}), "a", "b", set(), 0.8)
        self.assertEqual([e["id"] for e in section(rep, "Added")], ["X"])


class Escaping(unittest.TestCase):
    def test_word_diff_inside_a_literal(self):
        out = nd.word_diff("Run ``docker network create --ipv6 net``", "Run ``docker network create --ipv4=false net``")
        self.assertNotIn("`", out)
        self.assertIn("<del>--ipv6</del><ins>--ipv4=false</ins>", out)

    def test_whitespace_change_is_shown(self):
        self.assertIn("<del>·</del><ins>··</ins>", nd.word_diff("invalid address", "invalid  address"))

    def test_code_stays_on_one_line(self):
        self.assertNotIn("\n", nd.render_change(None, "First.\n\nSecond."))
        long = "A sentence that is longer than forty characters."
        self.assertNotIn("\n", nd.render_change(long, long + "\n\n" + long))

    def test_summary_too_long(self):
        rep = nd.compare(nd.EMPTY, doc({f"X{i}": need(f"X{i}") for i in range(50)}), "a", "b", set(), 0.8)
        limit, nd.SUMMARY_LIMIT = nd.SUMMARY_LIMIT, 500
        try:
            out = nd.summary(rep, "the full report")
        finally:
            nd.SUMMARY_LIMIT = limit
        self.assertIn("too long for the job summary; see the full report", out)
        self.assertNotIn("X49", out)

    def test_reports_escape_titles(self):
        title = "a`b*c<d"
        rep = nd.compare(nd.EMPTY, doc({"X": need("X", title)}), "a", "b", set(), 0.8)
        for name, out in (("markdown", nd.markdown(rep)), ("summary", nd.summary(rep, "r")),
                          ("html", nd.html_page(rep))):
            with self.subTest(report=name):
                self.assertNotIn(title, out)
                self.assertIn("a&#96;b&#42;c&lt;d", out)

    def test_markdown_markup_is_escaped(self):
        for c in "`*_[]\\|~":
            with self.subTest(c=c):
                self.assertNotIn(c, nd.esc(f"a{c}b"))


#: A stand-in for sphinx-build: it fails for a project that holds a file named "broken", and gives a
#: project without items otherwise.
FAKE_SPHINX = """import json, os, sys
src, out = sys.argv[-2:]
if os.path.exists(os.path.join(src, "broken")):
    sys.exit("broken")
os.makedirs(out, exist_ok=True)
json.dump({"current_version": "", "versions": {"": {"needs": {}}}}, open(os.path.join(out, "needs.json"), "w"))
"""


class Main(unittest.TestCase):
    """needs_diff.py on a repository whose first commit has no project, whose second commit has a
    project that does not build, and whose third commit fixes it."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.repo = os.path.join(cls.tmp.name, "repo")
        git = ["git", "-C", cls.repo, "-c", "user.name=t", "-c", "user.email=t@example.com"]
        os.makedirs(cls.repo)
        subprocess.run(["git", "init", "-q", cls.repo], check=True)
        Path(cls.repo, "README").write_text("")
        subprocess.run(git + ["add", "."], check=True)
        subprocess.run(git + ["commit", "-q", "-m", "no project"], check=True)
        os.makedirs(os.path.join(cls.repo, "proj"))
        Path(cls.repo, "proj", "conf.py").write_text("")
        Path(cls.repo, "proj", "broken").write_text("")
        subprocess.run(git + ["add", "."], check=True)
        subprocess.run(git + ["commit", "-q", "-m", "broken"], check=True)
        subprocess.run(git + ["rm", "-q", "proj/broken"], check=True)
        subprocess.run(git + ["commit", "-q", "-m", "fixed"], check=True)
        cls.sphinx = os.path.join(cls.tmp.name, "sphinx-build")
        Path(cls.sphinx).write_text("#!" + sys.executable + "\n" + FAKE_SPHINX)
        os.chmod(cls.sphinx, 0o755)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def run_diff(self, base, head):
        html = os.path.join(self.tmp.name, "report.html")
        if os.path.exists(html):
            os.remove(html)
        tmpdir = os.path.join(self.tmp.name, "tmpdir")
        os.makedirs(tmpdir, exist_ok=True)
        r = subprocess.run([sys.executable, str(SCRIPT), "--repo", self.repo, "--project", "proj", "--sphinx-build",
                            self.sphinx, "--html", html, "--", base, head], capture_output=True, text=True,
                           env=dict(os.environ, TMPDIR=tmpdir))
        # whatever the outcome, the run leaves no worktree and no temporary directory behind
        worktrees = subprocess.run(["git", "-C", self.repo, "worktree", "list", "--porcelain"], capture_output=True,
                                   text=True, check=True).stdout
        self.assertEqual(worktrees.count("worktree "), 1, worktrees)
        self.assertEqual(os.listdir(tmpdir), [])
        return r, html

    def test_a_base_without_the_project(self):
        r, html = self.run_diff("HEAD~2", "HEAD")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("HEAD&#126;2 (no specs)", r.stdout)
        self.assertIn("0 changed, 0 added, 0 removed", r.stdout)
        self.assertTrue(os.path.exists(html))

    def test_neither_with_the_project(self):
        r, html = self.run_diff("HEAD~2", "HEAD~2")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("No spec items to compare", r.stdout)
        self.assertIn("No spec items to compare", Path(html).read_text())

    def test_a_base_that_does_not_build(self):
        """A base that the toolchain cannot build gives a report that says so, not a failure."""
        r, html = self.run_diff("HEAD~1", "HEAD")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("could not be built", r.stdout)
        self.assertIn("could not be built", Path(html).read_text())

    def test_a_head_that_does_not_build(self):
        for base in ("HEAD", "HEAD~1"):
            with self.subTest(base=base):
                r, _ = self.run_diff(base, "HEAD~1")
                self.assertNotEqual(r.returncode, 0)
                self.assertIn("sphinx-build failed at HEAD~1", r.stderr)

    def test_a_revision_that_does_not_exist(self):
        r, _ = self.run_diff("no-such-revision", "HEAD")
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("cannot check out no-such-revision", r.stderr)
        self.assertNotIn("Traceback", r.stderr)


if __name__ == "__main__":
    unittest.main()
