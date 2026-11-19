"""Parse each expectation's ``observed_from`` into the vantage points that it names.

``observed_from`` is written for a reader, under this grammar, in which "or" binds tighter than
"→", and "→" tighter than ",":

    observed_from = part { "," part }
    part          = side [ "→" side ]      ; traffic sent from the first side is observed at the second
    side          = code { " or " code } [ narrowing ]
    narrowing     = "(" text ")" | preposition words
                                           ; narrows every vantage point of the side

A code is a vantage point of the subsystem of the expectation's document. A narrowing is in
parentheses or starts with a preposition, such as "on two nodes": anything else after a code, such
as a second code, "or" without a code after it, or "->" for "→", is an error, not a narrowing.

The build sets three fields from it, which filters can select by, and checks after the needextend
directives that none of them set these fields or changed observed_from:

- ``vantage``: every vantage point that it names;
- ``observed_at``: the vantage points at which the expectation is observed;
- ``sent_from``: the vantage points that send what is observed, before a "→".

A value that does not parse is a warning.
"""

import re

from sphinx.util import logging

LOGGER = logging.getLogger(__name__)

#: The words that a narrowing without parentheses starts with.
#: Words of direction, such as "to" and "from", are left out: "underlay to task" is an arrow
#: written in words, not a narrowing.
PREPOSITIONS = ("on", "in", "at", "with", "behind", "between", "inside", "outside")

#: The fields that the build sets from observed_from.
DERIVED = ("vantage", "observed_at", "sent_from")


def parser(codes):
    """A function that parses a value of observed_from into (parts, errors).

    Each part is a dict with the codes it is ``sent_from`` (empty without a "→"), the codes it is
    observed ``at``, and its ``narrowing`` or None.
    """
    alt = "|".join(re.escape(c) for c in sorted(codes, key=len, reverse=True))
    side = re.compile(rf"(?P<codes>(?:{alt})(?: or (?:{alt}))*)(?![\w:-])(?: (?P<narrowing>.+))?")
    code_start = re.compile(rf"(?:{alt})(?![\w:-])")

    colon_codes = [re.compile(rf"(?<![\w:-]){re.escape(c)}(?![\w:-])") for c in codes if ":" in c]

    def parse(value):
        parts, errors = [], []
        if not _balanced(value):
            return parts, [f"{value!r} has unbalanced parentheses"]
        for raw in _split(value, ","):
            raw = raw.strip()
            sides = [s.strip() for s in _split(raw, "→")]
            if not all(sides):
                errors.append(f"{value!r} has an empty vantage point")
                continue
            if len(sides) > 2:
                errors.append(f"{raw!r} has more than one '→'")
                continue
            parsed = []
            for s in sides:
                m = side.fullmatch(s)
                if not m:
                    errors.append(f"{s!r} does not start with a vantage point")
                    break
                narrowing = m.group("narrowing")
                if narrowing and code_start.match(narrowing):
                    errors.append(f"{s!r} names two vantage points: separate them with ',' or ' or '")
                    break
                if narrowing and not (re.fullmatch(r"\([^()]*\)", narrowing)
                                      or (narrowing.split(" ", 1)[0] in PREPOSITIONS
                                          and not any(rx.search(narrowing) for rx in colon_codes))):
                    errors.append(f"{s!r}: words after a vantage point are in parentheses or start with a "
                                  f"preposition, such as 'on'")
                    break
                parsed.append((m.group("codes").split(" or "), narrowing))
            else:
                sent, at = parsed if len(parsed) == 2 else (([], None), parsed[0])
                parts.append(dict(sent_from=sent[0], at=at[0], narrowing=at[1] or sent[1]))
        return parts, errors

    return parse


def _balanced(text):
    depth = 0
    for c in text:
        depth += {"(": 1, ")": -1}.get(c, 0)
        if depth < 0:
            return False
    return depth == 0


def _split(text, sep):
    """text split at each sep outside parentheses."""
    out, depth, begin = [], 0, 0
    for i, c in enumerate(text):
        depth += {"(": 1, ")": -1}.get(c, 0)
        if c == sep and depth == 0:
            out.append(text[begin:i])
            begin = i + 1
    out.append(text[begin:])
    return out


def _derived(parse, value):
    parts, errors = parse(value)
    at, sent = [], []
    for p in parts:
        at += [c for c in p["at"] if c not in at]
        sent += [c for c in p["sent_from"] if c not in sent]
    return dict(vantage=sent + [c for c in at if c not in sent], observed_at=at, sent_from=sent), errors


def _parse_for(app, need, parsers, warn=True):
    """The parser of the subsystem of need's document, or None after a warning if warn."""
    subsystem = (need.get("docname") or "").split("/", 1)[0]
    codes = app.config.spec_subsystem_vocabularies.get(subsystem, {}).get("observed_from")
    if codes is None:
        if warn:
            LOGGER.warning(f"{need['id']}: no vantage points are listed for the subsystem {subsystem!r}",
                           location=(need["docname"], need["lineno"]), type="spec", subtype="observed_from")
        return None
    return parsers.setdefault(subsystem, parser(codes))


def derive(app, needs):
    parsers = {}
    app.env.spec_observed_from = seen = {}
    app.env.spec_observed_written = written = set()
    for need in needs.values():
        loc = (need["docname"], need["lineno"])
        for field in DERIVED:
            if need.get(field):
                written.add((need["id"], field))
                LOGGER.warning(f"{need['id']}: {field} is computed from observed_from, never written",
                               location=loc, type="spec", subtype="observed_from")
        value = need.get("observed_from")
        if need["type"] != "expectation":
            continue
        seen[need["id"]] = value
        if not value:
            continue
        parse = _parse_for(app, need, parsers)
        if parse is None:
            continue
        # an option value that wraps keeps its line break
        fields, errors = _derived(parse, " ".join(value.split()))
        for e in errors:
            LOGGER.warning(f"{need['id']}: observed_from: {e}", location=loc, type="spec", subtype="observed_from")
        for field, derived in fields.items():
            need[field] = derived


def check(app, needs):
    """After the needextend directives: they neither set a derived field nor change observed_from,
    which the fields were derived from before the directives' filters ran."""
    parsers = {}
    seen = getattr(app.env, "spec_observed_from", {})
    for need in needs.values():
        loc = (need["docname"], need["lineno"])
        if need["type"] != "expectation":
            for field in DERIVED:
                if need.get(field) and (need["id"], field) not in getattr(app.env, "spec_observed_written", ()):
                    LOGGER.warning(f"{need['id']}: {field} is computed from observed_from, never written",
                                   location=loc, type="spec", subtype="observed_from")
            continue
        if need.get("observed_from") != seen.get(need["id"]):
            LOGGER.warning(f"{need['id']}: a needextend changes observed_from, which the build parses before "
                           f"the needextend directives", location=loc, type="spec", subtype="observed_from")
            continue
        if not need.get("observed_from"):
            continue
        parse = _parse_for(app, need, parsers, warn=False)
        if parse is None:
            continue
        fields, _ = _derived(parse, " ".join(need["observed_from"].split()))
        for field in DERIVED:
            if (need.get(field) or []) != fields[field]:
                LOGGER.warning(f"{need['id']}: {field} is computed from observed_from, never written",
                               location=loc, type="spec", subtype="observed_from")


def setup(app):
    app.add_config_value("spec_subsystem_vocabularies", {}, "env")
    # before the needextend directives, whose filters can select by the derived fields
    app.connect("needs-before-post-processing", derive)
    app.connect("needs-before-sealing", check)
    return {"parallel_read_safe": True, "parallel_write_safe": True}
