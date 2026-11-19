#!/usr/bin/env python3
"""Diff the Sphinx-Needs items of a documentation project between two git revisions.

Builds needs.json at each revision (or reads two given files), compares the items by ID,
and reports the added, removed, renamed and changed items, a word-level diff of each changed
field, and the linked items to re-check for each change. The report is printed as Markdown;
--summary prints only the list of items, and --html also writes the full report as a
self-contained HTML page.

  needs_diff.py [--summary] [--html FILE] --repo DIR --project SUBDIR BASE [HEAD]
  needs_diff.py [--summary] [--html FILE] --base-json A.json --head-json B.json
"""
import argparse, difflib, html, json, os, re, shutil, subprocess, sys, tempfile

# Core fields that describe where or how an item is rendered, not what it says. An item that only
# moves to another section, or whose section is renamed, does not change.
IGNORED_CORE = {
    "lineno", "docname", "doctype", "external_css", "external_url", "type_name", "layout",
    "style", "template", "pre_template", "post_template", "pre_content", "post_content",
    "jinja_content", "is_external", "is_import", "is_modified", "modifications",
    "has_dead_links", "has_forbidden_dead_links", "constraints_passed", "constraints_results",
    "constraints_error", "section_name", "sections", "parent_need", "signature", "parts", "project",
}
# Fields that the build computes from another field, whose change is reported there.
DERIVED = {"vantage", "observed_at", "sent_from"}
DEFAULT_EXCLUDED_TYPES = {"testfile", "testsuite", "testcase"}

# The needs.json of a revision without the project.
EMPTY = {"current_version": "", "versions": {"": {"needs": {}}}}

# GitHub drops a job summary larger than 1 MiB; --summary stays below this.
SUMMARY_LIMIT = 1000 * 1000


class BuildFailed(Exception):
    pass


def build(repo, rev, project, sphinx_build, log):
    """Build needs.json for rev in a temporary detached worktree; return its parsed content, or None
    for a revision without the project."""
    tmp = tempfile.mkdtemp(prefix="needs-diff-")
    wt = os.path.join(tmp, "wt")
    try:
        r = subprocess.run(["git", "-C", repo, "worktree", "add", "--quiet", "--detach", wt, rev],
                           capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit(f"cannot check out {rev}: {r.stderr.strip()}")
        src = os.path.join(wt, project)
        if not os.path.exists(os.path.join(src, "conf.py")):
            log(f"{rev}: no Sphinx project at {project}")
            return None
        out = os.path.join(tmp, "out")
        r = subprocess.run([sphinx_build, "-b", "needs", "-q", "-T", src, out],
                           capture_output=True, text=True)
        warnings = [l for l in r.stderr.splitlines() if "WARNING" in l or "ERROR" in l]
        if warnings:
            log(f"{rev}: {len(warnings)} build warning(s); first: {warnings[0]}")
        if r.returncode != 0:
            raise BuildFailed(f"sphinx-build failed at {rev}:\n{r.stderr[-2000:]}")
        with open(os.path.join(out, "needs.json")) as f:
            return json.load(f)
    finally:
        if os.path.exists(wt):
            subprocess.run(["git", "-C", repo, "worktree", "remove", "--force", wt], check=False)
        shutil.rmtree(tmp, ignore_errors=True)


def current(doc):
    v = doc["versions"][doc["current_version"]]
    return v["needs"], v.get("needs_schema", {}).get("properties", {})


def field_kinds(schema):
    """Split fields into compared fields, outgoing link types and back-link fields."""
    compared, links, back = [], [], []
    for name, p in schema.items():
        kind = p.get("field_type")
        if kind == "backlinks":
            back.append(name)
        elif kind == "links":
            if not p.get("readOnly"):  # a derived link type repeats its members
                links.append(name)
                compared.append(name)
        elif (kind == "core" and name in IGNORED_CORE) or name in DERIVED:
            continue
        else:
            compared.append(name)
    back = [b for b in back if b[:-len("_back")] in links]
    return compared, links, back


def norm(v):
    if v in (None, "", [], {}):
        return None
    if isinstance(v, list):
        return sorted(v) if all(isinstance(x, str) for x in v) else v
    if isinstance(v, str):
        return unwrap(v) or None
    return v


# The start of a line that begins a block of its own: a list item, a field, a directive or comment,
# a line block or table row, or a table border. Such a line is never a soft-wrapped continuation.
BLOCK_START = re.compile(r"(?:[-*+\u2022\u2023\u2043]|\(?(?:\d+|[A-Za-z]|[ivxlcdmIVXLCDM]+|#)[.)])(?:\s|$)"
                         r"|:[^:`\s][^`]*?:(?:\s|$)|\.\.(?:\s|$)|\|(?:\s|$)|\+[-=+]|=+(?:\s|$)")
# A list item marker, after which the item's text, and its continuation lines, start.
LIST_MARKER = re.compile(r"(?:[-*+\u2022\u2023\u2043]|\(?(?:\d+|[A-Za-z]|[ivxlcdmIVXLCDM]+|#)[.)])\s+")
# A directive whose content is kept as it is.
VERBATIM = re.compile(r"\.\. (?:code|code-block|sourcecode|parsed-literal|literalinclude|csv-table|raw|math)::")
TABLE_BORDER = re.compile(r"=+(?: +=+)*$|\+[-=+]*$")


def unwrap(text):
    """Join the soft-wrapped lines of each prose paragraph, so that a reformat is not a change.

    A line continues the one before it only if it is indented to where that line's text starts and
    does not begin a block of its own, such as a list item. Paragraph breaks, indentation, list
    item boundaries, the spacing within a line, tables and literal blocks are kept: a change to any
    of them is a change.
    """
    out, verbatim = [], None  # verbatim: the indent beyond which a paragraph is literal
    for para in re.split(r"\n(?:[ \t]*\n)+", text.strip("\n")):
        lines = [l.expandtabs(8).rstrip() for l in para.split("\n")]
        indent = min((len(l) - len(l.lstrip()) for l in lines if l), default=0)
        if verbatim is not None and indent > verbatim:
            out.append("\n".join(lines))
            continue
        verbatim = None
        if any(TABLE_BORDER.match(l.lstrip()) for l in lines):
            out.append("\n".join(lines))
            continue
        logical = []  # [indent, continuation indent, text]
        for l in lines:
            ind, s = len(l) - len(l.lstrip()), l.lstrip()
            # docutils starts a field, line or item only after another at the same indent: "403." at
            # the start of a line of a paragraph is text
            starts = BLOCK_START.match(s) and logical and logical[-1][0] == ind and BLOCK_START.match(logical[-1][2])
            if logical and ind == logical[-1][1] and not starts:
                logical[-1][2] += " " + s
                continue
            # an item's text starts after its marker, and after those of the lists nested at its start
            pos = 0
            while m := LIST_MARKER.match(s, pos):
                pos = m.end()
            logical.append([ind, ind + pos, s])
        out.append("\n".join(" " * i + t for i, _, t in logical))
        for i, _, t in logical:
            if VERBATIM.match(t):
                verbatim = i
        if logical and logical[-1][2].endswith("::"):
            verbatim = logical[-1][1]
    return "\n\n".join(out).strip()


# The characters that GitHub Markdown would read as markup inside the HTML of the report, such as
# a pair of backquotes that would make a code span of a word diff. Their references show the same
# characters in Markdown and in HTML.
MARKDOWN_CHARS = {c: f"&#{ord(c)};" for c in "`*_[]\\|~"}


def esc(s):
    return html.escape(str(s), quote=False).translate(str.maketrans(MARKDOWN_CHARS))


def code(s):
    """Inline code, on one line: a line break would end a Markdown table row."""
    return f"<code>{esc(s)}</code>".replace("\n", "<br>")


WORD = re.compile(r"\S+|\s+")


def word_diff(a, b):
    """Inline word diff as HTML with <del>/<ins>, which GitHub Markdown also displays."""
    aw, bw = WORD.findall(a), WORD.findall(b)
    out = []
    for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, aw, bw, autojunk=False).get_opcodes():
        old, new = "".join(aw[i1:i2]), "".join(bw[j1:j2])
        if tag == "equal":
            out.append(esc(old))
        elif not old.strip() and not new.strip():
            # A change in whitespace only: show it, with a middle dot for a space and ¶ for a break.
            shown = lambda w: w.replace(" ", "·").replace("\t", "→").replace("\n", "¶")
            out.append(f"<del>{shown(old)}</del><ins>{shown(new)}</ins>")
        else:
            if old.strip():
                out.append(f"<del>{esc(old)}</del>")
            if new.strip():
                out.append(f"<ins>{esc(new)}</ins>")
            elif new:
                out.append(new)
    return "".join(out).replace("\n", "<br>")


def render_change(old, new):
    """The change of one field, as an HTML fragment."""
    if isinstance(old, list) or isinstance(new, list):
        o, n = set(old or []), set(new or [])
        return " ".join([f"+{code(x)}" for x in sorted(n - o)] + [f"−{code(x)}" for x in sorted(o - n)])
    if isinstance(old, str) and isinstance(new, str) and (len(old) > 40 or len(new) > 40):
        return word_diff(old, new)
    show = lambda v: "<em>(none)</em>" if v is None else code(v)
    return f"{show(old)} → {show(new)}"


def _words(need):
    return ((need.get("title") or "") + " " + (need.get("content") or "")).split()


def _text(need):
    return (need.get("type"), " ".join(_words(need)))


def similarity(a, b):
    return difflib.SequenceMatcher(None, _words(a), _words(b), autojunk=False).ratio()


def related(need, links, back):
    """Linked items of need, as (other id, link, direction) triples."""
    rel = []
    for l in links:
        for other in need.get(l) or []:
            rel.append((other, l, "out"))
    for b in back:
        for other in need.get(b) or []:
            rel.append((other, b[:-len("_back")], "in"))
    return rel


def compare(base, head, base_label, head_label, excluded_types, rename_threshold):
    """The differences between two needs.json documents, as plain data for the renderers."""
    bn, bschema = current(base)
    hn, hschema = current(head)
    schema = {**bschema, **hschema}
    compared, links, back = field_kinds(schema)
    keep = lambda n: n.get("type") not in excluded_types
    bn = {k: v for k, v in bn.items() if keep(v)}
    hn = {k: v for k, v in hn.items() if keep(v)}

    added = sorted(set(hn) - set(bn))
    removed = sorted(set(bn) - set(hn))
    # The most similar pairs first, so that each item is paired with its best match, whatever the
    # order of the IDs: twins such as -V4 and -V6 items are not crossed.
    renamed, paired = [], set()
    # An item whose type, title and content are unchanged is paired first, by its text.
    by_text = {}
    for a in added:
        by_text.setdefault(_text(hn[a]), []).append(a)
    for r in removed:
        same = [a for a in by_text.get(_text(bn[r]), []) if a not in paired]
        if same:
            renamed.append((r, same[0], 1.0))
            paired |= {r, same[0]}
    pairs = []
    for a in added:
        if a in paired:
            continue
        m = difflib.SequenceMatcher(None, autojunk=False)
        m.set_seq2(_words(hn[a]))
        for r in removed:
            if r in paired or hn[a].get("type") != bn[r].get("type"):
                continue
            m.set_seq1(_words(bn[r]))
            # the quick ratios bound the ratio from above
            if m.real_quick_ratio() >= rename_threshold and m.quick_ratio() >= rename_threshold:
                score = m.ratio()
                if score >= rename_threshold:
                    pairs.append((score, r, a))
    for score, r, a in sorted(pairs, key=lambda p: (-p[0], p[1], p[2])):
        if r not in paired and a not in paired:
            renamed.append((r, a, score))
            paired |= {r, a}
    renamed.sort()
    removed = [r for r in removed if r not in paired]
    added = [a for a in added if a not in paired]

    def diffs(old, new):
        rows = []
        for f in compared:
            if f == "id":
                continue
            o, n = norm(old.get(f)), norm(new.get(f))
            if o != n:
                rows.append((f, render_change(o, n)))
        return rows

    def recheck(need, old=None):
        now = set(related(need, links, back))
        before = set(related(old, links, back)) if old else set()
        rel = [(o, l, d, False) for o, l, d in now] + [(o, l, d, True) for o, l, d in before - now]
        return [(o, hn[o].get("title", ""), l, d, gone) for o, l, d, gone in sorted(set(rel)) if o in hn]

    def entry(nid, n, **kw):
        return dict(id=nid, title=n.get("title", ""), type=n.get("type"), rows=[], fields=[],
                    content=None, note=None, old_id=None, recheck=[], **kw)

    sections = []
    changed = []
    for i in sorted(set(bn) & set(hn)):
        d = diffs(bn[i], hn[i])
        if d:
            changed.append(dict(entry(i, hn[i]), rows=d, recheck=recheck(hn[i], bn[i])))
    sections.append(("Changed", changed))
    sections.append(("Renamed", [dict(entry(a, hn[a]), old_id=r, note=f"Matched by {s:.0%} similar title and content.",
                                      rows=diffs(bn[r], hn[a]), recheck=recheck(hn[a], bn[r]))
                                 for r, a, s in renamed]))
    sections.append(("Added", [dict(entry(a, hn[a]), recheck=recheck(hn[a]),
                                    fields=[(f, ", ".join(v) if isinstance(v, list) else str(v))
                                            for f in compared
                                            if f not in ("id", "title", "type", "content")
                                            for v in [norm(hn[a].get(f))] if v is not None],
                                    content=(hn[a].get("content") or "").strip() or None)
                               for a in added]))
    sections.append(("Removed", [dict(entry(r, bn[r]), recheck=recheck({}, bn[r])) for r in removed]))
    return dict(base=base_label, head=head_label, sections=sections)


def counts(rep):
    n = {name: len(items) for name, items in rep["sections"]}
    return f"{n['Changed']} changed, {n['Added']} added, {n['Removed']} removed, {n['Renamed']} renamed."


def md_heading(e):
    old = f"{code(e['old_id'])} → " if e["old_id"] else ""
    return f"{old}{code(e['id'])} {esc(e['title'])} <em>({esc(e['type'])})</em>"


def why(l, direction, gone):
    s = f"this → {code(l)} → it" if direction == "out" else f"it → {code(l)} → this"
    return s + (" (until this change)" if gone else "")


def markdown(rep):
    """The full report as GitHub Markdown."""
    L = [f"# Spec changes, {code(rep['base'])} → {code(rep['head'])}", "", counts(rep), ""]
    cell = lambda s: s.replace("|", "\\|")
    for name, items in rep["sections"]:
        if not items:
            continue
        L += [f"## {name}", ""]
        for e in items:
            L += [f"### {md_heading(e)}", ""]
            if e["note"]:
                L += [e["note"], ""]
            if e["rows"]:
                L += ["| Field | Change |", "| --- | --- |"]
                L += [f"| {code(f)} | {cell(frag)} |" for f, frag in e["rows"]]
                L.append("")
            L += [f"- {code(f)}: {esc(v)}" for f, v in e["fields"]]
            if e["fields"]:
                L.append("")
            if e["content"]:
                L += ["````rst", e["content"], "````", ""]
            if e["recheck"]:
                L += ["**Re-check:**", ""]
                L += [f"- {code(o)} {esc(t)} — {why(l, d, g)}" for o, t, l, d, g in e["recheck"]]
                L.append("")
    return "\n".join(L)


def summary(rep, report_name):
    """The list of the items that changed, as GitHub Markdown, for a job summary."""
    head = [f"# Spec changes, {code(rep['base'])} → {code(rep['head'])}", "", counts(rep), ""]
    L = list(head)
    for name, items in rep["sections"]:
        if items:
            L += [f"## {name}", ""] + [f"- {md_heading(e)}" for e in items] + [""]
    out = "\n".join(L)
    if len(out.encode()) > SUMMARY_LIMIT:
        out = "\n".join(head + [f"The list of items is too long for the job summary; see {report_name}.", ""])
    return out


CSS = """
body { font: 14px/1.5 -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; margin: 2em auto;
       max-width: 70em; padding: 0 1em; color: #1f2328; background: #fff; }
code, pre { font: 12px/1.45 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
code { background: #eff1f3; padding: .1em .3em; border-radius: 4px; }
pre { background: #f6f8fa; padding: 1em; overflow-x: auto; white-space: pre-wrap; }
h3 { border-top: 1px solid #d1d9e0; padding-top: 1em; margin-top: 2em; font-size: 1.05em; }
table { border-collapse: collapse; width: 100%; }
th, td { border: 1px solid #d1d9e0; padding: .3em .6em; vertical-align: top; text-align: left; }
th { background: #f6f8fa; } td:first-child { white-space: nowrap; width: 1%; }
del { background: #ffebe9; color: #82071e; } ins { background: #dafbe1; color: #116329; text-decoration: none; }
em.type { color: #59636e; } nav li { display: inline; margin-right: 1.5em; }
"""


def html_page(rep):
    """The full report as a self-contained HTML page, without scripts or external styles."""
    title = f"Spec changes, {esc(rep['base'])} → {esc(rep['head'])}"
    L = ["<!DOCTYPE html>", '<html lang="en"><head><meta charset="utf-8">',
         '<meta name="viewport" content="width=device-width, initial-scale=1">',
         f"<title>{title}</title><style>{CSS}</style></head><body>",
         f"<h1>Spec changes, {code(rep['base'])} → {code(rep['head'])}</h1>", f"<p>{counts(rep)}</p>",
         "<nav><ul>" + "".join(f'<li><a href="#{name.lower()}">{name} ({len(items)})</a></li>'
                               for name, items in rep["sections"] if items) + "</ul></nav>"]
    listed = {e["id"] for _, items in rep["sections"] for e in items}
    for name, items in rep["sections"]:
        if not items:
            continue
        L.append(f'<h2 id="{name.lower()}">{name}</h2>')
        for e in items:
            old = f"{code(e['old_id'])} → " if e["old_id"] else ""
            L.append(f'<h3 id="{esc(e["id"])}">{old}{code(e["id"])} {esc(e["title"])} '
                     f'<em class="type">({esc(e["type"])})</em></h3>')
            if e["note"]:
                L.append(f"<p>{esc(e['note'])}</p>")
            if e["rows"]:
                L.append("<table><tr><th>Field</th><th>Change</th></tr>")
                L += [f"<tr><td>{code(f)}</td><td>{frag}</td></tr>" for f, frag in e["rows"]]
                L.append("</table>")
            if e["fields"]:
                L.append("<ul>" + "".join(f"<li>{code(f)}: {esc(v)}</li>" for f, v in e["fields"]) + "</ul>")
            if e["content"]:
                L.append(f"<pre>{esc(e['content'])}</pre>")
            if e["recheck"]:
                L.append("<p><strong>Re-check:</strong></p><ul>")
                L += [f'<li><a href="#{esc(o)}">{code(o)}</a> {esc(t)} — {why(l, d, g)}</li>'
                      if o in listed
                      else f"<li>{code(o)} {esc(t)} — {why(l, d, g)}</li>"
                      for o, t, l, d, g in e["recheck"]]
                L.append("</ul>")
    L.append("</body></html>")
    return "\n".join(L)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("base", nargs="?", help="base git revision")
    ap.add_argument("head", nargs="?", default="HEAD", help="head git revision (default HEAD)")
    ap.add_argument("--repo", default=".", help="git repository")
    ap.add_argument("--project", default=".", help="directory of conf.py, relative to the repository")
    ap.add_argument("--sphinx-build", default="sphinx-build")
    ap.add_argument("--base-json"), ap.add_argument("--head-json")
    ap.add_argument("--include-test-results", action="store_true",
                    help="also compare testfile, testsuite and testcase items")
    ap.add_argument("--rename-threshold", type=float, default=0.8)
    ap.add_argument("--summary", action="store_true",
                    help="print only the list of the items that changed, for a CI job summary")
    ap.add_argument("--html", metavar="FILE", help="also write the full report to FILE as an HTML page")
    a = ap.parse_args()
    log = lambda m: print(m, file=sys.stderr)

    def write_html(text):
        if a.html:
            os.makedirs(os.path.dirname(os.path.abspath(a.html)), exist_ok=True)
            with open(a.html, "w") as f:
                f.write(text)

    if a.base_json and a.head_json:
        base, head = json.load(open(a.base_json)), json.load(open(a.head_json))
        bl, hl = a.base_json, a.head_json
    elif a.base:
        try:
            head = build(a.repo, a.head, a.project, a.sphinx_build, log)
        except BuildFailed as e:
            sys.exit(str(e))
        try:
            base = build(a.repo, a.base, a.project, a.sphinx_build, log)
        except BuildFailed as e:
            # A change to the toolchain can leave the base unbuildable with the toolchain of the head,
            # which is not a failure of the change: the report says so instead.
            log(str(e))
            msg = f"No report: the specs at {a.base} could not be built with this toolchain."
            print(msg)
            write_html(f'<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Spec changes</title>'
                       f"</head><body><p>{esc(msg)}</p></body></html>\n")
            return
        bl, hl = a.base, a.head
        if base is None and head is None:
            msg = f"No spec items to compare: neither {bl} nor {hl} has a Sphinx project."
            print(msg)
            write_html(f'<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Spec changes</title>'
                       f"</head><body><p>{esc(msg)}</p></body></html>\n")
            return
        # A revision without the project has no items: every item of the other one is added or removed.
        if base is None:
            base, bl = EMPTY, f"{bl} (no specs)"
        if head is None:
            head, hl = EMPTY, f"{hl} (no specs)"
    else:
        ap.error("give BASE [HEAD] or --base-json and --head-json")
    excluded = set() if a.include_test_results else DEFAULT_EXCLUDED_TYPES
    rep = compare(base, head, bl, hl, excluded, a.rename_threshold)
    write_html(html_page(rep) + "\n")
    print(summary(rep, "the full report") if a.summary else markdown(rep))


if __name__ == "__main__":
    main()
