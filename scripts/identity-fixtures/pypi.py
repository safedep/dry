"""Generate the PyPI identity conformance fixture from the packaging reference implementation.

Run from the repository root:

    python3 -m venv /tmp/identity-fixtures
    /tmp/identity-fixtures/bin/pip install packaging==26.3
    /tmp/identity-fixtures/bin/python scripts/identity-fixtures/pypi.py > api/pb/testdata/identity/pypi.json

The Go test in api/pb reads the JSON and asserts that the Go fold agrees with
packaging on every row. Add inputs here, never edit the JSON by hand.
"""

import json
import sys

import packaging
from packaging.utils import InvalidName, canonicalize_name, canonicalize_version
from packaging.version import InvalidVersion, Version

VERSION_INPUTS = [
    # Spellings PyPI treats as release 1.0.
    "1.0", "1.0.0", "1.0.0.0", "01.0", "v1.0", "V1.0", "0!1.0", "0!v01.00.0", "000!1.0",
    # Trailing zeros with a suffix segment.
    "1.0.0rc1", "1.0rc1", "1rc1", "1.0.post1", "1.0.0.post1", "1.0.dev0", "1.0+local.1", "1.0.0+local.1",
    # Epoch.
    "01!002.00", "1!2", "2!1.0",
    # Pre-release spellings.
    "1.0RC1", "1.0alpha", "1.0a", "1.0_beta_02", "1.0b2", "1.0c1", "1.0preview1", "1.0pre1", "1.0-rc.1", "1.0.rc1",
    # Post-release spellings.
    "1.0-01", "1.0-1", "1.0R02", "1.0rev", "1.0.post", "1.0post1", "1.0.r1",
    # Dev.
    "1.0DEV", "1.0.dev", "1.0-dev1", "1.0dev1",
    # Local segments.
    "V01.0RC01.POST02.DEV03+LOCAL_004-ABC", "1.0+ubuntu-1", "1.0+ubuntu.1", "1.0+abc.007",
    # Whitespace.
    " 1.0\n", "\t2.0 ",
    # Large numbers.
    "999999999999999999999999999!1.0", "1.000000000000000000001",
    # Distinct releases that must not fold together.
    "1", "1.1", "1.0.1", "2.0", "1.0rc2", "1.0.post2", "1.0.dev1", "1.0+other",
    # Invalid under PEP 440: the fold keeps the raw string.
    "1.0_1", "1.0final", "1.0rc1rc2", "1.0.dev1.post1", "1!2!1.0", "1.0+local..1", "1.0+", "", "abc", "1.0.0-py3-none-any", "latest",
    # Non-ASCII. PEP 440 is an ASCII grammar, and the Kelvin sign case-folds to k.
    "1+\u212a", "1.0\u00a0", "\u0661.0",
    # Python's whitespace set is wider than Go's strings.TrimSpace: the four
    # information separators U+001C to U+001F strip too. U+FEFF and U+200B are not whitespace.
    "\x1c1.0\x1c", "\x1d1.0", "1.0\x1e", "\x1f1.0\x1f", "\x0b1.0\x0c", "\u20281.0", "\x851.0", "\ufeff1.0", "\u200b1.0",
    # Python's re.IGNORECASE lets exactly four non-ASCII letters match [a-z]:
    # U+0130, U+0131, U+017F and the Kelvin sign. packaging 21.3 to 26.0 read
    # 1.0po\u017ft1 as 1.post1 and 1.0+\u212a as 1+k; 26.1 made the grammar ASCII,
    # and PyPI runs 26.3. The fold follows PyPI and keeps them raw.
    "1.0po\u017ft1", "1.0.po\u017ft", "1.0prev\u0131ew1", "1.0PREV\u0130EW1", "1.0+\u212a", "1.0+\u017f", "1.0+\u0131", "1.0\u212a",
    # v comes before the epoch, so 0!v01.00.0 above is not a version.
    "v0!1.0",
    # Near misses of a keyword.
    "1.0p1", "1.0.p1", "1.0po1", "1.0ev1", "1.0de1", "1.0alp1",
]

NAME_INPUTS = [
    "DataToolKit", "datatoolkit", "Data_Tool.Kit", "data-tool-kit", "Flask", "flask", "Flask_RESTful",
    "flask-restful", "flask.restful", "FLASK__RESTFUL", "zope.interface", "Zope._-.Interface", "name.", "",
    # Non-ASCII: PyPI and packaging's name validator reject every one, so the
    # fold keeps them raw. Go's strings.ToLower would send U+0130 to the ASCII i.
    "\u0130nvoke", "\u212aeras", "\u017fix", "caf\u00e9",
]

VERSION_GROUPS = [
    ["1.0", "1.0.0", "1.0.0.0", "01.0", "v1.0", "V1.0", "0!1.0", "000!1.0", " 1.0\n", "\x1c1.0\x1c", "\x1f1.0\x1f"],
    ["1.0.0rc1", "1.0rc1", "1rc1", "1.0RC1", "1.0c1", "1.0preview1", "1.0pre1", "1.0-rc.1", "1.0.rc1"],
    ["1.0.post1", "1.0.0.post1", "1.0-01", "1.0-1", "1.0post1", "1.0.r1"],
    ["1.0+local.1", "1.0.0+local.1"],
    ["01!002.00", "1!2"],
]

VERSION_DISTINCT = [
    ["1.0", "1.1"], ["1.0", "1.0.1"], ["1.0", "1.0rc1"], ["1.0rc1", "1.0rc2"], ["1.0", "1.0.post1"],
    ["1.0.post1", "1.0.post2"], ["1.0", "1.0.dev0"], ["1.0.dev0", "1.0.dev1"], ["1.0+local.1", "1.0+other"],
    ["1!2", "2!1.0"], ["1.0", "1.0+local.1"], ["1.0", "1.0_1"],
    ["1.0po\u017ft1", "1.0.post1"], ["1.0+\u212a", "1.0+k"], ["1.0prev\u0131ew1", "1.0rc1"], ["1.0.post1", "1.0_1"],
]

NAME_GROUPS = [
    ["DataToolKit", "datatoolkit"],
    ["Flask_RESTful", "flask-restful", "flask.restful", "FLASK__RESTFUL"],
    ["zope.interface", "Zope._-.Interface"],
]

NAME_DISTINCT = [
    ["datatoolkit", "data-tool-kit"],
    ["datatoolkit", "Data_Tool.Kit"],
    ["\u0130nvoke", "invoke"],
    ["\u212aeras", "keras"],
    ["\u017fix", "six"],
]


def version_row(raw):
    try:
        Version(raw)
        return {"input": raw, "canonical": canonicalize_version(raw), "parsed": True}
    except InvalidVersion:
        return {"input": raw, "canonical": raw, "parsed": False}


def name_row(raw):
    if raw.isascii():
        return {"input": raw, "canonical": canonicalize_name(raw)}
    # The fold keeps a non-ASCII name raw. That is right only while packaging
    # agrees no such name is valid.
    try:
        canonicalize_name(raw, validate=True)
    except InvalidName:
        return {"input": raw, "canonical": raw}
    raise SystemExit(f"packaging {packaging.__version__} accepts the non-ASCII name {raw!r}")


def main():
    fixture = {
        "ecosystem": "ECOSYSTEM_PYPI",
        "rule_version": 1,
        "source": f"packaging {packaging.__version__}: packaging.utils.canonicalize_version and canonicalize_name",
        "generator": "scripts/identity-fixtures/pypi.py",
        "versions": [version_row(v) for v in VERSION_INPUTS],
        "names": [name_row(n) for n in NAME_INPUTS],
        "version_groups": VERSION_GROUPS,
        "version_distinct": VERSION_DISTINCT,
        "name_groups": NAME_GROUPS,
        "name_distinct": NAME_DISTINCT,
    }
    json.dump(fixture, sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
