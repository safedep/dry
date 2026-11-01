package conformance

import (
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/dry/api/pb"
	"github.com/safedep/dry/policy/match"
	"github.com/safedep/dry/policy/pbidentity"
	"github.com/safedep/dry/policy/project"
)

// decideWithDry is the path of pmg and the pull request check: project,
// match and pbidentity.
func decideWithDry(kind specv1.SubjectKind) DecideFunc {
	return func(raw []byte, q Query) (Decision, error) {
		proj, err := project.Decode(raw, Expect(kind))
		if err != nil {
			return Decision{}, err
		}

		index, _ := match.NewIndex(proj.Rules, pbidentity.Identity{})

		ecosystem, err := pb.EcosystemFromName(q.Package.Ecosystem)
		if err != nil {
			return Decision{}, err
		}

		pv := pb.NewPackageVersionFromParts(ecosystem, q.Package.Name, q.Package.Version)
		d := index.Decide(match.Query{
			Package: pbidentity.Package(pv),
			Control: q.Control,
			Verdict: q.Verdict,
			Now:     q.Now,
		})

		return Decision{Effect: d.Effect, RuleID: d.Rule.ID}, nil
	}
}

func TestCorpus(t *testing.T) {
	for _, kind := range []specv1.SubjectKind{
		specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
		specv1.SubjectKind_SUBJECT_KIND_PROJECT,
	} {
		t.Run(kind.String(), func(t *testing.T) {
			Run(t, kind, decideWithDry(kind))
		})
	}
}

func TestCasesHaveChecks(t *testing.T) {
	cases, err := Cases()
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	for _, c := range cases {
		assert.NotEmpty(t, c.Description, c.File)
		assert.NotEmpty(t, c.Checks, c.File)
		assert.Equal(t, "cases/"+c.Name+".json", c.File)
	}
}

func TestPolicyHeader(t *testing.T) {
	cases, err := Cases()
	require.NoError(t, err)

	tests := []struct {
		kind      specv1.SubjectKind
		subjectID string
		machineID string
	}{
		{kind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT, subjectID: EndpointID, machineID: MachineID},
		{kind: specv1.SubjectKind_SUBJECT_KIND_PROJECT, subjectID: ProjectID},
	}

	for _, tc := range tests {
		t.Run(tc.kind.String(), func(t *testing.T) {
			raw, err := cases[0].Policy(tc.kind)
			require.NoError(t, err)

			proj, err := project.Decode(raw, Expect(tc.kind))
			require.NoError(t, err)

			assert.Equal(t, Tenant, proj.Header.Tenant)
			assert.Equal(t, tc.kind, proj.Header.SubjectKind)
			assert.Equal(t, tc.subjectID, proj.Header.SubjectID)
			assert.Equal(t, tc.machineID, proj.Header.MachineID)
		})
	}
}

func TestPolicyUnknownSubjectKind(t *testing.T) {
	cases, err := Cases()
	require.NoError(t, err)

	_, err = cases[0].Policy(specv1.SubjectKind_SUBJECT_KIND_UNSPECIFIED)
	assert.Error(t, err)
}

// The unknown field must reach the tool, or the unknown-field case checks
// nothing.
func TestPolicyCarriesUnknownFields(t *testing.T) {
	cases, err := Cases()
	require.NoError(t, err)

	var unknownField Case
	for _, c := range cases {
		if c.Name == "unknown-field" {
			unknownField = c
		}
	}
	require.NotEmpty(t, unknownField.Name)

	kind := specv1.SubjectKind_SUBJECT_KIND_ENDPOINT
	raw, err := unknownField.Policy(kind)
	require.NoError(t, err)

	proj, err := project.Decode(raw, Expect(kind))
	require.NoError(t, err)

	var dropped, reported []string
	for _, e := range proj.Exceptions {
		switch e.Kind {
		case project.ExceptionDroppedRule:
			dropped = append(dropped, e.RuleID)
		case project.ExceptionUnknownField:
			reported = append(reported, e.RuleID)
		default:
		}
	}

	assert.Equal(t, []string{"r_trust_unknown"}, dropped)
	assert.Equal(t, []string{"r_block_unknown"}, reported)
}

type recorder struct {
	messages []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.messages = append(r.messages, fmt.Sprintf(format, args...))
}

func TestVerify(t *testing.T) {
	c := Case{File: "cases/exact-block.json"}
	chk := Check{
		Name: "every-version",
		Query: Query{
			Package: Package{Ecosystem: "npm", Name: "event-stream", Version: "3.3.6"},
			Control: match.ScopeMalware,
			Verdict: match.VerdictVerifiedMalware,
			Now:     issuedAt,
		},
		Want: Decision{Effect: match.EffectBlock, RuleID: "r_block_any"},
	}

	tests := []struct {
		name    string
		decide  DecideFunc
		wantOK  bool
		wantMsg string
	}{
		{
			name: "same decision",
			decide: func([]byte, Query) (Decision, error) {
				return Decision{Effect: match.EffectBlock, RuleID: "r_block_any"}, nil
			},
			wantOK: true,
		},
		{
			name: "other decision",
			decide: func([]byte, Query) (Decision, error) {
				return Decision{}, nil
			},
			wantMsg: `cases/exact-block.json, check "every-version": npm "event-stream" version "3.3.6", ` +
				`control malware, verdict verified-malware, now 2026-11-01T00:00:00Z: want block r_block_any, got none`,
		},
		{
			name: "other rule",
			decide: func([]byte, Query) (Decision, error) {
				return Decision{Effect: match.EffectBlock, RuleID: "r_other"}, nil
			},
			wantMsg: `cases/exact-block.json, check "every-version": npm "event-stream" version "3.3.6", ` +
				`control malware, verdict verified-malware, now 2026-11-01T00:00:00Z: want block r_block_any, got block r_other`,
		},
		{
			name: "error",
			decide: func([]byte, Query) (Decision, error) {
				return Decision{}, errors.New("cannot load")
			},
			wantMsg: `cases/exact-block.json, check "every-version": the tool returned an error: cannot load`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &recorder{}
			ok := verify(r, c, chk, []byte("policy"), tc.decide)

			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Empty(t, r.messages)
			} else {
				assert.Equal(t, []string{tc.wantMsg}, r.messages)
			}
		})
	}
}

func TestVerifyGivesACopyOfThePolicy(t *testing.T) {
	raw := []byte("policy")
	chk := Check{Name: "c", Want: Decision{}}

	verify(&recorder{}, Case{}, chk, raw, func(b []byte, _ Query) (Decision, error) {
		b[0] = 'X'
		return Decision{}, nil
	})

	assert.Equal(t, []byte("policy"), raw)
}

const validPolicy = `spec { core { package_rules {
  rule_id: "r_1"
  effect: EFFECT_BLOCK
  selector { purl: "pkg:npm/evil" }
  reason: "test"
} } }`

const validCheck = `{
  "name": "c",
  "package": {"ecosystem": "npm", "name": "evil", "version": "1.0.0"},
  "control": "malware",
  "verdict": "none",
  "want": {"effect": "block", "rule_id": "r_1"}
}`

func caseHeader(extra, check string) string {
	return `{"description": "d", "now": "2026-11-01T12:00:00Z",` + extra + ` "checks": [` + check + `]}`
}

func TestLoadCases(t *testing.T) {
	tests := []struct {
		name    string
		files   fstest.MapFS
		wantErr string
	}{
		{
			name: "valid",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader("", validCheck))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
		},
		{
			name:    "no case",
			files:   fstest.MapFS{"cases/README": {Data: []byte("x")}},
			wantErr: "cases/README: a case file ends with .json or .textproto",
		},
		{
			name: "no policy",
			files: fstest.MapFS{
				"cases/a.json": {Data: []byte(caseHeader("", validCheck))},
			},
			wantErr: "cases/a.json: no policy",
		},
		{
			name: "no header",
			files: fstest.MapFS{
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: "cases/a.textproto: no case header a.json",
		},
		{
			name: "unknown key",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader(`"nwo": "x",`, validCheck))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: json: unknown field "nwo"`,
		},
		{
			name: "policy sets a header",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader("", validCheck))},
				"cases/a.textproto": {Data: []byte(`header { tenant: "x" } ` + validPolicy)},
			},
			wantErr: "cases/a.textproto: the runner writes the header",
		},
		{
			name: "policy has no spec",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader("", validCheck))},
				"cases/a.textproto": {Data: []byte(``)},
			},
			wantErr: "cases/a.textproto: the policy has no spec",
		},
		{
			name: "unknown control",
			files: fstest.MapFS{
				"cases/a.json": {Data: []byte(caseHeader("", `{
  "name": "c", "package": {"ecosystem": "npm", "name": "evil"},
  "control": "license", "verdict": "none", "want": {"effect": "none"}}`))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: check "c": unknown control "license"`,
		},
		{
			name: "unknown ecosystem",
			files: fstest.MapFS{
				"cases/a.json": {Data: []byte(caseHeader("", `{
  "name": "c", "package": {"ecosystem": "npmjs", "name": "evil"},
  "control": "malware", "verdict": "none", "want": {"effect": "none"}}`))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: check "c": unknown ecosystem name: "npmjs"`,
		},
		{
			name: "rule id with no rule",
			files: fstest.MapFS{
				"cases/a.json": {Data: []byte(caseHeader("", `{
  "name": "c", "package": {"ecosystem": "npm", "name": "evil"},
  "control": "malware", "verdict": "none", "want": {"effect": "none", "rule_id": "r_1"}}`))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: check "c": a decision of a rule has a rule id`,
		},
		{
			name: "two checks with one name",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader("", validCheck+","+validCheck))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: check "c": two checks have this name`,
		},
		{
			name: "no time",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(`{"description": "d", "checks": [` + validCheck + `]}`)},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: check "c": the check has no time`,
		},
		{
			name: "unknown field of a known number",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader(`"unknown_fields": [{"rule_id": "r_1", "field": 5}],`, validCheck))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: "cases/a.json: unknown_fields: the contract knows field 5 of a package rule",
		},
		{
			name: "unknown field of no rule",
			files: fstest.MapFS{
				"cases/a.json":      {Data: []byte(caseHeader(`"unknown_fields": [{"rule_id": "r_2", "field": 99}],`, validCheck))},
				"cases/a.textproto": {Data: []byte(validPolicy)},
			},
			wantErr: `cases/a.json: unknown_fields: no package rule has the id "r_2"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cases, err := loadCases(tc.files, "cases")
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Len(t, cases, 1)
			assert.Equal(t, "a", cases[0].Name)
			assert.Equal(t, []Check{{
				Name: "c",
				Query: Query{
					Package: Package{Ecosystem: "npm", Name: "evil", Version: "1.0.0"},
					Control: match.ScopeMalware,
					Verdict: match.VerdictNone,
					Now:     issuedAt.Add(12 * time.Hour),
				},
				Want: Decision{Effect: match.EffectBlock, RuleID: "r_1"},
			}}, cases[0].Checks)
		})
	}
}
