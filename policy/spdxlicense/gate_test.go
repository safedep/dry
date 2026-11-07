package spdxlicense

import (
	"go/build"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// riskyLicenses is the deny list of default policy revision 2.
var riskyLicenses = []string{
	"GPL-2.0", "GPL-2.0-only", "GPL-2.0-or-later",
	"GPL-3.0", "GPL-3.0-only", "GPL-3.0-or-later",
	"AGPL-3.0", "AGPL-3.0-only", "AGPL-3.0-or-later",
}

func TestGateEvaluate(t *testing.T) {
	cases := []struct {
		name      string
		settings  GateSettings
		values    []string
		want      Verdict
		wantFails bool
	}{
		{"default deny fails a listed id", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-3.0-only"}, Denied, true},
		{"default deny fails a listed id in another case", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"gpl-3.0-only"}, Denied, true},
		{"default deny fails the deprecated GPL-2.0+", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-2.0+"}, Denied, true},
		{"default deny fails the deprecated GPL-3.0+", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-3.0+"}, Denied, true},
		{"default deny fails GPL-2.0-only+", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-2.0-only+"}, Denied, true},
		{"default deny fails an AND with a listed id", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-3.0-only AND MIT"}, Denied, true},
		{"default deny fails an OR of listed ids only", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-3.0-only OR AGPL-3.0-only"}, Denied, true},
		{"default deny passes an OR with a choice", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"MIT OR GPL-3.0-only"}, Pass, false},
		{"default deny passes a WITH term", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-2.0-only WITH Classpath-exception-2.0"}, Pass, false},
		{"default deny passes MIT", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"MIT"}, Pass, false},
		{"an empty deny list passes GPL", GateSettings{Deny: []string{}, Unknown: UnknownAllow}, []string{"GPL-3.0-only"}, Pass, false},
		{"an allow list fails a package outside it", GateSettings{Allow: []string{"MIT"}, Unknown: UnknownAllow}, []string{"Apache-2.0"}, NotAllowed, true},
		{"an allow list passes a package in it", GateSettings{Allow: []string{"MIT"}, Unknown: UnknownAllow}, []string{"MIT"}, Pass, false},
		{"an empty allow list is off", GateSettings{Allow: []string{}, Unknown: UnknownAllow}, []string{"Apache-2.0"}, Pass, false},
		{"the deny list goes first", GateSettings{Deny: []string{"GPL-3.0-only"}, Allow: []string{"GPL-3.0-only"}, Unknown: UnknownAllow}, []string{"GPL-3.0-only"}, Denied, true},
		{"unknown allow passes no license data", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, nil, Pass, false},
		{"unknown report reports no license data", GateSettings{Deny: riskyLicenses, Unknown: UnknownReport}, nil, Unknown, false},
		{"unknown fail fails no license data", GateSettings{Deny: riskyLicenses, Unknown: UnknownFail}, nil, Unknown, true},
		{"unknown fail fails a value that is not SPDX", GateSettings{Unknown: UnknownFail}, []string{"non-standard"}, Unknown, true},
		{"unknown fail fails NOASSERTION", GateSettings{Unknown: UnknownFail}, []string{"NOASSERTION"}, Unknown, true},
		{"NONE is unknown with no allow list", GateSettings{Deny: riskyLicenses, Unknown: UnknownFail}, []string{"NONE"}, Unknown, true},
		{"NONE fails an allow list", GateSettings{Allow: []string{"MIT"}, Unknown: UnknownAllow}, []string{"NONE"}, NotAllowed, true},
		{"unknown fail does not change a known license", GateSettings{Deny: riskyLicenses, Unknown: UnknownFail}, []string{"MIT"}, Pass, false},
		{"a denied value decides beside an unknown value", GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow}, []string{"GPL-3.0-only", "non-standard"}, Denied, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := NewGate(tc.settings)
			require.NoError(t, err)

			got := g.Evaluate(tc.values)
			assert.Equal(t, tc.want, got.Verdict)
			assert.Equal(t, tc.wantFails, got.Fails)
			if tc.want == Pass {
				assert.Empty(t, got.Denied, "a pass names no denied term")
			}
		})
	}
}

func TestGateEvaluateNamesTheDeniedTerms(t *testing.T) {
	g, err := NewGate(GateSettings{Deny: riskyLicenses, Unknown: UnknownAllow})
	require.NoError(t, err)

	assert.Equal(t, []string{"GPL-2.0-or-later"}, g.Evaluate([]string{"MIT", "GPL-2.0+"}).Denied)
}

func TestNewGateRefuses(t *testing.T) {
	cases := []struct {
		name     string
		settings GateSettings
		wantErr  error
		wantText string
	}{
		{"no unknown action", GateSettings{Deny: riskyLicenses}, ErrUnknownAction, ""},
		{"a later unknown action", GateSettings{Unknown: UnknownAction(4)}, ErrUnknownAction, ""},
		{"a set name in the allow list", GateSettings{Allow: []string{SetOSIApproved}, Unknown: UnknownAllow}, ErrInvalidEntry, "allow:"},
		{"an expression in the deny list", GateSettings{Deny: []string{"MIT OR GPL-3.0-only"}, Unknown: UnknownAllow}, ErrInvalidEntry, "deny:"},
		{"an id that is not on the list", GateSettings{Deny: []string{"GPL"}, Unknown: UnknownAllow}, ErrInvalidEntry, "deny:"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := NewGate(tc.settings)
			assert.Nil(t, g)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.ErrorContains(t, err, tc.wantText)
		})
	}
}

func TestValidateEntry(t *testing.T) {
	cases := []struct {
		entry    string
		valid    bool
		wantText string
	}{
		{"MIT", true, ""},
		{"mit", true, ""},
		{"GPL-2.0", true, ""},
		{"GPL-2.0+", true, ""},
		{"GPL-2.0-only+", true, ""},
		{"GPL-2.0-only WITH Classpath-exception-2.0", true, ""},
		{"LicenseRef-acme", true, ""},
		{" MIT ", true, ""},
		{"(MIT)", true, ""},
		{"GPL-2.0-only with Classpath-exception-2.0", true, ""},
		{SetOSIApproved, false, "set name"},
		{SetFSFLibre, false, "set name"},
		{SetOSIApprovedOrFSF, false, "set name"},
		{"MIT OR Apache-2.0", false, ""},
		{"MIT AND Apache-2.0", false, ""},
		{"GPL", false, ""},
		{"Apache 2.0", false, ""},
		{"MIT WITH nope", false, ""},
		{"GPL-2.0-or-later+", false, ""},
		{"LicenseRef-", false, ""},
		{"", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.entry, func(t *testing.T) {
			err := ValidateEntry(tc.entry)
			if tc.valid {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrInvalidEntry)
			assert.ErrorContains(t, err, tc.wantText)
		})
	}
}

// NOTICE names the version of the SPDX License List in list.json. go
// generate changes list.json when the pinned commit changes, and NOTICE must
// follow.
func TestNoticeNamesTheListVersion(t *testing.T) {
	notice, err := os.ReadFile("NOTICE")
	require.NoError(t, err)

	assert.Contains(t, string(notice), "version "+list.Version+",")
	assert.Contains(t, string(notice), "released "+strings.SplitN(list.ReleaseDate, "T", 2)[0])
	assert.Contains(t, string(notice), "CC-BY-3.0")
}

// TestNoContractImport keeps the rule that the package has no import of the
// SafeDep API contract. vet allows the contract only in its enrichers and
// cloud plugins, and its license control calls this package.
func TestNoContractImport(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	require.NoError(t, err)

	for _, imp := range pkg.Imports {
		assert.False(t, strings.HasPrefix(imp, "buf.build/"), "spdxlicense imports %s", imp)
		assert.False(t, strings.HasPrefix(imp, "google.golang.org/protobuf"), "spdxlicense imports %s", imp)
	}
}
