#!/usr/bin/env python3
"""Diff the Sphinx-Needs items of a documentation project between two git revisions.

Builds needs.json at each revision (or reads two given files), compares the items by ID,
and prints a Markdown report: added, removed, renamed and changed items, a word-level diff
of each changed field, and the linked items to re-check for each change.

  needs_diff.py --repo DIR --project SUBDIR BASE [HEAD]
  needs_diff.py --base-json A.json --head-json B.json
"""
import argparse, difflib, html, json, os, re, shutil, subprocess, sys, tempfile

# Core fields that describe where or how an item is rendered, not what it says.
IGNORED_CORE = {
    "lineno", "docname", "doctype", "external_css", "external_url", "type_name", "layout",
    "style", "template", "pre_template", "post_template", "pre_content", "post_content",
    "jinja_content", "is_external", "is_import", "is_modified", "modifications",
    "has_dead_links", "has_forbidden_dead_links", "constraints_passed", "constraints_results",
    "constraints_error", "section_name", "parent_need", "signature", "parts", "project",
}
DEFAULT_EXCLUDED_TYPES = {"testfile", "testsuite", "testcase"}


def build(repo, rev, project, sphinx_build, log):
    """Build needs.json for rev in a temporary detached worktree; return its parsed content."""
    tmp = tempfile.mkdtemp(prefix="needs-diff-")
    wt = os.path.join(tmp, "wt")
    subprocess.run(["git", "-C", repo, "worktree", "add", "--quiet", "--detach", wt, rev], check=True)
    try:
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
            sys.exit(f"sphinx-build failed at {rev}:\n{r.stderr[-2000:]}")
        with open(os.path.join(out, "needs.json")) as f:
            return json.load(f)
    finally:
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
        elif kind == "core" and name in IGNORED_CORE:
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
        # Line breaks and indentation are formatting; collapse them so a reformat is not a change.
        return re.sub(r"\s+", " ", v).strip()
    return v


WORD = re.compile(r"\S+|\s+")


def word_diff(a, b):
    """Inline word diff rendered with <del>/<ins>, which GitHub Markdown displays."""
    aw, bw = WORD.findall(a), WORD.findall(b)
    out = []
    for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, aw, bw, autojunk=False).get_opcodes():
        old, new = "".join(aw[i1:i2]), "".join(bw[j1:j2])
        if tag == "equal":
            out.append(html.escape(old))
        else:
            if old.strip():
                out.append(f"<del>{html.escape(old)}</del>")
            if new.strip():
                out.append(f"<ins>{html.escape(new)}</ins>")
            elif new:
                out.append(new)
    return "".join(out).replace("\n", "<br>")


def cell(s):
    return s.replace("|", "\\|")


def render_change(field, old, new):
    if isinstance(old, list) or isinstance(new, list):
        o, n = set(old or []), set(new or [])
        parts = [f"+`{x}`" for x in sorted(n - o)] + [f"−`{x}`" for x in sorted(o - n)]
        return f"| `{field}` | {' '.join(parts)} |"
    if isinstance(old, str) and isinstance(new, str) and (len(old) > 40 or len(new) > 40):
        return f"| `{field}` | {cell(word_diff(old, new))} |"
    show = lambda v: "*(none)*" if v is None else f"`{v}`"
    return f"| `{field}` | {cell(show(old))} → {cell(show(new))} |"


def similarity(a, b):
    ta = (a.get("title") or "") + " " + (a.get("content") or "")
    tb = (b.get("title") or "") + " " + (b.get("content") or "")
    return difflib.SequenceMatcher(None, ta.split(), tb.split(), autojunk=False).ratio()


def related(need, links, back):
    """Linked items of need, as (other id, description) pairs."""
    rel = []
    for l in links:
        for other in need.get(l) or []:
            rel.append((other, f"this → `{l}` → it"))
    for b in back:
        for other in need.get(b) or []:
            rel.append((other, f"it → `{b[:-5]}` → this"))
    return rel


def report(base, head, base_label, head_label, excluded_types, rename_threshold):
    bn, bschema = current(base)
    hn, hschema = current(head)
    schema = {**bschema, **hschema}
    compared, links, back = field_kinds(schema)
    keep = lambda n: n.get("type") not in excluded_types
    bn = {k: v for k, v in bn.items() if keep(v)}
    hn = {k: v for k, v in hn.items() if keep(v)}

    added = sorted(set(hn) - set(bn))
    removed = sorted(set(bn) - set(hn))
    renamed = []
    for r in list(removed):
        best = max(((similarity(bn[r], hn[a]), a) for a in added if hn[a].get("type") == bn[r].get("type")),
                   default=(0, None))
        if best[0] >= rename_threshold:
            renamed.append((r, best[1], best[0]))
            removed.remove(r)
            added.remove(best[1])

    def diffs(old, new):
        rows = []
        for f in compared:
            if f == "id":
                continue
            o, n = norm(old.get(f)), norm(new.get(f))
            if o != n:
                rows.append(render_change(f, o, n))
        return rows

    changed = []
    for i in sorted(set(bn) & set(hn)):
        d = diffs(bn[i], hn[i])
        if d:
            changed.append((i, d))

    def title(nid, n):
        return f"`{nid}` {n.get('title', '')} *({n.get('type')})*"

    def recheck(nid, need, old=None):
        now = set(related(need, links, back))
        before = set(related(old, links, back)) if old else set()
        rel = [(o, why, "") for o, why in now] + [(o, why, " (until this change)") for o, why in before - now]
        rel = [r for r in rel if r[0] in hn]
        if not rel:
            return []
        out = ["", "**Re-check:**", ""]
        for o, why, when in sorted(set(rel)):
            out.append(f"- `{o}` {hn[o].get('title', '')} — {why}{when}")
        return out

    L = [f"# Spec changes, `{base_label}` → `{head_label}`", "",
         f"{len(changed)} changed, {len(added)} added, {len(removed)} removed, {len(renamed)} renamed.", ""]
    if changed:
        L += ["## Changed", ""]
        for i, d in changed:
            L += [f"### {title(i, hn[i])}", "", "| Field | Change |", "| --- | --- |", *d]
            L += recheck(i, hn[i], bn[i])
            L.append("")
    if renamed:
        L += ["## Renamed", ""]
        for r, a, s in renamed:
            L += [f"### `{r}` → {title(a, hn[a])}", "", f"Matched by {s:.0%} similar title and content."]
            d = diffs(bn[r], hn[a])
            if d:
                L += ["", "| Field | Change |", "| --- | --- |", *d]
            L += recheck(a, hn[a], bn[r])
            L.append("")
    if added:
        L += ["## Added", ""]
        for a in added:
            n = hn[a]
            L += [f"### {title(a, n)}", ""]
            for f in compared:
                v = norm(n.get(f))
                if v is not None and f not in ("id", "title", "type", "content"):
                    L.append(f"- `{f}`: {', '.join(v) if isinstance(v, list) else v}")
            if n.get("content"):
                L += ["", "> " + n["content"].strip().replace("\n", "\n> ")]
            L += recheck(a, n)
            L.append("")
    if removed:
        L += ["## Removed", ""]
        for r in removed:
            L += [f"### {title(r, bn[r])}", ""]
            L += recheck(r, {}, bn[r])
            L.append("")
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
    a = ap.parse_args()
    log = lambda m: print(m, file=sys.stderr)
    if a.base_json and a.head_json:
        base, head = json.load(open(a.base_json)), json.load(open(a.head_json))
        bl, hl = a.base_json, a.head_json
    elif a.base:
        base = build(a.repo, a.base, a.project, a.sphinx_build, log)
        head = build(a.repo, a.head, a.project, a.sphinx_build, log)
        bl, hl = a.base, a.head
        if base is None or head is None:
            print(f"No spec items to compare: `{bl if base is None else hl}` has no Sphinx project.")
            return
    else:
        ap.error("give BASE [HEAD] or --base-json and --head-json")
    excluded = set() if a.include_test_results else DEFAULT_EXCLUDED_TYPES
    print(report(base, head, bl, hl, excluded, a.rename_threshold))


if __name__ == "__main__":
    main()
