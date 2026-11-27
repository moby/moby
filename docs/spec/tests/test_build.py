"""Build the fixture project with the specs' configuration and extensions, and check what the build
reports: each rule reports the item of tests/fixture/network/broken.rst that breaks it, and nothing
about the valid items. The Markdown builder renders what GitHub would otherwise misread.

make validate shows that the checks pass the real specs; these tests show that they still fail
a broken item, which no build of the real specs can show.
"""

import json
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SPEC = Path(__file__).resolve().parents[1]
REPO = SPEC.parents[1]
FIXTURE = Path(__file__).resolve().parent / "fixture"

#: The documents of the fixture whose items break rules.
BROKEN_DOCS = {"network/broken", "network/templates/nested", "storage/novocab"}

#: Set by setUpModule: the warnings of the needs build, and its schema findings by item ID.
WARNINGS = []
FINDINGS = {}
NEEDS = {}
MARKDOWN = ""
#: The other generated Markdown files, by docname.
GENERATED = {}


def _build(tmp, builder, retired_ids=None):
    src = tmp / "src"
    if not src.exists():
        shutil.copytree(SPEC / "src" / "ext", src / "ext", ignore=shutil.ignore_patterns("__pycache__"))
        shutil.copytree(SPEC / "src" / "templates", src / "templates")
        shutil.copy(SPEC / "src" / "conf.py", src)
        shutil.copytree(FIXTURE, src, dirs_exist_ok=True)
    if retired_ids is not None:
        (src / "retired-ids.txt").write_text(retired_ids)
    out = tmp / builder
    r = subprocess.run(
        [sys.executable, "-m", "sphinx", "-E", "-q", "--keep-going", "-b", builder,
         # the fixture's own root document, whatever the configuration names
         "-D", "root_doc=README", "-D", f"spec_repo_root={REPO}", str(src), str(out)],
        capture_output=True, text=True,
    )
    if r.returncode != 0:
        raise RuntimeError(f"sphinx-build -b {builder} failed:\n{r.stderr[-3000:]}")
    return out, r.stderr


def setUpModule():
    global MARKDOWN
    with tempfile.TemporaryDirectory() as d:
        tmp = Path(d)
        out, stderr = _build(tmp, "needs")
        # A warning starts a line; the retired-id warning continues on indented lines.
        WARNINGS.extend(re.split(r"\n(?=\S)", stderr.strip()))
        violations = json.loads((out / "schema_violations.json").read_text())
        for need_id, found in violations.get("validation_warnings", {}).items():
            FINDINGS[need_id] = {f["details"]["schema_path"].split("[", 1)[0] for f in found}
        doc = json.loads((out / "needs.json").read_text())
        NEEDS.update(doc["versions"][doc["current_version"]]["needs"])
        out, _ = _build(tmp, "markdown")
        MARKDOWN = (out / "network" / "markdown.md").read_text()
        for docname in ("network/README", "network/good"):
            GENERATED[docname] = (out / f"{docname}.md").read_text()


def warnings_about(text):
    return [w for w in WARNINGS if text in w]


class ValidItems(unittest.TestCase):
    def test_no_warnings(self):
        for doc in ("good.rst", "markdown.rst", "README.rst:"):
            with self.subTest(doc=doc):
                self.assertEqual(warnings_about(doc), [])

    def test_no_warnings_about_the_fixture_itself(self):
        # only the checks warn about the broken documents
        for w in WARNINGS:
            with self.subTest(w=w[:80]):
                self.assertFalse("WARNING" in w and "[spec." not in w and "[needs." not in w, w)

    def test_no_schema_findings(self):
        valid = [i for i, n in NEEDS.items() if n["docname"] not in BROKEN_DOCS]
        self.assertGreater(len(valid), 10)
        for need_id in valid:
            with self.subTest(need_id=need_id):
                self.assertNotIn(need_id, FINDINGS)

    def test_no_errors_without_a_location(self):
        # a schema violation is reported without a file, by its item's ID
        for w in WARNINGS:
            with self.subTest(w=w[:80]):
                self.assertFalse("ERROR" in w and not any(i in w for i in FINDINGS), w)

    def test_category_is_computed(self):
        cases = {"NET-TEST-DEFECT": "defect", "NET-TEST-CONTRACT": "contract", "NET-TEST-PROVISIONAL": "provisional",
                 "NET-TEST-DETECTOR": "detector", "REQ-NET-TEST-PROMISE": None, "PRIN-TEST-ROOT": None}
        for need_id, category in cases.items():
            with self.subTest(need_id=need_id):
                self.assertEqual(NEEDS[need_id].get("category"), category)

    def test_vantage_fields(self):
        need = NEEDS["NET-TEST-CONTRACT"]
        self.assertEqual(need["sent_from"], ["underlay"])
        self.assertEqual(need["observed_at"], ["task", "ctr"])
        self.assertEqual(need["vantage"], ["underlay", "task", "ctr"])

    def test_needextend_selects_by_vantage(self):
        self.assertIn("NET-HOST-TEST-KERNEL", NEEDS["NET-TEST-CONTRACT"]["needs"])
        self.assertNotIn("NET-HOST-TEST-KERNEL", NEEDS["NET-TEST-DETECTOR"].get("needs") or [])


class SchemaRules(unittest.TestCase):
    #: Each broken item, and the one rule that it breaks.
    CASES = {
        "NET-TEST-NOT-A-REQ": "req-id-prefix",
        "NET-TEST-NOT-A-RATIONALE": "rationale-id-prefix",
        "NET-TEST-NOT-A-PRINCIPLE": "principle-id-prefix",
        "REQ-NET-TEST-NOT-AN-EXPECTATION": "spec-code-id-prefix",
        "NET-TEST-NONE-AND-OS": "depends-on-none-alone",
        "NET-TEST-CLI-WITHOUT-CLI": "cli-form-of-api-expectation",
        "NET-TEST-CLI-OF-CLI": "cli-form-of-api-expectation",
        "NET-TEST-OS-LINUX-ON-WINDOWS": "os-linux-on-linux",
        "NET-TEST-OS-WINDOWS-ON-LINUX": "os-windows-on-windows",
        "REQ-NET-TEST-NO-SOURCE": "requirement-cites-a-source",
        "NET-TEST-REFINES-UNSTABLE": "refines-a-stable-promise",
        "NET-TEST-PROVISIONAL-ON-STABLE": "provisional-on-an-unstable-promise",
        "REQ-NET-TEST-UNREFINED": "requirement-is-refined",
        "REQ-NET-TEST-UNSTABLE-UNUSED": "provisional-requirement-is-used",
        # broken.rst is not in LAX_DOCS, so the rules for linked specs fail it
        "NET-HOST-TEST-UNNEEDED": "host-requirement-is-needed-violation",
        "NET-TEST-SAYS-NOTHING": "detector-says-what-it-detects-violation",
        "NET-TEST-NARROWS-NOT-DEFECT": "narrows-is-a-defect-link-violation",
        "NET-TEST-CONCERNS-NOT-DEFECT": "concerns-is-a-defect-link-violation",
        "NET-TEST-DEFECT-AFFECTS-NOTHING": "defect-affects-an-expectation-violation",
        # nor is a document in a directory named like an excluded one, below the top level
        "NET-TEST-NESTED": "detector-says-what-it-detects-violation",
        "NET-CEIL-TEST-UNUSED": "links > schema > properties > within_back > minItems",
        "RAT-NET-TEST-UNUSED": "links > schema > properties > rationale_back > minItems",
        "NET-TEST-OBSERVED-NOTHING": "fields > schema > properties > observed_from > minLength",
    }

    def test_each_rule_reports_its_item(self):
        for need_id, rule in self.CASES.items():
            with self.subTest(need_id=need_id):
                self.assertEqual(FINDINGS.get(need_id), {rule})

    def test_no_other_findings(self):
        self.assertEqual(set(FINDINGS) - set(self.CASES), set())


class Checks(unittest.TestCase):
    def assertWarns(self, *texts):
        for text in texts:
            with self.subTest(text=text):
                self.assertTrue(warnings_about(text), f"no warning contains {text!r}")

    def test_written_category(self):
        self.assertWarns("NET-TEST-WRITTEN-CATEGORY: category is written",
                         "REQ-NET-TEST-CATEGORY: category is written",
                         # even as its links give it
                         "NET-TEST-WRITTEN-MATCHING: category is written",
                         # a needextend adds a violation after the category was computed
                         "NET-TEST-EXTENDED-CATEGORY: a needextend makes the category 'contract' where its links make "
                         "it 'defect'",
                         # a needextend writes a category on a requirement
                         "REQ-NET-TEST-UNREFINED: a needextend makes the category 'contract' where its links make it None")
        self.assertEqual(NEEDS["NET-TEST-WRITTEN-CATEGORY"]["category"], "defect")
        self.assertIsNone(NEEDS["REQ-NET-TEST-CATEGORY"].get("category"))

    def test_observed_from(self):
        self.assertWarns(
            "NET-TEST-OBSERVED-TWO: observed_from: 'task ctr' names two vantage points",
            "NET-TEST-OBSERVED-UNKNOWN: observed_from: 'api:manager' does not start with a vantage point",
            "NET-TEST-OBSERVED-EMPTY: observed_from: 'api,' has an empty vantage point",
            "NET-TEST-OBSERVED-ARROWS: observed_from: 'task → ctr → node' has more than one '→'",
            "NET-TEST-OBSERVED-WORDS: observed_from: 'underlay to task': words after a vantage point",
            "NET-TEST-VANTAGE-WRITTEN: vantage is computed from observed_from",
            "NET-TEST-EXTENDED-VANTAGE: vantage is computed from observed_from",
            "NET-TEST-EXTENDED-OBSERVED: a needextend changes observed_from",
            "REQ-NET-TEST-NO-SOURCE: vantage is computed from observed_from",
            "NET-TEST-NO-VOCABULARY: no vantage points are listed for the subsystem 'storage'",
        )

    def test_roles(self):
        self.assertWarns(
            ":dockerd-flag:`--default-address-pools`: dockerd defines no such flag",
            ":drvopt:`com.docker.network.bogus.driver`:",
            ":ipt-chain:`bridge`:",
            ":nft-chain:`bridge`:",
            ":repo-path:`/etc/passwd`:",
            ":repo-path:`../..`:",
            ":repo-path:`API/openapi.yaml`:",
            ":api-op:`POST /networks/create trailing`:",
            ":api-op:`GET /networks/abc/bogus`:",
            ":dockerd-flag:`-X`:",
            ":kconfig:`NOT_A_KCONFIG`:",
            ":errno:`ENOTANERRNO`:",
            ":signal:`SIGNOPE`:",
            ":nft-table:`filter-FORWARD`:",
            ":drvopt:`com.docker.netwrok.brige.*`:",
        )

    def test_roles_that_hold_pass(self):
        for text in (":dockerd-flag:`--mtu`", ":drvopt:`com.docker.network.driver.mtu`", ":ipt-chain:`DOCKER-USER`",
                     ":repo-path:`api/openapi.yaml`", ":repo-path:`integration/network/bridge/`",
                     ":api-op:`POST /networks/create`", ":api-op:`GET /networks/{id}`", ":dockerd-flag:`--debug`",
                     ":dockerd-flag:`-D`", ":kconfig:`CONFIG_BRIDGE`", ":errno:`EPERM`", ":errno:`ENOTCONN`", ":signal:`SIGTERM`",
                     ":signal:`SIGSEGV`",
                     ":nft-table:`docker-bridges`", ":nft-chain:`filter-FORWARD`",
                     ":drvopt:`com.docker.network.bridge.*`"):
            with self.subTest(text=text):
                self.assertEqual(warnings_about(text), [])

    def test_ceiling_named_but_not_linked(self):
        self.assertWarns("NET-TEST-CEILING-UNLINKED: names the ceiling NET-CEIL-TEST but does not link it")

    def test_host_requirement_of_another_platform(self):
        self.assertWarns("NET-TEST-WRONG-PLATFORM: needs NET-HOST-TEST-KERNEL, a host requirement of none of its platforms")

    def test_retired_ids_hold_only_ids(self):
        with tempfile.TemporaryDirectory() as d:
            with self.assertRaisesRegex(RuntimeError, "'NET-TEST-RETIRED \\(in 29.0\\)' is not an ID"):
                _build(Path(d), "needs", retired_ids="NET-TEST-RETIRED (in 29.0)\n")

    def test_wrapped_observed_from(self):
        self.assertEqual(warnings_about("NET-TEST-OBSERVED-WRAPPED"), [])
        self.assertEqual(NEEDS["NET-TEST-OBSERVED-WRAPPED"]["vantage"], ["underlay", "task", "ctr"])

    def test_kernel_option_prefix(self):
        self.assertWarns("NET-HOST-TEST-UNNEEDED: kernel option CONFIG_BRIDGE is written with its CONFIG_ prefix")

    def test_one_warning_each(self):
        for text in ("NET-TEST-VANTAGE-WRITTEN: vantage", "REQ-NET-TEST-NO-SOURCE: vantage", "REQ-NET-TEST-CATEGORY: vantage",
                     "NET-TEST-NO-VOCABULARY: no vantage points"):
            with self.subTest(text=text):
                self.assertEqual(len(warnings_about(text)), 1)

    def test_retired_id(self):
        retired = warnings_about("retired-id")
        self.assertEqual(len(retired), 1)
        self.assertIn("NET-TEST-RETIRED", retired[0])
        self.assertNotIn("# An indented comment", retired[0])


class Markdown(unittest.TestCase):
    def test_angle_brackets_are_escaped(self):
        self.assertIn("named eth\\<N> in a heading", MARKDOWN)
        self.assertIn("**Rationale RAT-NET-TEST-OUTER**: An interface is named eth\\<N>", MARKDOWN)

    def test_angle_brackets_in_code_are_not_escaped(self):
        self.assertIn("`eth<N>` in code", MARKDOWN)
        self.assertIn("ip link set eth<N> up", MARKDOWN)
        self.assertIn("This comment holds <N>.", MARKDOWN)

    def test_code_that_holds_a_backquote(self):
        self.assertIn("A role in code: ``:ref:`x` and y``, and", MARKDOWN)
        self.assertIn("`` `x` and y ``.", MARKDOWN)

    def test_links_to_the_repository(self):
        # the Markdown file is one directory above its source
        self.assertIn("](../../../api/openapi.yaml)", MARKDOWN)

    def test_lines_do_not_start_blocks(self):
        self.assertIn("\n\\>=1280 bytes", MARKDOWN)
        self.assertIn("`echo 1 > /proc/x`", MARKDOWN)
        self.assertIn("\n\\- y + z.", MARKDOWN)
        self.assertIn("\n403. A count is:\n1\\. Not a list", MARKDOWN)
        self.assertIn("\n\\+ that starts with a plus, or\n1\\) with a number.", MARKDOWN)
        # the first line of a paragraph
        for start in ("\\>=1280 bytes", "\\# of retries", "2\\) A second", "- 4789\\. A list item"):
            with self.subTest(start=start):
                self.assertIn("\n\n" + start, MARKDOWN)

    def test_table_cells_keep_their_text(self):
        self.assertIn("29.10", MARKDOWN)
        self.assertIn("29.0", MARKDOWN)

    def test_links_to_other_documents(self):
        self.assertIn("[Fixture](../README.md)", MARKDOWN)
        self.assertIn("](broken.md#NET-TEST-NARROWS-NOT-DEFECT)", GENERATED["network/good"])

    def test_table_cells_are_one_line(self):
        self.assertIn("Either one or the other", MARKDOWN)

    def test_contents_link_the_label(self):
        # the title's id, markdown-cells-and-anchors, starts like a label of the document
        self.assertIn("(markdown.md#markdown-cells)", GENERATED["network/README"])

    def test_needextend_anchor_has_no_slash(self):
        good = GENERATED["network/good"]
        self.assertIn("needextend-network-good", good)
        self.assertNotIn("needextend-network/", good)

    def test_html_is_kept(self):
        self.assertTrue(MARKDOWN.startswith("<!-- Generated from src/network/markdown.rst"), MARKDOWN[:80])
        self.assertIn('<a id="RAT-NET-TEST-OUTER"></a>', MARKDOWN)

    def test_table_cell_pipes_are_escaped(self):
        row = next(line for line in MARKDOWN.splitlines() if "Either" in line)
        self.assertEqual(row.count("|") - row.count("\\|"), 3, row)

    def test_nested_need_is_rendered(self):
        self.assertIn("**Rationale RAT-NET-TEST-INNER**", MARKDOWN)
        self.assertNotIn("<svg", MARKDOWN)
        self.assertNotIn("<table", MARKDOWN)


if __name__ == "__main__":
    unittest.main()
