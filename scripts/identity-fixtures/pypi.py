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
from packaging.utils import canonicalize_name, canonicalize_version
from packaging.version import InvalidVersion, Version

VERSION_INPUTS = [
    # The vulnerability report: every spelling PyPI treats as release 1.0.
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
]

NAME_INPUTS = [
    "CalcBoxLite", "calcboxlite", "Calc_Box.Lite", "calc-box-lite", "Flask", "flask", "Flask_RESTful",
    "flask-restful", "flask.restful", "FLASK__RESTFUL", "zope.interface", "Zope._-.Interface", "name.", "",
]

VERSION_GROUPS = [
    ["1.0", "1.0.0", "1.0.0.0", "01.0", "v1.0", "V1.0", "0!1.0", "000!1.0", " 1.0\n"],
    ["1.0.0rc1", "1.0rc1", "1rc1", "1.0RC1", "1.0c1", "1.0preview1", "1.0pre1", "1.0-rc.1", "1.0.rc1"],
    ["1.0.post1", "1.0.0.post1", "1.0-01", "1.0-1", "1.0post1", "1.0.r1"],
    ["1.0+local.1", "1.0.0+local.1"],
    ["01!002.00", "1!2"],
]

VERSION_DISTINCT = [
    ["1.0", "1.1"], ["1.0", "1.0.1"], ["1.0", "1.0rc1"], ["1.0rc1", "1.0rc2"], ["1.0", "1.0.post1"],
    ["1.0.post1", "1.0.post2"], ["1.0", "1.0.dev0"], ["1.0.dev0", "1.0.dev1"], ["1.0+local.1", "1.0+other"],
    ["1!2", "2!1.0"], ["1.0", "1.0+local.1"], ["1.0", "1.0_1"],
]

NAME_GROUPS = [
    ["CalcBoxLite", "calcboxlite"],
    ["Flask_RESTful", "flask-restful", "flask.restful", "FLASK__RESTFUL"],
    ["zope.interface", "Zope._-.Interface"],
]

NAME_DISTINCT = [
    ["calcboxlite", "calc-box-lite"],
    ["calcboxlite", "Calc_Box.Lite"],
]


def version_row(raw):
    try:
        Version(raw)
        return {"input": raw, "canonical": canonicalize_version(raw), "parsed": True}
    except InvalidVersion:
        return {"input": raw, "canonical": raw, "parsed": False}


def main():
    fixture = {
        "ecosystem": "ECOSYSTEM_PYPI",
        "rule_version": 1,
        "source": f"packaging {packaging.__version__}: packaging.utils.canonicalize_version and canonicalize_name",
        "generator": "scripts/identity-fixtures/pypi.py",
        "versions": [version_row(v) for v in VERSION_INPUTS],
        "names": [{"input": n, "canonical": canonicalize_name(n)} for n in NAME_INPUTS],
        "version_groups": VERSION_GROUPS,
        "version_distinct": VERSION_DISTINCT,
        "name_groups": NAME_GROUPS,
        "name_distinct": NAME_DISTINCT,
    }
    json.dump(fixture, sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
