package pb

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/package-url/packageurl-go"
	"github.com/stretchr/testify/require"
)

// Fuzz targets for the PyPI rule and the purl constructor. Plain go test runs
// the seeds only. Run one with
//
//	go test -run '^$' -fuzz '^FuzzPypiVersion$' -fuzztime 2m ./api/pb/
//
// With IDENTITY_ORACLE_PYTHON naming a Python that has the fixture's
// packaging release installed, the PyPI targets also check every input
// against packaging:
//
//	python3 -m venv /tmp/oracle && /tmp/oracle/bin/pip install packaging==26.3
//	IDENTITY_ORACLE_PYTHON=/tmp/oracle/bin/python go test -run '^$' -fuzz '^FuzzPypiVersion$' ./api/pb/
//
// Pin anything they find in scripts/identity-fixtures/pypi.py.

// pep440CanonicalPattern is the only shape canonicalize_version emits.
var pep440CanonicalPattern = regexp.MustCompile(`^(?:[1-9][0-9]*!)?` +
	`(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*))*` +
	`(?:(?:a|b|rc)(?:0|[1-9][0-9]*))?` +
	`(?:\.post(?:0|[1-9][0-9]*))?` +
	`(?:\.dev(?:0|[1-9][0-9]*))?` +
	`(?:\+[a-z0-9]+(?:\.[a-z0-9]+)*)?$`)

func FuzzPypiVersion(f *testing.F) {
	fixture := readPypiFixture(f)
	for _, row := range fixture.Versions {
		f.Add(row.Input)
	}
	oracle := startOracle(f, fixture)

	f.Fuzz(func(t *testing.T, raw string) {
		canonical, parsed := pep440Version(raw)
		if !parsed {
			require.Equal(t, raw, canonical, "an unparsed version stays raw")
		} else {
			require.Regexp(t, pep440CanonicalPattern, canonical)
			for _, variant := range []string{canonical, " " + raw + "\n", strings.ToUpper(raw)} {
				got, ok := pep440Version(variant)
				require.True(t, ok, variant)
				require.Equal(t, canonical, got, "%q and %q name one version", raw, variant)
			}
		}

		if want, ok := oracle.ask(t, "v", raw); ok {
			require.Equal(t, want, oracleAnswer{Canonical: canonical, Parsed: parsed}, "packaging disagrees on %q", raw)
		}
	})
}

func FuzzPypiName(f *testing.F) {
	fixture := readPypiFixture(f)
	for _, row := range fixture.Names {
		f.Add(row.Input)
	}
	oracle := startOracle(f, fixture)

	f.Fuzz(func(t *testing.T, raw string) {
		canonical := pypiIdentityName(raw)
		if !isASCII(raw) {
			require.Equal(t, raw, canonical, "a non-ASCII name stays raw")
		} else {
			for _, variant := range []string{canonical, strings.ToUpper(strings.ReplaceAll(raw, "_", "."))} {
				require.Equal(t, canonical, pypiIdentityName(variant), "%q and %q name one project", raw, variant)
			}
		}

		if want, ok := oracle.ask(t, "n", raw); ok {
			if isASCII(raw) {
				require.Equal(t, want.Canonical, canonical, "packaging disagrees on %q", raw)
			} else {
				// packaging lower-cases a non-ASCII name. Keeping it raw is
				// safe only while packaging calls every such name invalid.
				require.False(t, want.Valid, "packaging accepts the non-ASCII name %q", raw)
			}
		}
	})
}

// FuzzNewPackageVersionFromPurl checks the purl constructor against
// packageurl-go, the parts constructor and its own URN.
func FuzzNewPackageVersionFromPurl(f *testing.F) {
	for _, seed := range []string{
		"pkg:pypi/Flask_RESTful@0.3.10.0",
		"pkg:pypi/flask@1.0?extension=whl#sub/path",
		"pkg://pypi/name@1.0%40x",
		"pkg://pypi/ns%2Frequests@2.0",
		"pkg://npm/%2E%2E/pypi/x@1",
		"pkg:pypi/ns/requests@2.0",
		"pkg:npm/%40angular/core@1.0.0",
		"pkg:golang/github.com//Azure//x@v1",
		"pkg:composer/Laravel/Framework@10.0",
		"pkg:maven/org.apache/commons-lang3@3.0",
		"pkg:mAven/:0",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, purl string) {
		pv, err := NewPackageVersionFromPurl(purl)

		// For the opaque pkg:type/... form the constructor accepts what the
		// parser accepts, except a namespace on a type that names a package
		// without one, which the name mapping would drop. The pkg:/ and
		// pkg:// forms may be rejected where the parser resolves their
		// decoded path.
		parsed, parserErr := packageurl.FromString(purl)
		opaque := !strings.HasPrefix(purl[strings.Index(purl, ":")+1:], "/")
		ecosystem := purlMapEcosystem(parsed.Type)
		droppedNamespace := parsed.Namespace != "" && purlIdentityName(ecosystem, parsed) == parsed.Name
		if parserErr == nil && opaque && !droppedNamespace && ecosystem != packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED {
			require.NoError(t, err, "packageurl-go accepts %q", purl)
		}
		if err != nil {
			return
		}

		parts := NewPackageVersionFromParts(pv.Ecosystem(), pv.RawName(), pv.RawVersion())
		require.Equal(t, parts.Key(), pv.Key(), "%q and its parts name one identity", purl)
		require.True(t, pv.Equal(parts))

		if urn, err := pv.URN(); err == nil {
			back, err := NewPackageVersionFromPurl(urn)
			require.NoError(t, err, urn)
			require.Equal(t, pv.Key(), back.Key(), "%q and its URN %q name one identity", purl, urn)
		}
	})
}

func readPypiFixture(tb testing.TB) identityFixture {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "identity", "pypi.json"))
	require.NoError(tb, err)
	var fixture identityFixture
	require.NoError(tb, json.Unmarshal(data, &fixture))
	return fixture
}

// oracleScript answers one JSON request per line: {"v": s} with packaging's
// canonical version, {"n": s} with its canonical name. Its first line is the
// packaging release.
const oracleScript = `
import json, sys
import packaging
from packaging.utils import InvalidName, canonicalize_name, canonicalize_version
from packaging.version import InvalidVersion, Version

print(json.dumps(packaging.__version__), flush=True)
for line in sys.stdin:
    request = json.loads(line)
    if "v" in request:
        try:
            Version(request["v"])
            answer = {"canonical": canonicalize_version(request["v"]), "parsed": True}
        except InvalidVersion:
            answer = {"canonical": request["v"], "parsed": False}
    else:
        try:
            canonicalize_name(request["n"], validate=True)
            valid = True
        except InvalidName:
            valid = False
        answer = {"canonical": canonicalize_name(request["n"]), "valid": valid}
    print(json.dumps(answer), flush=True)
`

type oracleAnswer struct {
	Canonical string `json:"canonical"`
	Parsed    bool   `json:"parsed"`
	Valid     bool   `json:"valid"`
}

// packagingOracle is one Python process per fuzz worker. A nil oracle answers
// nothing, so the targets run without Python.
type packagingOracle struct {
	enc *json.Encoder
	out *bufio.Scanner
}

func startOracle(f *testing.F, fixture identityFixture) *packagingOracle {
	f.Helper()
	python := os.Getenv("IDENTITY_ORACLE_PYTHON")
	if python == "" {
		return nil
	}

	cmd := exec.CommandContext(f.Context(), python, "-c", oracleScript)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	require.NoError(f, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(f, err)
	// The context ends before the cleanup runs. Closing stdin lets the oracle
	// finish its loop and exit, where the default cancel would kill it.
	cmd.Cancel = stdin.Close
	require.NoError(f, cmd.Start())
	f.Cleanup(func() {
		// A clean exit after the cancel reports the context error, so only
		// another error means the oracle failed.
		if err := cmd.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			f.Errorf("oracle: %v", err)
		}
	})

	o := &packagingOracle{enc: json.NewEncoder(stdin), out: bufio.NewScanner(stdout)}
	o.out.Buffer(nil, 1<<24)

	// Older packaging accepts spellings PyPI rejects, so an oracle on another
	// release than the fixture's would judge the fold by the wrong rule.
	var version string
	require.True(f, o.out.Scan(), "the oracle did not start")
	require.NoError(f, json.Unmarshal(o.out.Bytes(), &version))
	require.True(f, strings.HasPrefix(fixture.Source, "packaging "+version+":"),
		"the oracle runs packaging %s, the fixture says %q", version, fixture.Source)
	return o
}

// ask returns packaging's answer for raw, or false when there is no oracle or
// raw is not valid UTF-8, which a Python str cannot hold.
func (o *packagingOracle) ask(t *testing.T, kind, raw string) (oracleAnswer, bool) {
	t.Helper()
	var answer oracleAnswer
	if o == nil || !utf8.ValidString(raw) {
		return answer, false
	}
	require.NoError(t, o.enc.Encode(map[string]string{kind: raw}))
	require.True(t, o.out.Scan(), "oracle: %v", o.out.Err())
	require.NoError(t, json.Unmarshal(o.out.Bytes(), &answer))
	return answer, true
}
