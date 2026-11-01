package conformance

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/safedep/dry/api/pb"
	"github.com/safedep/dry/policy/match"
	"github.com/safedep/dry/policy/project"
)

// The directory is not "testdata", because a tool runs the corpus from
// another module, and the go command can leave testdata out of a vendor
// directory.
//
//go:embed cases
var casesFS embed.FS

const casesDir = "cases"

// The runner writes these values into the header of each policy. The loader
// of a tool must expect them. Expect returns them as a project.Expect.
const (
	Tenant     = "conformance.acme.safedep.io"
	MachineID  = "m_conformance"
	EndpointID = "ep_conformance"
	ProjectID  = "prj_conformance"
)

// issuedAt is before the time of each check, so a tool that refuses a policy
// from the future accepts it.
var issuedAt = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

// Package is a package as a tool sees it at decision time, before the tool
// puts it in canonical form.
type Package struct {
	// Ecosystem is the ecosystem name of pb.EcosystemName, as "npm", "pypi",
	// "nuget" or "github-actions". pb.EcosystemFromName reads it.
	Ecosystem string

	// Name is the name as the registry or the manifest gives it. It can
	// differ in case or in separators from the selector of the rule.
	Name string

	// Version is "" when the tool has no version.
	Version string
}

// Query is what a tool has when it decides.
type Query struct {
	Package Package

	// Control is the control that asks.
	Control match.Scope

	Verdict match.Verdict

	// Now is the time of the check. It is never zero.
	Now time.Time
}

func (q Query) String() string {
	verdict := fmt.Sprintf("verdict(%d)", int(q.Verdict))
	for name, v := range verdicts {
		if v == q.Verdict {
			verdict = name
		}
	}

	return fmt.Sprintf("%s %q version %q, control %s, verdict %s, now %s",
		q.Package.Ecosystem, q.Package.Name, q.Package.Version, q.Control, verdict, q.Now.Format(time.RFC3339))
}

// Decision is the result that the corpus checks.
type Decision struct {
	// Effect is match.EffectNone when no rule decides.
	Effect match.Effect

	// RuleID is the id of the rule that decides, or "" when no rule
	// decides. A block entry with no id has the id match.BlockRuleIDNone.
	RuleID string
}

func (d Decision) String() string {
	if d.RuleID == "" {
		return d.Effect.String()
	}
	return d.Effect.String() + " " + d.RuleID
}

// Check is one query against the policy of a case, and the decision that it
// expects.
type Check struct {
	Name  string
	Query Query
	Want  Decision
}

// Case is one resolved policy and the checks that run against it.
type Case struct {
	// Name is the base name of the case files.
	Name string

	// File is the path of the case header, as "cases/exact-block.json". A
	// failure names it.
	File string

	Description string
	Checks      []Check

	policy  *specv1.ResolvedPolicy
	unknown []unknownField
}

// unknownField adds a field to a package rule. The text format cannot hold a
// field that the contract does not know, so the case header names it.
type unknownField struct {
	RuleID string `json:"rule_id"`
	Field  int32  `json:"field"`
}

type caseFile struct {
	Description   string         `json:"description"`
	Now           time.Time      `json:"now"`
	UnknownFields []unknownField `json:"unknown_fields"`
	Checks        []checkFile    `json:"checks"`
}

type checkFile struct {
	Name    string      `json:"name"`
	Package packageFile `json:"package"`
	Control string      `json:"control"`
	Verdict string      `json:"verdict"`
	Now     time.Time   `json:"now"`
	Want    wantFile    `json:"want"`
}

type packageFile struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

type wantFile struct {
	Effect string `json:"effect"`
	RuleID string `json:"rule_id"`
}

var controls = map[string]match.Scope{
	"malware":       match.ScopeMalware,
	"cooldown":      match.ScopeCooldown,
	"vulnerability": match.ScopeVulnerability,
}

var verdicts = map[string]match.Verdict{
	"none":             match.VerdictNone,
	"malware":          match.VerdictMalware,
	"verified-malware": match.VerdictVerifiedMalware,
}

var effects = map[string]match.Effect{
	"none":  match.EffectNone,
	"trust": match.EffectTrust,
	"block": match.EffectBlock,
}

// Cases returns the cases of the corpus, in the order of their names.
func Cases() ([]Case, error) {
	return loadCases(casesFS, casesDir)
}

// Expect returns what the loader of a tool must expect in the header of a
// policy of the runner.
func Expect(kind specv1.SubjectKind) project.Expect {
	e := project.Expect{Tenant: Tenant, SubjectKind: kind}
	switch kind {
	case specv1.SubjectKind_SUBJECT_KIND_ENDPOINT:
		e.MachineID = MachineID
	case specv1.SubjectKind_SUBJECT_KIND_PROJECT:
		e.ProjectID = ProjectID
	default:
	}
	return e
}

// Policy returns the bytes of the resolved policy of the case, on the wire,
// with a header for the subject kind.
func (c Case) Policy(kind specv1.SubjectKind) ([]byte, error) {
	if c.policy == nil {
		return nil, fmt.Errorf("%s: the case has no policy", c.File)
	}

	header := specv1.ResolvedPolicyHeader_builder{
		Tenant:   Tenant,
		Revision: 1,
		IssuedAt: timestamppb.New(issuedAt),
	}

	switch kind {
	case specv1.SubjectKind_SUBJECT_KIND_ENDPOINT:
		header.Subject = specv1.Subject_builder{Kind: kind, Id: EndpointID}.Build()
		header.MachineId = MachineID
	case specv1.SubjectKind_SUBJECT_KIND_PROJECT:
		header.Subject = specv1.Subject_builder{Kind: kind, Id: ProjectID}.Build()
	default:
		return nil, fmt.Errorf("subject kind %s has no policy header", kind)
	}

	policy, ok := proto.Clone(c.policy).(*specv1.ResolvedPolicy)
	if !ok {
		return nil, fmt.Errorf("%s: cannot copy the policy", c.File)
	}
	policy.SetHeader(header.Build())

	if err := addUnknownFields(policy, c.unknown); err != nil {
		return nil, fmt.Errorf("%s: %w", c.File, err)
	}

	return proto.Marshal(policy)
}

func addUnknownFields(policy *specv1.ResolvedPolicy, unknown []unknownField) error {
	for _, u := range unknown {
		rule := findRule(policy, u.RuleID)
		if rule == nil {
			return fmt.Errorf("unknown_fields: no package rule has the id %q", u.RuleID)
		}

		r := rule.ProtoReflect()
		b := protowire.AppendTag(nil, protowire.Number(u.Field), protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
		r.SetUnknown(append(r.GetUnknown(), b...))
	}
	return nil
}

func findRule(policy *specv1.ResolvedPolicy, ruleID string) protoreflect.ProtoMessage {
	for _, r := range policy.GetSpec().GetCore().GetPackageRules() {
		if r.GetRuleId() == ruleID {
			return r
		}
	}
	return nil
}

// loadCases reads the cases of a directory. Each case is a pair of files with
// one base name: a header in JSON and a policy in the text format.
func loadCases(fsys fs.FS, dir string) ([]Case, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the cases: %w", err)
	}

	names := map[string]bool{}
	for _, e := range entries {
		file := path.Join(dir, e.Name())
		switch {
		case e.IsDir():
			return nil, fmt.Errorf("%s: a directory is not a case", file)
		case strings.HasSuffix(e.Name(), ".json"):
			names[strings.TrimSuffix(e.Name(), ".json")] = true
		case strings.HasSuffix(e.Name(), ".textproto"):
			name := strings.TrimSuffix(e.Name(), ".textproto")
			if _, err := fs.Stat(fsys, path.Join(dir, name+".json")); err != nil {
				return nil, fmt.Errorf("%s: no case header %s.json", file, name)
			}
		default:
			return nil, fmt.Errorf("%s: a case file ends with .json or .textproto", file)
		}
	}

	var cases []Case
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !names[name] {
			continue
		}

		c, err := loadCase(fsys, dir, name)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("%s: no case", dir)
	}

	return cases, nil
}

func loadCase(fsys fs.FS, dir, name string) (Case, error) {
	c := Case{Name: name, File: path.Join(dir, name+".json")}

	raw, err := fs.ReadFile(fsys, c.File)
	if err != nil {
		return Case{}, fmt.Errorf("%s: %w", c.File, err)
	}

	// An unknown key is a typo in a case, and a typo must not skip a check.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var cf caseFile
	if err := dec.Decode(&cf); err != nil {
		return Case{}, fmt.Errorf("%s: %w", c.File, err)
	}

	policyFile := path.Join(dir, name+".textproto")
	text, err := fs.ReadFile(fsys, policyFile)
	if err != nil {
		return Case{}, fmt.Errorf("%s: no policy: %w", c.File, err)
	}

	policy := &specv1.ResolvedPolicy{}
	if err := prototext.Unmarshal(text, policy); err != nil {
		return Case{}, fmt.Errorf("%s: %w", policyFile, err)
	}

	// The runner writes the header for the subject kind of the tool. A header
	// in the file would only be overwritten.
	if policy.HasHeader() {
		return Case{}, fmt.Errorf("%s: the runner writes the header, so remove it", policyFile)
	}

	if !policy.HasSpec() {
		return Case{}, fmt.Errorf("%s: the policy has no spec", policyFile)
	}

	c.policy = policy
	c.Description = cf.Description
	c.unknown = cf.UnknownFields

	if err := c.fill(cf); err != nil {
		return Case{}, fmt.Errorf("%s: %w", c.File, err)
	}

	return c, nil
}

func (c *Case) fill(cf caseFile) error {
	if strings.TrimSpace(cf.Description) == "" {
		return errors.New("the case has no description")
	}

	if len(cf.Checks) == 0 {
		return errors.New("the case has no check")
	}

	for _, u := range cf.UnknownFields {
		num := protowire.Number(u.Field)
		if !num.IsValid() {
			return fmt.Errorf("unknown_fields: %d is not a valid field number", u.Field)
		}

		rule := findRule(c.policy, u.RuleID)
		if rule == nil {
			return fmt.Errorf("unknown_fields: no package rule has the id %q", u.RuleID)
		}

		if rule.ProtoReflect().Descriptor().Fields().ByNumber(num) != nil {
			return fmt.Errorf("unknown_fields: the contract knows field %d of a package rule", u.Field)
		}
	}

	seen := map[string]bool{}
	for _, f := range cf.Checks {
		chk, err := toCheck(f, cf.Now)
		if err != nil {
			return fmt.Errorf("check %q: %w", f.Name, err)
		}

		if seen[chk.Name] {
			return fmt.Errorf("check %q: two checks have this name", chk.Name)
		}
		seen[chk.Name] = true

		c.Checks = append(c.Checks, chk)
	}

	return nil
}

func toCheck(f checkFile, now time.Time) (Check, error) {
	if f.Name == "" {
		return Check{}, errors.New("the check has no name")
	}

	if _, err := pb.EcosystemFromName(f.Package.Ecosystem); err != nil {
		return Check{}, err
	}

	if f.Package.Name == "" {
		return Check{}, errors.New("the package has no name")
	}

	control, ok := controls[f.Control]
	if !ok {
		return Check{}, fmt.Errorf("unknown control %q", f.Control)
	}

	verdict, ok := verdicts[f.Verdict]
	if !ok {
		return Check{}, fmt.Errorf("unknown verdict %q", f.Verdict)
	}

	effect, ok := effects[f.Want.Effect]
	if !ok {
		return Check{}, fmt.Errorf("unknown effect %q", f.Want.Effect)
	}

	if (effect == match.EffectNone) != (f.Want.RuleID == "") {
		return Check{}, errors.New("a decision of a rule has a rule id, and no other decision has one")
	}

	if !f.Now.IsZero() {
		now = f.Now
	}

	if now.IsZero() {
		return Check{}, errors.New("the check has no time: set now in the case or in the check")
	}

	return Check{
		Name: f.Name,
		Query: Query{
			Package: Package{
				Ecosystem: f.Package.Ecosystem,
				Name:      f.Package.Name,
				Version:   f.Package.Version,
			},
			Control: control,
			Verdict: verdict,
			Now:     now,
		},
		Want: Decision{Effect: effect, RuleID: f.Want.RuleID},
	}, nil
}
